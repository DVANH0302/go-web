# Module 2 — The `net/http` Package: Deep Dive

> **Calibration:** We go into the internals here. Not just "call this function" but *why* it works, what's happening under the hood, and where it breaks in production. Spring Boot hides all of this behind annotations — Go exposes it deliberately.

---

## 2.1 How Go's HTTP Server Actually Works

Before writing a single line, understand the execution model.

### The request lifecycle

```
Internet
  │
  ▼
net.Listener (TCP socket, :8080)
  │  accepts raw TCP connections
  ▼
http.Server
  │  reads bytes off the TCP connection
  │  parses HTTP/1.1 or HTTP/2 frames
  │  constructs *http.Request
  │  constructs http.ResponseWriter (wraps the TCP conn)
  │  launches a goroutine per request ← this is the key
  ▼
http.Handler.ServeHTTP(w, r)
  │  your code runs here
  ▼
ResponseWriter flushes bytes back down the TCP connection
```

**Every request runs in its own goroutine.** This is not a thread pool like Spring's Tomcat — Go's runtime schedules goroutines across OS threads dynamically. At high concurrency, you may have thousands of goroutines running simultaneously. This is why your handler code must be goroutine-safe.

### The three things you actually need

```go
http.ListenAndServe(addr string, handler http.Handler) error
```

That's the entire server. It:
1. Opens a TCP listener on `addr`
2. Accepts connections in a loop
3. For each connection, reads HTTP requests and calls `handler.ServeHTTP`

Everything else — `ServeMux`, middleware chains, routers — is just implementations of `http.Handler`.

---

## 2.2 The `Handler` Interface — The Beating Heart

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}
```

This single interface is the most important thing in Go web development. **Everything** is a `Handler` — your routes, your middleware, your entire application. If you understand this interface deeply, the rest of `net/http` falls into place.

### `HandlerFunc` — functions as handlers

Writing a struct for every handler is verbose. Go provides an adapter:

```go
type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    f(w, r)
}
```

`HandlerFunc` is a type that wraps a function and gives it a `ServeHTTP` method — making it satisfy the `Handler` interface. This is how `mux.HandleFunc` works internally:

```go
// What HandleFunc actually does:
func (mux *ServeMux) HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
    mux.Handle(pattern, HandlerFunc(handler)) // converts the func to a Handler
}
```

So `mux.HandleFunc("/", home)` is syntactic sugar for `mux.Handle("/", http.HandlerFunc(home))`. Knowing this matters when you write middleware.

### Building a server from scratch — no shortcuts

```go
package main

import (
    "fmt"
    "log"
    "net/http"
)

// Explicit struct handler — implements http.Handler directly
type App struct {
    version string
}

func (a App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "App version %s — %s %s", a.version, r.Method, r.URL.Path)
}

func main() {
    app := App{version: "1.0"}

    server := &http.Server{
        Addr:    ":8080",
        Handler: app, // app satisfies http.Handler
    }

    log.Println("listening on :8080")
    log.Fatal(server.ListenAndServe())
}
```

Notice: no `ServeMux` yet. The entire app is a single handler. Every request hits `App.ServeHTTP`. This is the simplest possible Go web server — one struct, one method.

---

## 2.3 `ServeMux` — Routing Under the Hood

`ServeMux` is itself just an `http.Handler` that does routing internally:

```go
type ServeMux struct {
    mu    sync.RWMutex        // protects the map — remember, concurrent requests
    m     map[string]muxEntry // pattern → handler
    es    []muxEntry          // sorted by pattern length, for longest-match
    hosts bool
}
```

When a request comes in, `ServeMux.ServeHTTP` locks the map, finds the longest matching pattern, and calls that handler's `ServeHTTP`. It's a dispatcher — nothing magic.

### Fixed paths vs subtree paths — the rules

```go
mux.HandleFunc("/users", handleUsers)   // fixed path — exact match only
mux.HandleFunc("/static/", serveFiles)  // subtree path — matches /static/anything
mux.HandleFunc("/", catchAll)           // subtree — matches everything not caught above
```

**Critical behaviour:** `"/"` is a subtree pattern. It matches everything. This is the most common source of "why is my 404 handler returning 200?" bugs.

```go
// This catches ALL unmatched routes — not just "/"
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    // actual home page logic
})
```

Go 1.22 changed this significantly — more in Module 3.

### Never use `DefaultServeMux` in production

```go
// This registers on the global DefaultServeMux
http.HandleFunc("/", home)
http.ListenAndServe(":8080", nil) // nil = use DefaultServeMux
```

The problem: any imported package can register routes on `DefaultServeMux`. If you import a debugging library like `net/http/pprof`, it auto-registers profiling endpoints on `DefaultServeMux`:

```go
import _ "net/http/pprof" // registers /debug/pprof/* on DefaultServeMux silently
```

In production, this exposes profiling data to the internet. Always use a locally scoped mux:

```go
mux := http.NewServeMux()
http.ListenAndServe(":8080", mux) // explicit mux — you control what's registered
```

---

## 2.4 `ResponseWriter` — What It Actually Is

```go
type ResponseWriter interface {
    Header() http.Header        // returns the header map to set before writing
    Write([]byte) (int, error)  // writes the body
    WriteHeader(statusCode int) // sends the status line + headers
}
```

The concrete type behind `ResponseWriter` is `*http.response` (unexported). It wraps the TCP connection's `bufio.Writer`. Understanding the write order is critical:

### The write order — this is where bugs happen

HTTP response structure:
```
HTTP/1.1 200 OK          ← status line (WriteHeader)
Content-Type: text/html  ← headers (Header().Set())
                         ← blank line
<html>...</html>         ← body (Write)
```

Once bytes are sent down the wire, they can't be recalled. Go enforces this:

```go
w.Header().Set("Content-Type", "application/json") // fine — not sent yet

w.WriteHeader(http.StatusOK) // sends status + all headers NOW

w.Header().Set("X-Too-Late", "ignored") // too late — headers already sent, silently ignored

w.Write([]byte(`{"ok":true}`)) // sends body
```

**Implicit `WriteHeader(200)`:** The first call to `Write` automatically calls `WriteHeader(200)` if you haven't done it yourself. This catches people out:

```go
// BUG — common mistake
func handler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("processing...")) // this implicitly sends 200 OK
    
    // ... some logic that finds an error ...
    
    w.WriteHeader(http.StatusInternalServerError) // TOO LATE — header already sent
    // Go logs: "superfluous response.WriteHeader call"
}
```

Rule: **always call `WriteHeader` before `Write`, and always set headers before `WriteHeader`.**

### Content-Type sniffing — a production gotcha

If you don't set `Content-Type`, Go calls `http.DetectContentType` on the first 512 bytes of your body to guess it. This works for HTML and images but **fails for JSON**:

```go
// BUG — sends Content-Type: text/plain; charset=utf-8
w.Write([]byte(`{"name":"alice"}`))

// CORRECT — always set Content-Type explicitly for JSON
w.Header().Set("Content-Type", "application/json")
w.Write([]byte(`{"name":"alice"}`))
```

In Spring, `@RestController` sets this for you. In Go, you do it yourself.

---

## 2.5 `*http.Request` — Anatomy

```go
type Request struct {
    Method string        // "GET", "POST", etc.
    URL    *url.URL      // parsed URL
    Header http.Header   // map[string][]string
    Body   io.ReadCloser // request body — must be closed
    
    // populated by ServeMux
    Pattern  string      // matched pattern (Go 1.22+)
    
    // context — for cancellation, deadlines, passing values
    ctx context.Context  // unexported, accessed via r.Context()
    
    // ... many more fields
}
```

### Reading the body correctly

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // Body is a ReadCloser — you must close it
    defer r.Body.Close()
    
    // Limit body size — never trust client input
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
    
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "body too large or read error", http.StatusBadRequest)
        return
    }
}
```

Without `MaxBytesReader`, a client can send a 10GB body and exhaust your server's memory. Spring Boot has `spring.servlet.multipart.max-file-size` for this — in Go you do it explicitly.

### Header access — case insensitive, slice values

```go
// Headers are canonicalised — these are all equivalent
r.Header.Get("content-type")
r.Header.Get("Content-Type")
r.Header.Get("CONTENT-TYPE")

// Get returns first value only
ct := r.Header.Get("Content-Type") // "application/json"

// Values returns all values for a header (e.g. multiple Accept headers)
accepts := r.Header.Values("Accept") // []string{"text/html", "application/json"}

// Direct map access — NOT canonicalised, be careful
r.Header["Content-Type"] // []string{"application/json"}
```

### URL and query parameters

```go
// Full URL: /search?q=golang&page=2
r.URL.Path     // "/search"
r.URL.RawQuery // "q=golang&page=2"

// Parsed query params
q := r.URL.Query()         // returns url.Values (map[string][]string)
q.Get("q")                 // "golang" — first value
q.Get("missing")           // "" — safe, no panic
q["page"]                  // []string{"2"} — direct map access
```

---

## 2.6 Writing Responses — The Complete Picture

### Status codes — use the constants

```go
// Never hardcode integers
w.WriteHeader(200) // bad — magic number
w.WriteHeader(http.StatusOK) // good — self-documenting

// Common ones you'll use daily
http.StatusOK                   // 200
http.StatusCreated              // 201
http.StatusNoContent            // 204
http.StatusBadRequest           // 400
http.StatusUnauthorized         // 401
http.StatusForbidden            // 403
http.StatusNotFound             // 404
http.StatusMethodNotAllowed     // 405
http.StatusUnprocessableEntity  // 422
http.StatusInternalServerError  // 500
```

### The three response patterns

**Pattern 1 — plain text error (use `http.Error`)**
```go
http.Error(w, "not found", http.StatusNotFound)
// equivalent to:
w.Header().Set("Content-Type", "text/plain; charset=utf-8")
w.Header().Set("X-Content-Type-Options", "nosniff")
w.WriteHeader(http.StatusNotFound)
fmt.Fprintln(w, "not found")
```

**Pattern 2 — JSON response**
```go
func writeJSON(w http.ResponseWriter, status int, v any) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    return json.NewEncoder(w).Encode(v)
}
```

**Pattern 3 — streaming / large responses**
```go
// json.NewEncoder writes directly to w — no intermediate buffer
// good for large responses
json.NewEncoder(w).Encode(largeSlice)

// vs json.Marshal — buffers entire response in memory first
data, _ := json.Marshal(largeSlice)
w.Write(data)
```

For large datasets, always use the encoder to avoid memory spikes.

---

## 2.7 Middleware — The Handler Wrapping Pattern

This is where Go's interface design pays off. Middleware in Go is **a function that takes a Handler and returns a Handler**.

```go
type Middleware func(http.Handler) http.Handler
```

It wraps your handler, running code before and/or after:

```go
func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        next.ServeHTTP(w, r) // call the wrapped handler
        
        log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
    })
}
```

Applying it:

```go
mux := http.NewServeMux()
mux.HandleFunc("/", home)

// wrap the entire mux in Logger middleware
http.ListenAndServe(":8080", Logger(mux))
```

### Execution order — this trips people up

```go
func A(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fmt.Println("A before")
        next.ServeHTTP(w, r)
        fmt.Println("A after")
    })
}

func B(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fmt.Println("B before")
        next.ServeHTTP(w, r)
        fmt.Println("B after")
    })
}

handler := A(B(mux))
// Output for a request:
// A before
// B before
// [handler runs]
// B after
// A after
```

The outermost middleware runs first before, and last after. Like nested function calls — the call stack unwinds in reverse.

In Spring, this is `@Order` on `Filter` beans. In Go, the nesting order is explicit and obvious.

### Capturing the status code in middleware

`ResponseWriter` doesn't expose the status code after it's been written. For logging, you need to intercept it:

```go
type responseRecorder struct {
    http.ResponseWriter
    status int
    written bool
}

func (rr *responseRecorder) WriteHeader(status int) {
    if !rr.written {
        rr.status = status
        rr.written = true
        rr.ResponseWriter.WriteHeader(status)
    }
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
    if !rr.written {
        rr.WriteHeader(http.StatusOK) // capture implicit 200
    }
    return rr.ResponseWriter.Write(b)
}

func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rr := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
        start := time.Now()
        
        next.ServeHTTP(rr, r)
        
        log.Printf("%d %s %s %v", rr.status, r.Method, r.URL.Path, time.Since(start))
    })
}
```

This pattern — embedding `ResponseWriter` and overriding specific methods — is used everywhere in production Go web code.

---

## 2.8 Structuring Your App — Beyond `main.go`

This is where Spring Boot's `@Autowired` DI container would normally manage dependencies. In Go, you do dependency injection manually through struct fields.

```go
type Application struct {
    logger   *slog.Logger
    db       *sql.DB
    config   Config
}

// handlers are methods on Application — they close over its dependencies
func (app *Application) handleGetUser(w http.ResponseWriter, r *http.Request) {
    // app.db is available here — no global, no DI container
    // app.logger is available here
}

func (app *Application) routes() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/users", app.handleGetUser)
    mux.HandleFunc("/health", app.handleHealth)
    
    // wrap the whole mux in middleware
    return Logger(mux)
}

func main() {
    app := &Application{
        logger: slog.Default(),
        db:     mustConnectDB(),
        config: loadConfig(),
    }
    
    http.ListenAndServe(":8080", app.routes())
}
```

This pattern — `Application` struct with handler methods — is the idiomatic Go alternative to Spring's component scanning. Simple, explicit, testable.

---

## 2.9 `http.Server` — Production Configuration

`http.ListenAndServe` is fine for learning but **never use it in production**. It uses default timeouts — which means no timeouts at all.

```go
// NEVER in production — no timeouts
http.ListenAndServe(":8080", mux)

// Correct — always configure timeouts
server := &http.Server{
    Addr:    ":8080",
    Handler: mux,
    
    // Time to read the entire request including body
    ReadTimeout: 5 * time.Second,
    
    // Time to write the response
    WriteTimeout: 10 * time.Second,
    
    // Time for idle keep-alive connections
    IdleTimeout: 120 * time.Second,
    
    // Time to read request headers only (subset of ReadTimeout)
    ReadHeaderTimeout: 2 * time.Second,
}

log.Fatal(server.ListenAndServe())
```

Without `ReadTimeout`, a slow client can hold a connection open forever (Slowloris attack). Without `WriteTimeout`, a slow client receiving a large response ties up your goroutine indefinitely.

### Graceful shutdown

```go
func main() {
    server := &http.Server{Addr: ":8080", Handler: mux}
    
    // Start server in a goroutine
    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()
    
    // Wait for interrupt signal (Ctrl+C, SIGTERM from Kubernetes)
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    log.Println("shutting down...")
    
    // Give in-flight requests 30 seconds to complete
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Fatal("forced shutdown:", err)
    }
}
```

In a Kubernetes deployment, `SIGTERM` is sent when a pod is being replaced. Without graceful shutdown, in-flight requests get killed mid-response. Spring Boot has `server.shutdown=graceful` — in Go you wire this yourself.

---

## Drills

### Drill 2.1 — Handler interface
Without using `mux.HandleFunc`, create a type `Router` that implements `http.Handler`. It should store a `map[string]http.HandlerFunc` and dispatch requests by `r.URL.Path`. Return 404 for unregistered paths. Register two routes and verify both work.

### Drill 2.2 — Response header ordering bug
Write a handler that deliberately triggers the "superfluous WriteHeader" bug. Run it, hit the endpoint, observe the log warning. Then fix it. The point is to see the warning message so you recognise it in production.

### Drill 2.3 — `responseRecorder` middleware
Implement the `Logger` middleware from section 2.7 completely — capturing status code, method, path, and duration. It should log a line like:
```
200 GET /users 423µs
```
Apply it to a mux with two routes and verify the log output for each.

### Drill 2.4 — Body limiting
Write a handler that reads a request body. Without `MaxBytesReader`, send a 2MB body and observe it reads fine. Then add `MaxBytesReader` with a 1KB limit and observe the error. Handle it correctly with a 400 response.

---

## Mini Project — JSON API Server with Middleware

Build an HTTP server with the following:

**Routes:**
```
GET  /health          → {"status":"ok","version":"1.0"}
GET  /users           → list of hardcoded users as JSON
POST /users           → read JSON body, echo it back with an ID added
GET  /users/{id}      → return user by ID or 404 (hint: parse ID from URL path manually)
```

**Requirements:**
- `Application` struct holds a `version string` and `users []User`
- All handlers are methods on `Application`
- A `Logger` middleware logs: status code, method, path, duration
- A `RecoverPanic` middleware catches panics and returns 500 (don't let the server crash)
- Both middleware applied to the entire mux
- Proper `Content-Type: application/json` on all responses
- Proper `http.Server` with timeouts configured
- `POST /users` must validate the body isn't empty and return 400 if it is

**What this tests:**
- Interface satisfaction
- Struct-based dependency injection
- Middleware chaining
- Manual URL parsing (before we get to Go 1.22 path params in Module 3)
- Error handling in handlers
- Response write ordering

**Stretch goal:** Add a `RateLimit` middleware that allows max 10 requests per second using a `time.Ticker` and a buffered channel as a token bucket. This is harder — think about what happens when the bucket is full.

---

## Book Reference

- **Chapter 2.1–2.2** — Project setup, `ListenAndServe`, first handler
- **Chapter 2.3** — `ServeMux`, fixed vs subtree patterns
- **Chapter 2.4** — `WriteHeader`, `Header().Set()`, `http.Error`
- **Chapter 2.9** — `http.Handler` interface (the book calls this out explicitly)
- **Chapter 3.3** — Dependency injection via `Application` struct

Read these after completing the mini project — you'll find Alex's explanations click much faster when you've already wrestled with the concepts yourself.