package main

import "net/http"

func (app *application) createBook(w http.ResponseWriter, r *http.Request) {

}

func (app *application) getAllBook(w http.ResponseWriter, r *http.Request) {
	books, err := app.books.GetAll()

	if err != nil {
		app.writeJson(w, http.StatusInternalServerError, envelope{"Error": "Internal Server Error"})
	}

	app.writeJson(w, 200, envelope{"books": books})
}

func (app *application) getBookByID(w http.ResponseWriter, r *http.Request) {

}

func (app *application) deleteBookByID(w http.ResponseWriter, r *http.Request) {

}
