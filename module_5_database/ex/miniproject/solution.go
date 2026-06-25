package main

// =============================================================================
// CODE REVIEW — MISTAKES FOUND AND FIXED
// =============================================================================
//
// MISTAKE 1: Struct tags missing quotes — HIGH
// -----------------------------------------------
// You wrote:   `json:id`
// Should be:   `json:"id"`
// Without the quotes Go silently ignores the tag entirely.
// Your JSON response was returning "ID", "Title", "Author" (capitalized)
// instead of "id", "title", "author". Same problem in your `input` struct
// inside createBookHandler.
//
// MISTAKE 2: Missing defer rows.Close() — HIGH
// -----------------------------------------------
// In getALlBooksHandler you called db.Query() but never deferred rows.Close().
// Every time that handler runs, the connection is checked out from the pool
// and never returned. Under load the pool drains and your API hangs.
// Rule: always defer rows.Close() immediately after checking the error
// from db.Query().
//
// MISTAKE 3: Insert error not handled in createBookHandler — HIGH
// -----------------------------------------------
// After calling QueryRow(...).Scan(&newID) you never checked if err != nil.
// If the INSERT failed (bad data, constraint violation, DB down), your code
// ignored the error and still responded with "Successfully created with id: 0".
// Silent failures are the worst kind of bug — the client thinks it worked.
//
// MISTAKE 4: Missing return after 400 in getBookByIDHandler — HIGH
// -----------------------------------------------
// After writing the 400 response for invalid path variable, you forgot to
// return. The handler continued executing with a garbage id_int value (0),
// hit the database, and either returned the wrong book or another error.
// Every early response MUST be followed by a return.
//
// MISTAKE 5: Unused variable q in getALlBooksHandler — LOW
// -----------------------------------------------
// You declared q := "SELECT * FROM BOOKS;" but then used a different inline
// query string in db.Query(). Go will not compile with unused variables.
// Removed q entirely.
//
// =============================================================================

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type application struct {
	DB *sql.DB
}

type envelope map[string]any

// MISTAKE 1 FIXED: added quotes around all json tag values
type Book struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	Year    int    `json:"year"`
	Created string `json:"created"`
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("pgx", path)
	if err != nil {
		return nil, fmt.Errorf("error with connection string: %w", err)
	}

	db.SetMaxIdleConns(25)
	db.SetMaxOpenConns(25)
	db.SetConnMaxIdleTime(1 * time.Minute)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to db: %w", err)
	}

	return db, nil
}

func (app *application) getALlBooksHandler(w http.ResponseWriter, r *http.Request) {
	// MISTAKE 5 FIXED: removed unused variable q

	rows, err := app.DB.Query(`SELECT * FROM books`)
	if err != nil {
		app.writeJSON(w, 500, envelope{"error": err.Error()})
		return
	}
	// MISTAKE 2 FIXED: added defer rows.Close() immediately after error check
	// This returns the connection to the pool when the handler exits,
	// no matter how many return statements are below.
	defer rows.Close()

	var books []Book

	for rows.Next() {
		var thisBook Book

		if err := rows.Scan(&thisBook.ID, &thisBook.Title, &thisBook.Author, &thisBook.Year, &thisBook.Created); err != nil {
			app.writeJSON(w, 500, envelope{"error": err.Error()})
			return
		}

		books = append(books, thisBook)
	}

	if err := rows.Err(); err != nil {
		app.writeJSON(w, 500, envelope{"error": err.Error()})
		return
	}

	app.writeJSON(w, 200, envelope{"books": books})
}

func (app *application) getBookByIDHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	id_int, err := strconv.Atoi(id)

	if err != nil {
		app.writeJSON(w, 400, envelope{"error": fmt.Errorf("invalid path variable: %w", err).Error()})
		// MISTAKE 4 FIXED: added return here
		// Without this, the handler continued executing with id_int = 0
		// and hit the database with a garbage value.
		return
	}

	row := app.DB.QueryRow(`SELECT * FROM books WHERE id = $1`, id_int)

	var book Book

	err = row.Scan(
		&book.ID,
		&book.Title,
		&book.Author,
		&book.Year,
		&book.Created,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.writeJSON(w, http.StatusNotFound, envelope{
				"error": "book not found",
			})
			return
		}

		app.writeJSON(w, http.StatusInternalServerError, envelope{
			"error": err.Error(),
		})
		return
	}

	app.writeJSON(w, http.StatusOK, envelope{
		"book": book,
	})
}

func (app *application) createBookHandler(w http.ResponseWriter, r *http.Request) {
	// MISTAKE 1 FIXED: added quotes around json tag values
	var input struct {
		Title  string `json:"title"`
		Author string `json:"author"`
		Year   int    `json:"year"`
	}

	if err := app.decodeJSON(w, r, &input); err != nil {
		app.writeJSON(w, 400, envelope{"error": err.Error()})
		return
	}

	var newID int
	err := app.DB.QueryRow(
		"INSERT INTO books (title, author, year) VALUES ($1, $2, $3) RETURNING id",
		input.Title,
		input.Author,
		input.Year,
	).Scan(&newID)

	// MISTAKE 3 FIXED: added error check after Scan
	// Before this fix, if the INSERT failed your code responded with
	// "Successfully created with id: 0" — the client thought it worked.
	if err != nil {
		app.writeJSON(w, 500, envelope{"error": err.Error()})
		return
	}

	msg := fmt.Sprintf("successfully created with id: %d", newID)
	app.writeJSON(w, http.StatusCreated, envelope{"msg": msg})
}

func (app *application) deleteBookByIDHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	id_int, err := strconv.Atoi(id)

	if err != nil {
		app.writeJSON(w, 400, envelope{"error": err.Error()})
		return
	}

	res, err := app.DB.Exec(
		"DELETE FROM books WHERE id = $1", id_int,
	)
	if err != nil {
		app.writeJSON(w, 500, envelope{"error": err.Error()})
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		app.writeJSON(w, 500, envelope{"error": err.Error()})
		return
	}
	if rowsAffected != 1 {
		app.writeJSON(w, 404, envelope{"error": "book not found"})
		return
	}

	msg := fmt.Sprintf("successfully deleted id: %d", id_int)
	app.writeJSON(w, http.StatusOK, envelope{"msg": msg})
}

func main() {
	dbPath := "postgres://myuser:mypass@localhost:5432/mydatabase?sslmode=disable"

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := &application{
		DB: db,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /books", app.getALlBooksHandler)
	mux.HandleFunc("GET /books/{id}", app.getBookByIDHandler)
	mux.HandleFunc("POST /books", app.createBookHandler)
	mux.HandleFunc("DELETE /books/{id}", app.deleteBookByIDHandler)

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("listening on :8080")
	log.Fatal(server.ListenAndServe())
}

func (app *application) writeJSON(w http.ResponseWriter, status int, body envelope) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(body)
}

func (app *application) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	err := decoder.Decode(dst)
	if err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxErr):
			return fmt.Errorf("malformed JSON at position %d", syntaxErr.Offset)
		case errors.As(err, &unmarshalErr):
			return fmt.Errorf("wrong type for field %q", unmarshalErr.Field)
		case errors.Is(err, io.EOF):
			return fmt.Errorf("body must not be empty")
		default:
			return err
		}
	}
	return nil
}
