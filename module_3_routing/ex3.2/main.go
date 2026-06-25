package main

import (
	"log"
	"net/http"
	"os"
)

type Book struct {
	ID    int
	Title string
}

type application struct {
	logger *log.Logger
	books  []Book
	nextID int
}

func (a *application) getBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	a.logger.Printf("Received request with book id: %v\n", id)
}

func (a *application) createBook(w http.ResponseWriter, r *http.Request) {

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
