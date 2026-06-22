package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

type application struct {
	logger  *log.Logger
	version string
}

func (a *application) home(w http.ResponseWriter, r *http.Request) {
	a.logger.Println("home called")
	message := fmt.Sprintf("Welcom to v%s", a.version)
	w.Write([]byte(message))
}

func (a *application) health(w http.ResponseWriter, r *http.Request) {
	a.logger.Println("health called")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	message := fmt.Sprintf(`{"status": "ok", "version":%s}`, a.version)
	w.Write([]byte(message))
}

func main() {

	mux := http.NewServeMux()
	app := &application{
		logger:  log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime),
		version: "1.0",
	}

	mux.HandleFunc("/home", app.home)
	mux.HandleFunc("/health", app.health)

	http.ListenAndServe(":8080", mux)
}
