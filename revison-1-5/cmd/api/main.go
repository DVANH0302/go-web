package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"
)

type application struct {
	infoLog  *log.Logger
	errorLog *log.Logger
	books    BookRepository
	loans    LoanRepository
	members  MemberRepository
}

type bookRepositoryImpl struct {
	db *sql.DB
}

func main() {
	// router

	app := &application{
		infoLog:  log.New(os.Stdout, "INFO\t", log.Ltime|log.Ldate),
		errorLog: log.New(os.Stderr, "ERROR\t", log.Ltime|log.Ldate),
		books:    bookRepositoryImpl{db: DB},
	}

	mux := app.routes()

	// server

	server := &http.Server{
		Addr:           ":8080",
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	log.Fatal(server.ListenAndServe())
}
