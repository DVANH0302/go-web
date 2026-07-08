# Module 8 — Code (worked example)

A minimal book API in the proper layout. Read top-to-bottom: config → repository → service → handler → main.

## `internal/config/config.go`

```go
package config

import "os"

type Config struct {
    Port string
    DSN  string
}

// getenv with a fallback — os.Getenv returns "" when unset.
func getenv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func Load() Config {
    return Config{
        Port: getenv("PORT", "4000"),
        DSN:  getenv("DB_DSN", "postgres://user:pass@localhost/bookapi"),
    }
}
```

## `internal/books/repository.go` — SQL only

```go
package books

import "database/sql"

type Book struct {
    ID     int    `json:"id"`
    Title  string `json:"title"`
    Author string `json:"author"`
}

type Repository struct {
    DB *sql.DB
}

func (r *Repository) GetAll() ([]Book, error) {
    rows, err := r.DB.Query(`SELECT id, title, author FROM books`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var books []Book
    for rows.Next() {
        var b Book
        if err := rows.Scan(&b.ID, &b.Title, &b.Author); err != nil {
            return nil, err
        }
        books = append(books, b)
    }
    return books, rows.Err()
}

func (r *Repository) Create(b Book) (int, error) {
    var id int
    err := r.DB.QueryRow(
        `INSERT INTO books (title, author) VALUES ($1, $2) RETURNING id`,
        b.Title, b.Author,
    ).Scan(&id)
    return id, err
}
```

## `internal/books/service.go` — business rules only

```go
package books

import "errors"

var ErrInvalidBook = errors.New("title and author are required")

type Service struct {
    Repo *Repository
}

func (s *Service) List() ([]Book, error) {
    return s.Repo.GetAll()
}

func (s *Service) Add(b Book) (int, error) {
    if b.Title == "" || b.Author == "" { // validation lives HERE, not in the handler
        return 0, ErrInvalidBook
    }
    return s.Repo.Create(b)
}
```

## `internal/books/handler.go` — HTTP only

```go
package books

import (
    "encoding/json"
    "errors"
    "net/http"
)

type Handler struct {
    Service *Service
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
    books, err := h.Service.List()
    if err != nil {
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(books)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    var b Book
    if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
        http.Error(w, "malformed JSON", http.StatusBadRequest)
        return
    }
    id, err := h.Service.Add(b)
    if errors.Is(err, ErrInvalidBook) {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    if err != nil {
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]int{"id": id})
}
```

## `cmd/api/main.go` — wiring only

```go
package main

import (
    "database/sql"
    "log"
    "net/http"

    "github.com/joho/godotenv"
    _ "github.com/lib/pq"

    "bookapi/internal/books"
    "bookapi/internal/config"
)

func main() {
    _ = godotenv.Load() // loads .env if present; fine if it's missing (prod)

    cfg := config.Load()

    db, err := sql.Open("postgres", cfg.DSN)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    if err := db.Ping(); err != nil {
        log.Fatal(err)
    }

    // Wire the layers: repo → service → handler
    repo := &books.Repository{DB: db}
    svc := &books.Service{Repo: repo}
    h := &books.Handler{Service: svc}

    mux := http.NewServeMux()
    mux.HandleFunc("GET /books", h.List)
    mux.HandleFunc("POST /books", h.Create)

    log.Printf("starting server on :%s", cfg.Port)
    log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
}
```

## `.env` (gitignored!)

```
PORT=4000
DB_DSN=postgres://user:pass@localhost/bookapi?sslmode=disable
```

Setup commands:

```bash
go get github.com/joho/godotenv
echo ".env" >> .gitignore
go run ./cmd/api          # note: ./cmd/api, not main.go
```

**Trace one request in your head:** `POST /books` → mux → `Handler.Create` (decodes JSON) → `Service.Add` (validates) → `Repository.Create` (SQL) → back up the chain. Each layer only knows its neighbour.
