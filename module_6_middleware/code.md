# Module 6 — Middleware (Code Reference)

## responseRecorder
```go
type responseRecorder struct {
    http.ResponseWriter
    statusCode int
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
    return &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rec *responseRecorder) WriteHeader(statusCode int) {
    rec.statusCode = statusCode
    rec.ResponseWriter.WriteHeader(statusCode)
}
```

## Logging middleware
```go
func withLogging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rec := newResponseRecorder(w)

        next.ServeHTTP(rec, r)

        log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.statusCode, time.Since(start))
    })
}
```

## Panic recovery middleware
```go
func withRecovery(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                w.Header().Set("Connection", "close")
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
                log.Printf("panic recovered: %v", err)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

## CORS middleware
```go
func withCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
```

## Chain helper
```go
type middleware func(http.Handler) http.Handler

func chain(h http.Handler, mws ...middleware) http.Handler {
    for i := len(mws) - 1; i >= 0; i-- {
        h = mws[i](h)
    }
    return h
}
```

## Wiring it all together
```go
mux := http.NewServeMux()
mux.HandleFunc("/books", booksHandler)

handler := chain(mux, withRecovery, withLogging, withCORS)

http.ListenAndServe(":4000", handler)
```
