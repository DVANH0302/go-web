package main

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

type Book struct {
	ID      int    `json:id`
	Title   string `json:title`
	Author  string `json:author`
	Year    int    `json:year`
	Created string `json:created`
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("pgx", path)
	if err != nil {
		return nil, fmt.Errorf("Error with connection string: %v\n", err)
	}

	db.SetMaxIdleConns(25)
	db.SetMaxOpenConns(25)
	db.SetConnMaxIdleTime(1 * time.Minute)
	db.SetConnMaxLifetime(5 * time.Minute)

	err = db.Ping()

	if err != nil {
		return nil, fmt.Errorf("Error connecting db: %v\n", err)
	}

	return db, nil
}

func (app *application) getALlBooksHandler(w http.ResponseWriter, r *http.Request) {

	q := "SELECT * FROM BOOKS;"

	rows, err := app.DB.Query(
		`SELECT * FROM books;`,
	)

	if err != nil {
		app.writeJSON(w, 500,
			envelope{"Error with query": err.Error(), "Query:": q})
		return
	}
	var books []Book

	for rows.Next() {
		var thisBook = Book{}

		if err := rows.Scan(&thisBook.ID, &thisBook.Title, &thisBook.Author, &thisBook.Year, &thisBook.Created); err != nil {
			app.writeJSON(w, 500, envelope{"Error": err})
			return
		}

		books = append(books, thisBook)
	}

	if err := rows.Err(); err != nil {
		app.writeJSON(w, 500, envelope{"Error": err})
		return
	}

	fmt.Printf("books: %v\n", books)

	app.writeJSON(w, 200, envelope{"Books": books})
}

func (app *application) getBookByIDHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	id_int, err := strconv.Atoi(id)

	if err != nil {
		app.writeJSON(w, 400, envelope{"Error": fmt.Errorf("Invalid path variable %v", err).Error()})
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
	var input struct {
		Title  string `json:title`
		Author string `json:author`
		Year   int    `json:year`
	}

	err := app.decodeJSON(w, r, &input)
	if err != nil {
		app.writeJSON(w, 400, envelope{"error": err.Error()})
		return
	}

	var newID int
	err = app.DB.QueryRow(
		"INSERT INTO books (title, author, year) VALUES ($1, $2, $3) RETURNING id",
		input.Title,
		input.Author,
		input.Year,
	).Scan(&newID)

	msg := fmt.Sprintf("Successfully created with id: %d", newID)
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
		"DELETE FROM books where id = $1", id_int,
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
		app.writeJSON(w, 404, envelope{"error": "books not found"})

		return
	}

	msg := fmt.Sprintf("Successfully deleted id: %d", id_int)
	app.writeJSON(w, http.StatusOK, envelope{"msg": msg})
}

func main() {

	dbPath := "postgres://myuser:mypass@localhost:5432/mydatabase?sslmode=disable"

	db, err := openDB(dbPath)

	if err != nil {
		log.Fatal(err)
		return
	}

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

	log.Fatal(server.ListenAndServe())

}

func (app *application) writeJSON(w http.ResponseWriter, status int, body envelope) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(body)
}

func (app *application) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit

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
