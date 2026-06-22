package main

import (
	"fmt"
	"net/http"
)

func home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Write([]byte("This is home page\n"))
}

func api(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("This is api\n"))
}

func pong(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong\n"))
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", home)
	mux.HandleFunc("/api/", api)
	mux.HandleFunc("/api/ping", pong)

	fmt.Println("Server listening on port 8080")
	http.ListenAndServe(":8080", mux)
}
