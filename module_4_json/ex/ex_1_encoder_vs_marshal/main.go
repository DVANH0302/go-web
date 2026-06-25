package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

type envelope map[string]any

type Book struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type User struct {
	ID       int    `json: "id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"-"`
}

type application struct {
	logger *log.Logger
	books  []Book
	nextID int
	mu     *sync.RWMutex
}

func (a *application) getBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	a.logger.Printf("Received request with book id: %v\n", id)
}

func (a *application) createBook(w http.ResponseWriter, r *http.Request) {
	var book Book
	err := json.NewDecoder(r.Body).Decode(&book)
	if err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	a.mu.RLock()
	for _, b := range a.books {
		if b.ID == book.ID {
			http.Error(w, "invalid ID", http.StatusBadRequest)
			return
		}
	}
	a.mu.RUnlock()

	a.mu.Lock()
	a.books = append(a.books, book)
	a.mu.Unlock()
}

func (a *application) deleteBook(w http.ResponseWriter, r *http.Request) {

}

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /books/{id}", app.getBook)

	mux.HandleFunc("POST /books", app.createBook)
	mux.HandleFunc("DELETE /books/{id}", app.deleteBook)

	return mux
}

func main() {

	user := User{
		ID:    1,
		Name:  "Andy",
		Email: "abc@gmail.com",
	}

	json.NewEncoder(os.Stdout).Encode(user)

	data, err := json.Marshal(user)

	if err != nil {
		log.Fatal(err)
		return
	}

	os.Stdout.Write(data)

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

// helper

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
