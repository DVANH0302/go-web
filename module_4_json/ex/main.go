package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
)

type envelope map[string]any

type Book struct {
	ID    int
	Title string
}

type User struct {
	ID    int
	Name  string
	Email string
}

type application struct {
	logger *log.Logger
	books  []Book
	nextID int
	mutex  sync.RWMutex
}

func (a *application) getBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	id_int, err := strconv.Atoi(id)

	if err != nil {
		http.Error(w, "invalid ID", http.StatusBadRequest)
		return
	}

	a.mutex.RLock()
	for _, b := range a.books {
		if b.ID == id_int {
			a.writeJson(w, http.StatusAccepted, envelope{"book": b})
			a.mutex.RUnlock()
			return
		}
	}
	a.mutex.RUnlock()

	a.writeJson(w, http.StatusNotFound, envelope{"error": "book not found"})
}

func (a *application) createBook(w http.ResponseWriter, r *http.Request) {

	var input struct {
		Title string `json:"title"`
	}

	err := a.decodeJson(w, r, &input)

	if err != nil {
		err = a.writeJson(w, http.StatusBadRequest, envelope{"error": err.Error()})
	}

	a.mutex.Lock()
	var thisBook Book = Book{
		ID:    a.nextID,
		Title: input.Title,
	}
	a.nextID++
	a.books = append(a.books, thisBook)
	a.mutex.Unlock()
}

func (a *application) getAllBooks(w http.ResponseWriter, r *http.Request) {
	a.mutex.RLock()
	a.writeJson(w, http.StatusAccepted, envelope{"books": a.books})
	a.mutex.RUnlock()
}

func (a *application) deleteBook(w http.ResponseWriter, r *http.Request) {

}

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /books/{id}", app.getBook)
	mux.HandleFunc("GET /books", app.getAllBooks)
	mux.HandleFunc("POST /books", app.createBook)
	mux.HandleFunc("DELETE /books/{id}", app.deleteBook)

	return mux
}

func main() {

	app := &application{
		logger: log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime),
		books:  make([]Book, 0),
		nextID: 0,
	}

	server := &http.Server{
		Addr:    ":8080",
		Handler: app.routes(),
	}

	app.logger.Println("Server listening on :8080")
	log.Fatal(server.ListenAndServe())
}

func (a *application) decodeJson(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)

	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("malformed JSON at position %d", syntaxError.Offset)
		case errors.As(err, &unmarshalError):
			return fmt.Errorf("wrong type for field %q", unmarshalError.Field)
		case errors.Is(err, io.EOF):
			return fmt.Errorf("body must not be empty")
		default:
			return err
		}
	}
	return nil

}

func (a *application) writeJson(w http.ResponseWriter, status int, data envelope) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}
