package main

import (
	"log"
	"net/http"
	"note/internal/notes"
)

func main() {

	mux := http.NewServeMux()

	noteStore := notes.NewMemoryStores()
	noteService := notes.NewService(noteStore)
	noteHandler := notes.NewHandler(noteService)

	mux.HandleFunc("GET /notes/{id}", noteHandler.GetNote)
	mux.HandleFunc("POST /notes", noteHandler.CreateNote)
	mux.HandleFunc("DELETE /notes/{id}", noteHandler.DeleteNote)
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}
