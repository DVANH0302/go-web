# Module 2 — How HTTP Works in Go

> **Goal:** Understand not just how to write a Go web server, but what is actually happening under the hood at every step. Every concept is built from the ground up — nothing is skipped.

---

## 2.1 What Happens When a Request Arrives

Before writing any code, understand what a web server actually does at a low level.

When a browser visits `http://localhost:8080/users`:

```
1. Browser opens a TCP connection to port 8080
2. Browser sends raw text bytes over that TCP connection:

      GET /users HTTP/1.1
      Host: localhost:8080
      Accept: text/html
      (blank line)

3. Your server reads those bytes off the TCP socket
4. Parses the bytes into a structured object (method, path, headers, body)
5. Figures out which function should handle /users
6. That function runs and writes response bytes back down the TCP socket
7. Browser reads those bytes and renders the page
```

In Spring Boot, steps 3–5 are done by Tomcat + Spring MVC. You write `@GetMapping("/users")` and never see them. In Go, you interact directly with the parsed request (step 4) and write the response yourself (step 6). You still don't open the TCP socket — Go's `net` package does that — but you're one layer above it, not five.

This is why Go web code feels more manual. You're not closer to the metal because Go is primitive — Go deliberately doesn't hide the HTTP mechanics from you.

---

## 2.2 `ServeHTTP` — The One Method That Handles a Request

Go's entire HTTP system is built on a single method signature:

```go
ServeHTTP(w http.ResponseWriter, r *http.Request)
```

This is the method a handler must have. When a request comes in, Go calls this method and passes two things:

- `r *http.Request` — everything about the incoming request. You **read** from this.
- `w http.ResponseWriter` — how you send the response back. You **write** to this.

That's the whole contract. **Read from `r`, write to `w`.**

Let's look at each one properly.

---

### `*http.Request` — the incoming request

`r` is a pointer to this struct (simplified):

```go
type Request struct {
    Method string        // "GET", "POST", "PUT", "DELETE" etc.
    URL    *url.URL      // parsed URL — has .Path, .Query(), .RawQuery etc.
    Header Header        // map[string][]string — the request headers
    Body   io.ReadCloser // the request body — a readable stream of bytes
}
```

Everything the client sent is in here. You never create this yourself — Go creates it by parsing the raw bytes off the TCP socket and gives it to you.

---

### `http.ResponseWriter` — how you write the response

`w` is an interface:

```go
type ResponseWriter interface {
    Header() http.Header           // get the response header map — set headers here
    WriteHeader(statusCode int)    // send the HTTP status code + all headers
    Write([]byte) (int, error)     // send the response body
}
```

It's an interface, not a struct, because the concrete implementation (`*http.response`, unexported) wraps the raw TCP connection. When you call `w.Write(...)`, bytes go down the TCP socket to the client.

---

### Writing your first `ServeHTTP`

Now that you know what the two parameters are, let's write a type that has this method:

```go
package main

import (
    "fmt"
    "net/http"
)

type HomeHandler struct{}

func (h HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(w, "Hello from ServeHTTP")
}
```

`HomeHandler` now knows how to handle an HTTP request. But how do you connect it to a running server? That's what `http.Handler` and `ServeMux` are for — coming next.

---

### Drill 2.2-A

Write a type called `InfoHandler` with a `ServeHTTP` method that responds with:
```
Method: GET
Path: /info
```
using the actual values from `r.Method` and `r.URL.Path`. Don't register it yet — just write the type and the method. We'll register it in the next section once you understand how.

---

## 2.3 `http.Handler` — The Interface

Go formalises the `ServeHTTP` contract as an interface:

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}
```

Any type that has a `ServeHTTP(ResponseWriter, *Request)` method **satisfies this interface**. That type is then called a **handler**.

`HomeHandler` from section 2.2 satisfies `http.Handler` — it has `ServeHTTP`. Go confirms this at compile time, not runtime. If your type is missing the method or has the wrong signature, your code won't compile.

This is the same implicit interface satisfaction from Module 1 — no `implements` keyword, no annotation. If you have the method, you satisfy the interface.

**Why does this interface exist?**

Because `ServeMux` (the router) and `ListenAndServe` (the server) need a common type to work with. They don't care what your handler struct looks like — whether it has a db field, a logger, or nothing at all. They just need to know: can I call `ServeHTTP` on it? The interface is that guarantee.

---

### Drill 2.3-A

Does your `InfoHandler` from Drill 2.2-A satisfy `http.Handler`? Write a line that proves it at compile time:

```go
var _ http.Handler = InfoHandler{} // compile error if InfoHandler doesn't satisfy http.Handler
```

This is a common Go pattern for asserting interface satisfaction. The blank identifier `_` discards the value — you only care about the compile-time check.

---

## 2.4 `ServeMux` — The Router

Now you have a handler type. But a real server needs many handlers — one for `/users`, one for `/about`, one for `/health`. You need something that:

1. Keeps a map of URL patterns → handlers
2. When a request arrives, finds the right handler and calls its `ServeHTTP`

That's `ServeMux`. Internally it looks like this:

```go
// Simplified from Go's source
type ServeMux struct {
    mu sync.RWMutex        // protects the map — many goroutines read it concurrently
    m  map[string]muxEntry // pattern → handler
}

type muxEntry struct {
    h       Handler
    pattern string
}
```

It holds a map of patterns to handlers. The `sync.RWMutex` is there because hundreds of requests can read the map simultaneously — without a lock, that's a data race.

### `mux.Handle` — registering a handler

`ServeMux` has a `Handle` method:

```go
func (mux *ServeMux) Handle(pattern string, handler http.Handler)
```

It takes a pattern and anything that satisfies `http.Handler`. You use it like this:

```go
mux := http.NewServeMux()
mux.Handle("/", HomeHandler{})     // HomeHandler satisfies http.Handler
mux.Handle("/info", InfoHandler{}) // so does InfoHandler
```

When a request comes in for `/info`, the mux finds `InfoHandler` in its map and calls `InfoHandler.ServeHTTP(w, r)`.

### `ServeMux` itself satisfies `http.Handler`

Here's something important: `ServeMux` has its own `ServeHTTP` method:

```go
func (mux *ServeMux) ServeHTTP(w ResponseWriter, r *Request) {
    handler, _ := mux.match(r.URL.Path) // find the matching handler
    handler.ServeHTTP(w, r)             // call it
}
```

So `ServeMux` satisfies `http.Handler` too. It's a handler that dispatches to other handlers. This matters in the next step when we connect it to the actual server.

---

### Drill 2.4-A

Register your `InfoHandler` on `/info` using `mux.Handle`. Also register `HomeHandler` on `/`. Print the mux to confirm it compiled. Don't start a server yet — just make sure registration works without errors.

---

## 2.5 `ListenAndServe` — Starting the Server

Now you have:
- Handlers that implement `ServeHTTP`
- A mux that routes requests to them

The last piece: actually starting the server and accepting connections.

```go
http.ListenAndServe(":8080", mux)
```

The second argument is `http.Handler`. We pass the mux — and that works because `ServeMux` satisfies `http.Handler`.

Under the hood, `ListenAndServe` does this:

```go
// Simplified from Go's source
func ListenAndServe(addr string, handler Handler) error {
    ln, _ := net.Listen("tcp", addr) // open a TCP socket on port 8080
    for {
        conn, _ := ln.Accept()       // block, wait for next TCP connection
        go serveConn(conn, handler)  // handle it in a NEW goroutine
    }
}
```

The critical line: `go serveConn(conn, handler)`. Every incoming request gets its own goroutine. This is why Go handles high concurrency without a thread pool — the runtime schedules goroutines dynamically.

`serveConn` reads the raw HTTP bytes off the connection, builds a `*http.Request`, creates the `ResponseWriter`, and then calls:

```go
handler.ServeHTTP(w, r) // handler here is your mux
```

The mux's `ServeHTTP` finds the right sub-handler and calls its `ServeHTTP`. The whole system is a chain of `ServeHTTP` calls:

```
ListenAndServe
  → mux.ServeHTTP(w, r)
    → HomeHandler.ServeHTTP(w, r)
      → your code runs
```

Everything is `ServeHTTP` calling `ServeHTTP`. This is the mental model that makes middleware click later.

---

### Your first complete server — using everything so far

```go
package main

import (
    "fmt"
    "net/http"
    "log"
)

type HomeHandler struct{}

func (h HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(w, "Home page")
}

type InfoHandler struct{}

func (h InfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Method: %s\nPath: %s\n", r.Method, r.URL.Path)
}

func main() {
    mux := http.NewServeMux()
    mux.Handle("/", HomeHandler{})
    mux.Handle("/info", InfoHandler{})

    log.Print("Listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

Every line here builds on the previous sections:
- `HomeHandler` and `InfoHandler` implement `http.Handler` via `ServeHTTP`
- `mux.Handle` registers them using the `http.Handler` interface
- `ListenAndServe` accepts the mux because it too satisfies `http.Handler`
- A request comes in → `ListenAndServe` calls `mux.ServeHTTP` → mux calls the right handler's `ServeHTTP`

---

### Drill 2.5-A

Run the server above. Then use `curl -v http://localhost:8080/info`. The `-v` flag shows the raw HTTP exchange. Identify:
- The request line Go received
- The response status line
- The response headers (notice `Content-Type` — you never set it, where did it come from?)
- The response body

---

## 2.6 `http.HandlerFunc` — Functions as Handlers

Writing a new struct type for every handler is verbose. Most of the time you just want a plain function:

```go
func home(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(w, "Home page")
}
```

But this plain function doesn't satisfy `http.Handler` — it has no `ServeHTTP` method. If you try:

```go
mux.Handle("/", home) // compile error: home is not http.Handler
```

Go won't compile it. So how do you use plain functions as handlers?

### The adapter type

Go's standard library provides `http.HandlerFunc`:

```go
// From Go's source code — read this carefully
type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    f(w, r)
}
```

`HandlerFunc` is a **named type** whose underlying type is `func(ResponseWriter, *Request)`. It has a `ServeHTTP` method — which simply calls itself as a function.

This means: if you convert a plain function to `HandlerFunc`, it now satisfies `http.Handler`:

```go
mux.Handle("/", http.HandlerFunc(home)) // converts home to HandlerFunc, which satisfies Handler
```

`http.HandlerFunc(home)` is a **type conversion** — not a function call. You're converting the value `home` from type `func(ResponseWriter, *Request)` to type `http.HandlerFunc`. After conversion, it has a `ServeHTTP` method, so it satisfies `http.Handler`.

The `ServeHTTP` method just calls `f(w, r)` — which calls your original `home` function. It's a wrapper that gives a function the interface it needs.

---

### Drill 2.6-A

Write a plain function `healthCheck(w http.ResponseWriter, r *http.Request)` that writes `"ok"`. Register it using `mux.Handle` with the explicit `http.HandlerFunc` conversion. Confirm it compiles and works. Do NOT use `HandleFunc` yet — we'll get to that next.

---

## 2.7 `mux.HandleFunc` — The Shortcut

Writing `http.HandlerFunc(home)` every time is still verbose. So `ServeMux` has a shortcut:

```go
func (mux *ServeMux) HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
    mux.Handle(pattern, HandlerFunc(handler)) // does the conversion for you
}
```

`HandleFunc` takes a plain function, converts it to `HandlerFunc`, and calls `Handle`. That's the entire implementation.

So these two lines are **completely identical**:

```go
mux.Handle("/", http.HandlerFunc(home)) // explicit
mux.HandleFunc("/", home)               // shortcut — same thing
```

`HandleFunc` is just convenience. It exists so you don't have to type `http.HandlerFunc(...)` every time.

Now you understand the full chain:

```
Your plain function
  → http.HandlerFunc conversion gives it ServeHTTP
    → satisfies http.Handler
      → registered with mux.Handle
        → mux calls ServeHTTP when request matches
          → your function runs
```

`mux.HandleFunc` just collapses the first two steps into one.

---

### Drill 2.7-A

Rewrite the server from section 2.5 — but this time use plain functions and `mux.HandleFunc` instead of structs and `mux.Handle`. The behaviour should be identical. Then in a comment, explain what `mux.HandleFunc` is doing under the hood in your own words.

---

## 2.8 URL Routing — How the Mux Matches Paths

### Two kinds of patterns

**Fixed paths** — no trailing slash. Exact match only.

```go
mux.HandleFunc("/users", handleUsers)  // matches /users and ONLY /users
mux.HandleFunc("/about", handleAbout)  // matches /about and ONLY /about
```

`/users` does NOT match `/users/`, `/users/42`, or `/users?id=1`.

**Subtree paths** — trailing slash. Matches the prefix and everything below it.

```go
mux.HandleFunc("/static/", serveFiles) // matches /static/, /static/main.css, /static/img/logo.png
mux.HandleFunc("/api/", handleAPI)     // matches /api/, /api/users, /api/users/42
```

Think of the trailing slash as a wildcard: `/api/` behaves like `/api/**`.

### `/` is a catch-all

`"/"` ends with a slash so it's a subtree pattern. It matches **everything not matched by a more specific pattern**:

```go
mux.HandleFunc("/", home) // catches /, /anything, /foo/bar — all unmatched routes
```

This is a very common bug — users visiting `/typo` land on your home handler, not a 404:

```go
// BUG
func home(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Home page")) // /typo, /missing all end up here
}

// FIX
func home(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return // MUST return — http.NotFound just writes bytes, doesn't stop execution
    }
    w.Write([]byte("Home page"))
}
```

### Longest match wins

```go
mux.HandleFunc("/", home)              // matches everything
mux.HandleFunc("/api/", handleAPI)     // matches /api/*
mux.HandleFunc("/api/users", handleUsers) // matches /api/users exactly

// Request: /api/users → all three match → picks /api/users (longest)
// Request: /api/posts → first two match → picks /api/ (longest)
// Request: /about     → only / matches
```

### Never use `DefaultServeMux`

```go
// DON'T do this
http.HandleFunc("/", home)
http.ListenAndServe(":8080", nil) // nil = use global DefaultServeMux
```

`DefaultServeMux` is a global: `var DefaultServeMux = NewServeMux()`. Any package you import can register routes on it. The `net/http/pprof` package does this automatically when imported — silently exposing profiling endpoints to the internet in production.

Always create your own:

```go
mux := http.NewServeMux() // only your code touches this
```

---

### Drill 2.8-A

Create a mux with these routes:
```
/             → "home" — exact path only, 404 everything else
/api/         → responds with the full requested path
/api/ping     → responds with "pong" — should take priority over /api/
```

Test with curl:
- `/` → home
- `/about` → 404
- `/api/ping` → pong
- `/api/anything` → the path
- `/api/` → the path

Explain in a comment why `/api/ping` wins over `/api/` for that request.

---

## 2.9 The Request in Depth

### Method

```go
r.Method // "GET", "POST", "PUT", "DELETE", "PATCH" etc.
```

`ServeMux` does not filter by method. Every method lands in the same handler. You check it yourself:

```go
func handleUser(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        // fetch
    case http.MethodPost:
        // create
    default:
        w.Header().Set("Allow", "GET, POST")
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}
```

Always use constants (`http.MethodGet`, `http.MethodPost`) not strings (`"GET"`, `"POST"`). They're the same at runtime but the compiler can't catch a typo in a string literal.

### Query parameters

```go
// URL: /search?q=golang&page=2
r.URL.RawQuery          // "q=golang&page=2" — the raw unparsed string

params := r.URL.Query() // parses into url.Values = map[string][]string
params.Get("q")         // "golang"
params.Get("page")      // "2"
params.Get("missing")   // "" — safe, never panics
params["tag"]           // []string — use when multiple values: ?tag=go&tag=web
```

Query params are always strings. Convert and validate:

```go
pageStr := r.URL.Query().Get("page")
page := 1 // default
if pageStr != "" {
    var err error
    page, err = strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        http.Error(w, "invalid page", http.StatusBadRequest)
        return
    }
}
```

### Headers

```go
r.Header.Get("Content-Type")   // "application/json"
r.Header.Get("Authorization")  // "Bearer abc123"
r.Header.Get("Missing")        // "" — safe
r.Header.Values("Accept")      // []string — all values for that header
```

`Header.Get` is case-insensitive — Go canonicalises header names internally.

### The body

The body is an `io.ReadCloser` — a network stream, not a string:

```go
defer r.Body.Close() // always close — releases the connection resources

body, err := io.ReadAll(r.Body) // reads all bytes from the stream into memory
if err != nil {
    http.Error(w, "failed to read body", http.StatusBadRequest)
    return
}
// body is []byte
```

`defer r.Body.Close()` should be the first thing in any handler that reads the body. You can only read the body once — it's a stream from the network, not a buffer.

---

### Drill 2.9-A

Write a handler for `/echo` that:
- Only accepts POST (405 otherwise)
- Reads the body
- Reads the `X-Request-ID` header (default `"unknown"` if missing)
- Reads `?upper=true` query param
- If `upper=true`, responds with the body uppercased
- Prepends the request ID: `[req-id] body`

```bash
curl -X POST -H "X-Request-ID: abc123" "localhost:8080/echo?upper=true" -d "hello"
# → [abc123] HELLO
```

---

## 2.10 The Response in Depth

`ResponseWriter` has three methods. The order you call them in is strict.

```go
w.Header()           // get the header map — modify it FIRST, nothing sent yet
w.WriteHeader(code)  // sends status line + all headers — SECOND
w.Write([]byte)      // sends body — LAST
```

### Why the order is strict

An HTTP response on the wire looks like this:

```
HTTP/1.1 201 Created\r\n
Content-Type: application/json\r\n
\r\n
{"id": 1}
```

Once the status line and headers are written to the TCP socket, they are gone — you cannot change them. Go enforces this strictly:

- `w.Header()` — just gives you a map in memory. Nothing sent.
- `w.WriteHeader(code)` — flushes status line + all current headers to the wire. After this, the header map is frozen.
- `w.Write(body)` — sends body bytes. If `WriteHeader` hasn't been called yet, it automatically calls `WriteHeader(200)` first.

**The implicit `WriteHeader(200)` is the source of many bugs:**

```go
// BUG
func handler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("hello"))              // triggers implicit WriteHeader(200) — headers sent!
    w.WriteHeader(http.StatusBadRequest)  // TOO LATE — Go logs "superfluous response.WriteHeader call"
    w.Header().Set("X-Error", "true")    // TOO LATE — already sent, silently ignored
}
```

**Correct pattern:**

```go
// CORRECT
func handler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json") // 1. set headers
    w.WriteHeader(http.StatusCreated)                   // 2. send status + headers
    w.Write([]byte(`{"id": 1}`))                        // 3. send body
}
```

If you're sending 200, you can skip `WriteHeader` — the first `Write` calls it automatically.

### The JSON Content-Type gotcha

If you don't set `Content-Type`, Go sniffs the first 512 bytes of your body to guess it. It recognises HTML, PNG, PDF. It **cannot tell JSON from plain text**:

```go
w.Write([]byte(`{"name": "alice"}`))
// Go sniffs it → sets Content-Type: text/plain; charset=utf-8
// Your API clients break expecting application/json
```

Always set it explicitly:

```go
w.Header().Set("Content-Type", "application/json")
w.Write([]byte(`{"name": "alice"}`))
```

### `http.Error` shortcut

```go
// Instead of this:
w.Header().Set("Content-Type", "text/plain; charset=utf-8")
w.WriteHeader(http.StatusNotFound)
w.Write([]byte("not found"))

// Write this:
http.Error(w, "not found", http.StatusNotFound)
```

Use `http.Error` for all error responses.

---

### Drill 2.10-A

Write a handler for `/create` that:
- Only accepts POST
- Returns 201 with `Content-Type: application/json` and body `{"created": true}`

Then write a deliberately buggy version that triggers `superfluous response.WriteHeader call`. Run it, see the warning in your logs, fix it.

---

## 2.11 The `application` Struct — Dependency Injection

### The problem

Handlers need shared resources — a database, a logger, a config. Where do they come from?

In Spring Boot: `@Autowired`. In Go: there is no DI container. Instead, you use a pattern that's simpler and more explicit.

### The solution — struct with methods

Put your dependencies in a struct:

```go
type application struct {
    logger *log.Logger
    // later: db *sql.DB, config Config, etc.
}
```

Make handlers **methods** on that struct:

```go
func (app *application) home(w http.ResponseWriter, r *http.Request) {
    app.logger.Print("home called") // logger available via app
    w.Write([]byte("Home"))
}

func (app *application) getUser(w http.ResponseWriter, r *http.Request) {
    app.logger.Print("getUser called")
}
```

Wire it in `main`:

```go
func main() {
    logger := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)

    app := &application{logger: logger}

    mux := http.NewServeMux()
    mux.HandleFunc("/", app.home)
    mux.HandleFunc("/user", app.getUser)

    http.ListenAndServe(":8080", mux)
}
```

### Why `app.home` works as a handler

`app.home` is a **method value**. Go binds `app` to the method and produces a value of type `func(http.ResponseWriter, *http.Request)` — exactly what `HandleFunc` expects. When the mux calls it, `app` is already captured.

Under the hood Go is creating: `func(w http.ResponseWriter, r *http.Request) { app.home(w, r) }`.

### Why not globals?

You could put the logger and db in package-level globals. But:
- Testing is hard — you can't swap the logger or db for mocks without changing globals
- Dependencies are hidden — you can't tell what a handler needs by reading it
- The struct approach makes dependencies explicit, visible, and testable

Same principle as Spring DI — just without the magic.

---

### Drill 2.11-A

Create an `application` struct with a `logger` and a `version string`. Add two handler methods:
- `home` — responds with `"Welcome to v{version}"`
- `health` — responds with JSON `{"status":"ok","version":"{version}"}`

Set `version` to `"1.0"` in `main`. Wire both routes. Test with curl.

---

## 2.12 Putting It All Together

Here is a complete server. Every single line maps back to a section in this module:

```go
package main

import (
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strconv"
    "strings"
)

type application struct {
    logger *log.Logger
}

func (app *application) home(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {      // 2.8 — catch-all fix
        http.NotFound(w, r)
        return
    }
    w.Write([]byte("Welcome"))
}

func (app *application) getUser(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {      // 2.9 — method check
        w.Header().Set("Allow", http.MethodGet)
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    idStr := r.URL.Query().Get("id")     // 2.9 — query param
    id, err := strconv.Atoi(idStr)
    if err != nil || id < 1 {
        http.Error(w, "invalid id", http.StatusBadRequest)
        return
    }

    app.logger.Printf("fetching user id=%d", id)

    w.Header().Set("Content-Type", "application/json")  // 2.10 — set header first
    fmt.Fprintf(w, `{"id":%d,"name":"Alice"}`, id)      // 2.10 — write body
}

func (app *application) createUser(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        w.Header().Set("Allow", http.MethodPost)
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    defer r.Body.Close()                             // 2.9 — always close body
    body, err := io.ReadAll(r.Body)
    if err != nil || len(strings.TrimSpace(string(body))) == 0 {
        http.Error(w, "body required", http.StatusBadRequest)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)                // 2.10 — 201, before Write
    fmt.Fprintf(w, `{"name":"%s"}`, strings.TrimSpace(string(body)))
}

func main() {
    logger := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
    app := &application{logger: logger}

    mux := http.NewServeMux()                 // 2.4 — local mux
    mux.HandleFunc("/", app.home)             // 2.7 — HandleFunc shortcut
    mux.HandleFunc("/user", app.getUser)
    mux.HandleFunc("/user/create", app.createUser)

    logger.Print("Starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux)) // 2.5 — starts server
}
```

---

## Mini Project — In-Memory Book API

Build a complete book API from scratch.

**Routes:**
```
GET    /            → {"status":"ok"} — exact path only, 404 everything else
GET    /books       → all books as JSON array
GET    /book?id=1   → single book or 404
POST   /book        → body = book title, returns 201 + new book as JSON
DELETE /book?id=1   → removes book, returns 204 No Content
```

**Requirements:**
- `Book` struct: `ID int`, `Title string`
- `application` struct: `logger *log.Logger`, `books []Book`, `nextID int`
- All handlers are methods on `application`
- Correct `Content-Type: application/json` on all JSON responses
- Correct status codes: 200, 201, 204, 400, 404, 405
- Method check on every route
- `POST /book` → 400 if body is empty or whitespace only
- `DELETE /book` → 404 if ID not found
- `GET /book` → 400 if id param invalid, 404 if not found
- `GET /` → 404 for any path that isn't exactly `"/"`

**Test commands:**
```bash
curl -v http://localhost:8080/
curl -v http://localhost:8080/unknown
curl -v http://localhost:8080/books
curl -X POST http://localhost:8080/book \
  -H "Content-Type: application/json" \
  -d '{"Title": "The Go Programming Language"}'
curl -v -X POST http://localhost:8080/book -d "Let's Go"
curl -v "http://localhost:8080/book?id=1"
curl -v -X DELETE "http://localhost:8080/book?id=1"
curl -v http://localhost:8080/books
curl -v -X PUT http://localhost:8080/book  # expect 405
curl -v -X POST http://localhost:8080/book -d ""  # expect 400
```

---

## Book Reference

After the mini project, read these in *Let's Go*:
- **Ch. 2.2** — Web application basics → our 2.2, 2.5
- **Ch. 2.3** — Routing requests → our 2.8
- **Ch. 2.4** — Customizing HTTP headers → our 2.10
- **Ch. 2.5** — URL query strings → our 2.9
- **Ch. 2.9** — The http.Handler interface → our 2.3, 2.6, 2.7
- **Ch. 3.3** — Dependency injection → our 2.11