package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

type BookRepository interface {
}

type LoanRepository interface {
}

type MemberRepository interface {
}

type app struct {
	infoLog  *log.Logger
	errorLog *log.Logger
	books    BookRepository
	loans    LoanRepository
	members  MemberRepository
}

func main() {
	// router
	serveMux := http.NewServeMux()

	thisApp := &app{
		infoLog:  log.New(os.Stdout, "INFO\t", log.Ltime|log.Ldate),
		errorLog: log.New(os.Stderr, "ERROR\t", log.Ltime|log.Ldate),
	}

	serveMux.HandleFunc("/hello", thisApp.hello)

	// server

	server := &http.Server{
		Addr:           ":8080",
		Handler:        serveMux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	log.Fatal(server.ListenAndServe())
}
