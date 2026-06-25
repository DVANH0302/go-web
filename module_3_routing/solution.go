package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
)

type Book struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type application struct {
	logger *log.Logger
	mu     sync.RWMutex
	books  []Book
	nextID int
}

func (a *application) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", a.home)
	mux.HandleFunc("GET /books", a.listBooks)
	mux.HandleFunc("POST /books", a.createBook)
	mux.HandleFunc("GET /books/{id}", a.getBook)
	mux.HandleFunc("DELETE /books/{id}", a.deleteBook)

	return mux
}

func (a *application) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (a *application) listBooks(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a.books)
}

func (a *application) getBook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	a.mu.RLock()
	var found *Book
	for i := range a.books {
		if a.books[i].ID == id {
			found = &a.books[i]
			break
		}
	}
	a.mu.RUnlock()

	if found == nil {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(found)
}

func (a *application) createBook(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title string `json:"title"`
	}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil || input.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	book := Book{ID: a.nextID, Title: input.Title}
	a.books = append(a.books, book)
	a.nextID++
	a.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(book)
}

func (a *application) deleteBook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	found := false
	for i := range a.books {
		if a.books[i].ID == id {
			a.books = append(a.books[:i], a.books[i+1:]...)
			found = true
			break
		}
	}
	a.mu.Unlock()

	if !found {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func main() {
	app := &application{
		logger: log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime),
		books:  []Book{},
		nextID: 1,
	}

	log.Fatal(http.ListenAndServe(":8080", app.routes()))
}
