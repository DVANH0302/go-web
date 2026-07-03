package main

import "net/http"

func (app *application) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /books", app.createBook)
	mux.HandleFunc("GET /books", app.getAllBook)
	mux.HandleFunc("GET /books/{id}", app.getBookByID)
	mux.HandleFunc("DELETE /books/{id}", app.deleteBookByID)

	return mux
}
