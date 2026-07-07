package main

import (
	"fmt"
	"net/http"
	"time"
)

type responseRecorder struct {
	statusCode int
	w          http.ResponseWriter
}

type middleware func(next http.Handler) http.Handler

func (rec *responseRecorder) Header() http.Header {
	return rec.w.Header()
}

func (rec *responseRecorder) Write(input []byte) (int, error) {
	return rec.w.Write(input)
}

func (rec *responseRecorder) WriteHeader(statusCode int) {
	rec.statusCode = statusCode
	rec.w.WriteHeader(statusCode)
}

func hello(w http.ResponseWriter, r *http.Request) {
	time.Sleep(3 * time.Second)
	w.WriteHeader(http.StatusNoContent)
}

func panic_handler(w http.ResponseWriter, r *http.Request) {
	panic("something failed")
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			responseRecorder := responseRecorder{
				w: w,
			}
			start := time.Now()
			next.ServeHTTP(&responseRecorder, r)
			duration := time.Since(start).Milliseconds()

			fmt.Printf("Method: %s, Path: %s, Status: %d, Duration: %dms\n", r.Method, r.URL.Path, responseRecorder.statusCode, duration)

		})
}

func chain(h http.Handler, mws ...middleware) http.Handler {

	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}

	return h
}

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {

			defer func() {
				if r := recover(); r != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Internal Server Error\n"))
				}
			}()

			next.ServeHTTP(w, r)
		})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)

		})
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", hello)

	mux.HandleFunc("/panic", panic_handler)

	middleware_handler := chain(mux, withRecovery, withLogging, withCORS)

	http.ListenAndServe(":8080", middleware_handler)
}
