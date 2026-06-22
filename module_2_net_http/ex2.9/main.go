package main

import (
	"fmt"
	"io"
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

func echo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)

	if err != nil {
		http.Error(w, "Error reading the body", http.StatusInternalServerError)
		return
	}

	header := r.Header.Get("X-Request-ID")
	upper := r.URL.Query().Get("upper")

	message := fmt.Sprintf("body: %s \n header: %s \n upper: %s \n ", body, header, upper)
	w.Write([]byte(message))
}

func create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	w.Write([]byte(
		`{
			"created": true
		}`))

}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", home)
	mux.HandleFunc("/api/", api)
	mux.HandleFunc("/api/ping", pong)
	mux.HandleFunc("/echo", echo)
	mux.HandleFunc("/create", create)
	fmt.Println("Server listening on port 8080")
	err := http.ListenAndServe(":8080", mux)

	if err != nil {
		fmt.Printf("Server returning error: %v\n", err)
	}
}
