package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type Book struct {
	ID    int
	Title string
}

type application struct {
	mu_books sync.RWMutex
	logger   *log.Logger
	books    []Book
	nextID   int
}

func (a *application) home_handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)

	w.Write([]byte(`{"status": "ok"}`))
}

func (a *application) books_handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)

	var sb strings.Builder
	a.mu_books.RLock()
	for i, b := range a.books {

		str_book := fmt.Sprintf(`{"ID": %d, "Title": "%s"}`, b.ID, b.Title)

		if i != len(a.books)-1 {
			str_book += ", "
		}

		sb.WriteString(str_book)
	}
	a.mu_books.Unlock()

	msg := fmt.Sprintf(`{"books": [%s]}`, sb.String())
	w.Write([]byte(msg))
}

func (a *application) book_handler(w http.ResponseWriter, r *http.Request) {

	allowedMethods := []string{http.MethodGet, http.MethodPost, http.MethodDelete}

	if !slices.Contains(allowedMethods, r.Method) {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")

		id_param := r.URL.Query().Get("id")
		book_index, err := strconv.Atoi(id_param)

		if err != nil {
			http.Error(w, "Error reading book index", http.StatusBadRequest)
			return
		}
		w.WriteHeader(200)

		var found *Book
		for i := range a.books {
			if a.books[i].ID == book_index {
				found = &a.books[i]
				break
			}
		}
		if found == nil {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}

		a.mu_books.RLock()
		msg := fmt.Sprintf(`{"ID": %d, "Title": %s}`, found.ID, found.Title)
		a.mu_books.RUnlock()

		w.Write([]byte(msg))

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading body", http.StatusBadRequest)
			return
		}

		var newBook Book
		err = json.Unmarshal(body, &newBook)
		if err != nil {
			http.Error(w, "Error parsing body in json format", http.StatusBadRequest)
			return
		}

		a.mu_books.Lock()

		newBook.ID = a.nextID
		a.books = append(a.books, newBook)
		a.nextID++

		a.mu_books.Unlock()

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created succesfully\n"))

	case http.MethodDelete:
		id_param := r.URL.Query().Get("id")
		id, err := strconv.Atoi(id_param)
		if err != nil {
			http.Error(w, "Error reading id", http.StatusBadRequest)
			return
		}

		a.mu_books.Lock()
		found := false
		for i := range a.books {
			if a.books[i].ID == id {
				a.books = append(a.books[:i], a.books[i+1:]...)
				found = true
				break
			}
		}
		a.mu_books.Unlock()

		if !found {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	}

}

func main() {

	mux := http.NewServeMux()
	app := &application{
		// mu
		logger: log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime),
		books:  make([]Book, 0),
		nextID: 0,
	}

	for range 5 {
		app.books = append(app.books, Book{ID: app.nextID, Title: "Book " + strconv.Itoa(app.nextID)})
		app.nextID++
	}

	mux.HandleFunc("/", app.home_handler)
	mux.HandleFunc("/books", app.books_handler)
	mux.HandleFunc("/book", app.book_handler)

	http.ListenAndServe(":8080", mux)

}
