# Module 05 — Database (PostgreSQL): Code

Every example builds on the previous one. By the end you have a full working database layer for the book API.

---

## Setup — Install Driver & Create Database

```bash
# Install the pgx driver (stdlib wrapper gives us database/sql compatibility)
go get github.com/jackc/pgx/v5/stdlib

# Create the database in psql
psql -U postgres
CREATE DATABASE booksdb;
\c booksdb
```

---

## Step 1 — Connect and Ping

```go
package main

import (
    "database/sql"
    "fmt"
    "log"
    "time"

    _ "github.com/jackc/pgx/v5/stdlib" // blank import: registers the "pgx" driver
)

func openDB(dsn string) (*sql.DB, error) {
    // sql.Open does NOT open a connection.
    // It validates the dsn format and creates the pool manager.
    // If dsn is wrong, you won't know until Ping().
    db, err := sql.Open("pgx", dsn)
    if err != nil {
        return nil, err
    }

    // Pool tuning — do this before first use.
    db.SetMaxOpenConns(25)              // max connections total (idle + in-use)
    db.SetMaxIdleConns(25)              // max connections sitting warm in pool
    db.SetConnMaxLifetime(5 * time.Minute) // retire connections after 5m (handles PG restarts)
    db.SetConnMaxIdleTime(1 * time.Minute) // retire idle connections after 1m

    // Ping actually opens one connection and authenticates.
    // This is where a wrong password / host / database name errors out.
    if err := db.Ping(); err != nil {
        return nil, err
    }

    return db, nil
}

func main() {
    dsn := "postgres://postgres:password@localhost:5432/booksdb?sslmode=disable"

    db, err := openDB(dsn)
    if err != nil {
        log.Fatal(err) // log.Fatal prints the error then calls os.Exit(1)
    }
    defer db.Close() // return pool resources on shutdown

    fmt.Println("connected successfully")

    // db.Stats() shows you the pool state — useful for debugging
    stats := db.Stats()
    fmt.Printf("open connections: %d\n", stats.OpenConnections)
}
```

**What to observe:**
- Change the password to something wrong → `Ping()` fails, not `Open()`
- `db.Stats().OpenConnections` is 1 after Ping, shows pool is real

---

## Step 2 — Migration File

Create this file manually. Run it with `psql` before any Go code touches the table.

```sql
-- migrations/000001_create_books.up.sql

CREATE TABLE IF NOT EXISTS books (
    id      SERIAL PRIMARY KEY,          -- SERIAL = auto-incrementing integer
    title   TEXT    NOT NULL,
    author  TEXT    NOT NULL,
    year    INT     NOT NULL,
    created TIMESTAMPTZ DEFAULT NOW()    -- TIMESTAMPTZ = timestamp with timezone
);

-- Seed some data so SELECT queries have something to return
INSERT INTO books (title, author, year) VALUES
    ('The Go Programming Language', 'Donovan & Kernighan', 2015),
    ('Clean Code', 'Robert Martin', 2008),
    ('Designing Data-Intensive Applications', 'Martin Kleppmann', 2017);
```

```bash
# Run it:
psql -U postgres -d booksdb -f migrations/000001_create_books.up.sql
```

---

## Step 3 — The Book Struct

```go
// internal/books/model.go
package books

import "time"

// Book maps exactly to the books table columns.
// Field names don't have to match column names — Scan maps positionally,
// not by name. But matching them makes the code easier to reason about.
type Book struct {
    ID      int
    Title   string
    Author  string
    Year    int
    Created time.Time
}
```

---

## Step 4 — `GetAll` with `db.Query` and `rows.Scan`

```go
// internal/books/repository.go
package books

import (
    "database/sql"
    "fmt"
)

type BookRepository struct {
    DB *sql.DB // exported field so main.go can inject the pool
}

// GetAll returns every book in the table.
func (r *BookRepository) GetAll() ([]Book, error) {
    // $1, $2 are PostgreSQL placeholders. No user input here, but
    // we still use the query method — keeps the pattern consistent.
    rows, err := r.DB.Query(`
        SELECT id, title, author, year, created
        FROM books
        ORDER BY created DESC
    `)
    if err != nil {
        // Error here = the query never started.
        // Could be: no connection, syntax error, table doesn't exist.
        return nil, fmt.Errorf("GetAll query: %w", err)
    }
    defer rows.Close() // CRITICAL: returns connection to pool even if we return early

    var books []Book

    // rows.Next() advances the cursor one row.
    // Returns false when exhausted OR on error (check rows.Err() after).
    for rows.Next() {
        var b Book
        // Scan reads the current row into the variables.
        // Order MUST match the SELECT column order: id, title, author, year, created
        err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.Year, &b.Created)
        if err != nil {
            return nil, fmt.Errorf("GetAll scan: %w", err)
        }
        books = append(books, b)
    }

    // rows.Err() captures errors that happened during iteration.
    // rows.Next() swallows them — you MUST check here.
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("GetAll rows: %w", err)
    }

    return books, nil
}
```

---

## Step 5 — `GetByID` with `QueryRow` and `ErrNoRows`

```go
// GetByID fetches one book by primary key.
// Returns sql.ErrNoRows if the id doesn't exist — caller decides what that means.
func (r *BookRepository) GetByID(id int) (*Book, error) {
    var b Book

    // QueryRow returns *sql.Row immediately — no error yet.
    // The query runs lazily when you call .Scan().
    err := r.DB.QueryRow(`
        SELECT id, title, author, year, created
        FROM books
        WHERE id = $1
    `, id).Scan(&b.ID, &b.Title, &b.Author, &b.Year, &b.Created)
    // ↑ $1 is replaced by the value of `id` parameter
    // PostgreSQL receives them as separate items — injection is impossible

    if err != nil {
        // Don't wrap ErrNoRows — callers check for it with errors.Is()
        // and wrapping would require errors.Is to unwrap it.
        // (We'll revisit this in Module 9 — Error Handling)
        return nil, err
    }

    return &b, nil
}
```

**Caller side — how to handle ErrNoRows:**

```go
book, err := repo.GetByID(42)
if errors.Is(err, sql.ErrNoRows) {
    http.NotFound(w, r) // 404 — normal outcome, not a crash
    return
}
if err != nil {
    log.Println(err)
    http.Error(w, "Internal Server Error", 500) // real database problem
    return
}
// use book here
```

---

## Step 6 — `Create` with `RETURNING`

```go
// Create inserts a new book and returns its generated ID.
// We use QueryRow + RETURNING because PostgreSQL doesn't support LastInsertId().
func (r *BookRepository) Create(title, author string, year int) (int, error) {
    var newID int

    err := r.DB.QueryRow(`
        INSERT INTO books (title, author, year)
        VALUES ($1, $2, $3)
        RETURNING id
    `, title, author, year).Scan(&newID)
    // RETURNING id makes PostgreSQL treat the INSERT like a SELECT —
    // it returns one row with the newly generated id column value.

    if err != nil {
        return 0, fmt.Errorf("Create: %w", err)
    }

    return newID, nil
}
```

---

## Step 7 — `Delete` with `RowsAffected`

```go
// Delete removes a book by ID.
// Returns sql.ErrNoRows if no row was deleted (id didn't exist).
func (r *BookRepository) Delete(id int) error {
    result, err := r.DB.Exec(`
        DELETE FROM books WHERE id = $1
    `, id)
    if err != nil {
        return fmt.Errorf("Delete exec: %w", err)
    }

    // RowsAffected tells us how many rows the DELETE actually removed.
    // If 0, the id didn't exist.
    n, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("Delete rows affected: %w", err)
    }
    if n == 0 {
        return sql.ErrNoRows // reuse this sentinel to signal "not found"
    }

    return nil
}
```

---

## Step 8 — Transactions

```go
// Transfer moves a book from one category to another atomically.
// Both updates must succeed or neither does.
func (r *BookRepository) Transfer(bookID, fromCategoryID, toCategoryID int) error {
    // db.Begin() checks out a connection and starts a transaction on it.
    tx, err := r.DB.Begin()
    if err != nil {
        return fmt.Errorf("Transfer begin: %w", err)
    }

    // defer Rollback is your safety net.
    // If anything below returns early (error path), Rollback undoes all work.
    // If Commit was called successfully, Rollback is a no-op — safe to call twice.
    defer tx.Rollback()

    // Step 1 — remove book from the source category
    _, err = tx.Exec(`
        DELETE FROM book_categories
        WHERE book_id = $1 AND category_id = $2
    `, bookID, fromCategoryID)
    if err != nil {
        // Rollback fires via defer — step 1 is undone
        return fmt.Errorf("Transfer remove: %w", err)
    }

    // Step 2 — add book to the destination category
    _, err = tx.Exec(`
        INSERT INTO book_categories (book_id, category_id)
        VALUES ($1, $2)
    `, bookID, toCategoryID)
    if err != nil {
        // Rollback fires via defer — BOTH step 1 AND step 2 are undone
        return fmt.Errorf("Transfer add: %w", err)
    }

    // Commit makes both changes permanent atomically.
    // If Commit fails (rare — server crash, network), defer Rollback cleans up.
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("Transfer commit: %w", err)
    }

    return nil // defer Rollback fires but is a no-op after Commit
}
```

---

## Step 9 — Wiring It All Together in `main.go`

```go
// main.go
package main

import (
    "database/sql"
    "encoding/json"
    "errors"
    "log"
    "net/http"
    "strconv"
    "time"

    "yourmodule/internal/books"
    _ "github.com/jackc/pgx/v5/stdlib"
)

type application struct {
    books *books.BookRepository
}

func openDB(dsn string) (*sql.DB, error) {
    db, err := sql.Open("pgx", dsn)
    if err != nil {
        return nil, err
    }
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(25)
    db.SetConnMaxLifetime(5 * time.Minute)
    if err := db.Ping(); err != nil {
        return nil, err
    }
    return db, nil
}

func main() {
    dsn := "postgres://postgres:password@localhost:5432/booksdb?sslmode=disable"

    db, err := openDB(dsn)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    app := &application{
        books: &books.BookRepository{DB: db},
    }

    mux := http.NewServeMux()
    mux.HandleFunc("GET /books", app.listBooks)
    mux.HandleFunc("GET /books/{id}", app.getBook)
    mux.HandleFunc("POST /books", app.createBook)
    mux.HandleFunc("DELETE /books/{id}", app.deleteBook)

    log.Println("listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux))
}

// Handler: GET /books
func (app *application) listBooks(w http.ResponseWriter, r *http.Request) {
    bookList, err := app.books.GetAll()
    if err != nil {
        log.Println(err)
        http.Error(w, "Internal Server Error", 500)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(bookList)
}

// Handler: GET /books/{id}
func (app *application) getBook(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(r.PathValue("id"))
    if err != nil {
        http.Error(w, "invalid id", 400)
        return
    }

    book, err := app.books.GetByID(id)
    if errors.Is(err, sql.ErrNoRows) {
        http.NotFound(w, r)
        return
    }
    if err != nil {
        log.Println(err)
        http.Error(w, "Internal Server Error", 500)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(book)
}

// Handler: POST /books
func (app *application) createBook(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Title  string `json:"title"`
        Author string `json:"author"`
        Year   int    `json:"year"`
    }

    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        http.Error(w, "bad request", 400)
        return
    }

    id, err := app.books.Create(input.Title, input.Author, input.Year)
    if err != nil {
        log.Println(err)
        http.Error(w, "Internal Server Error", 500)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(201)
    json.NewEncoder(w).Encode(map[string]int{"id": id})
}

// Handler: DELETE /books/{id}
func (app *application) deleteBook(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(r.PathValue("id"))
    if err != nil {
        http.Error(w, "invalid id", 400)
        return
    }

    err = app.books.Delete(id)
    if errors.Is(err, sql.ErrNoRows) {
        http.NotFound(w, r)
        return
    }
    if err != nil {
        log.Println(err)
        http.Error(w, "Internal Server Error", 500)
        return
    }

    w.WriteHeader(204) // 204 No Content — success, nothing to return
}
```

---

## Testing It With curl

```bash
# List all books
curl -s http://localhost:8080/books | jq

# Get one book
curl -s http://localhost:8080/books/1 | jq

# Get a book that doesn't exist
curl -v http://localhost:8080/books/999

# Create a book
curl -s -X POST http://localhost:8080/books \
  -H "Content-Type: application/json" \
  -d '{"title":"Pragmatic Programmer","author":"Hunt & Thomas","year":1999}' | jq

# Delete a book
curl -v -X DELETE http://localhost:8080/books/1

# Try to delete it again — should 404
curl -v -X DELETE http://localhost:8080/books/1
```

---

## SQL Injection Demo

Run this in psql to see what a vulnerable query produces:

```sql
-- Simulating: SELECT * FROM books WHERE title = '' OR '1'='1'
SELECT * FROM books WHERE title = '' OR '1'='1';
-- Returns ALL rows — attacker can read your entire database

-- Simulating: '; DROP TABLE books; --
-- The Go driver with $1 placeholders prevents this entirely.
-- PostgreSQL never sees it as SQL — just a literal string value.
```

The driver sends these as two separate packets:
1. The query template: `SELECT * FROM books WHERE title = $1`
2. The parameter value: `' OR '1'='1'`

PostgreSQL processes them separately. The parameter is **always data, never SQL**.
