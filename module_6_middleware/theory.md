# Module 6 — Middleware (Theory)

## The problem
How do you run code before/after every handler without copying it into every handler?

## The wrapping pattern
A middleware is a function that takes a `Handler` and returns a new `Handler`:

```go
func withLogging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fmt.Println("before")
        next.ServeHTTP(w, r)
        fmt.Println("after")
    })
}
```

- Code before `next.ServeHTTP` runs before the wrapped handler.
- Code after it runs after.
- This is just the `Handler` interface (Module 2) used creatively — nothing new.

## http.Handler recap
```go
type Handler interface {
    ServeHTTP(w http.ResponseWriter, r *http.Request)
}
```
Any type with a `ServeHTTP(w, r)` method satisfies it. `http.HandlerFunc` is a named
function type with `ServeHTTP` defined on it, letting you convert a plain function
into a `Handler`.

`*http.ServeMux` also satisfies `Handler` — its `ServeHTTP` matches the route and
calls the matched handler's `ServeHTTP`. It's handlers calling handlers all the way down.

## responseRecorder
Problem: `http.ResponseWriter` lets you *set* the status code (`WriteHeader`) but not
*read it back* afterward.

Fix: wrap `ResponseWriter`, embed it for free `Header()`/`Write()`, override `WriteHeader`
to remember the code before delegating:

```go
type responseRecorder struct {
    http.ResponseWriter
    statusCode int
}

func (rec *responseRecorder) WriteHeader(statusCode int) {
    rec.statusCode = statusCode
    rec.ResponseWriter.WriteHeader(statusCode)
}
```

Default `statusCode` to `http.StatusOK` (200) since Go sends 200 implicitly if a
handler just calls `Write` without ever calling `WriteHeader`.

Pass the recorder (not `w`) into `next.ServeHTTP` so every write is intercepted.

## Middleware to build from scratch
- **Logging** — method, path, status, duration on one line (uses responseRecorder)
- **Panic recovery** — `defer` + `recover()` inside the wrapper, returns 500 instead
  of crashing the server
- **CORS** — sets `Access-Control-Allow-Origin` etc. headers before calling `next`

## Chaining
Nesting middleware manually gets ugly fast:
```go
withCORS(withRecovery(withLogging(mux)))
```
Cleaner: a `chain` helper that applies a slice of middleware in order.
