package main

import (
	"fmt"
	"net/http"
)

type InfoHandler struct{}

// ServeHTTP implements http.Handler
func (i InfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Method: %s \n Path: %s", r.Method, r.URL.Path)
}

var _ http.Handler = InfoHandler{}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

func main() {
	mux := http.NewServeMux()

	mux.Handle("/info", InfoHandler{})
	mux.Handle("/", http.HandlerFunc(healthCheck))

	fmt.Println("Server listening on port 8080")
	http.ListenAndServe(":8080", mux)
}
