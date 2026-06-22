# Module 2 — How HTTP Works in Go

> **Goal:** By the end of this module you will be able to build a working HTTP server from scratch and understand *exactly* what every line does and why it's there. No magic, no skipping steps.

---

## 2.1 What Actually Happens When a Server Receives a Request

Before writing any code, let's understand what a web server actually does — because Go makes this very visible, unlike Spring Boot which hides it behind annotations.

When a browser visits `http://localhost:8080/users`, this happens:

```
1. Browser opens a TCP connection to your machine on port 8080
2. Browser sends raw text over that connection:

   GET /users HTTP/1.1
   Host: localhost:8080
   Accept: text/html

3. Your server reads those bytes
4. Your server parses them into a structured request object
5. Your server figures out which function should handle /users
6. That function runs and writes a response back
7. Browser reads the response bytes and renders the page
```

In Spring Boot, steps 3–5 are handled by Tomcat and Spring MVC — you just write a `@GetMapping("/users")` method. In Go, you are much closer to steps 3–5. You still don't do TCP yourself, but you interact with the parsed request and write the response directly.

This is why Go web code feels more "manual" at first — you're actually seeing what's happening.

---

## 2.2 Your First HTTP Server — Line by Line

Here is the simplest possible Go web server:

```go
package main

import (
    "log"
    "net/http"
)

func home(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Hello from Go"))
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", home)

    log.Print("Starting server on :8080")
    err := http.ListenAndServe(":8080", mux)
    log.Fatal(err)
}
```

Let's go through every single line and understand it.

---

### `mux := http.NewServeMux()`

`mux` is short for **multiplexer** — a router. Its job is simple: look at the incoming request URL, find which handler matches, and call it.

Think of it like a switchboard operator. A request comes in for `/users`, the mux looks at its table, finds the handler registered for `/users`, and calls it.

In Spring Boot, the router is built into the framework and you configure it with annotations like `@GetMapping`. In Go, the mux is an explicit object you create and control.

---

### `mux.HandleFunc("/", home)`

This registers the `home` function as the handler for the `/` path.

`HandleFunc` takes two things:
- A URL pattern (`"/"`)
- A function with a specific signature: `func(http.ResponseWriter, *http.Request)`

Any function that has exactly this signature can be a handler. That's it. No annotations, no special base class.

---

### `func home(w http.ResponseWriter, r *http.Request)`

Every handler in Go takes exactly these two parameters. Always. Let's understand what they are.

**`r *http.Request`** — this is the incoming request. It's a pointer to a struct that contains everything about the request: the URL, the HTTP method (GET/POST/etc), the headers, the body. You *read* from this.

**`w http.ResponseWriter`** — this is how you write the response back to the client. You *write* to this.

The pattern is: read from `r`, write to `w`.

---

### `w.Write([]byte("Hello from Go"))`

`Write` sends bytes back to the client as the response body. It takes `[]byte`, not `string` — that's why we convert with `[]byte("Hello from Go")`.

When you call this, Go sends:
```
HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8
Content-Length: 13

Hello from Go
```

Notice Go automatically adds the status code (200 OK) and some headers. You didn't write those — Go did it automatically because you didn't set them yourself.

---

### `http.ListenAndServe(":8080", mux)`

This does three things:
1. Opens a TCP socket on port 8080
2. Starts listening for incoming connections
3. For each connection, reads the HTTP request, matches it against the mux, calls the right handler

The second argument is the mux — the thing that decides which handler to call.

This function **blocks forever** — it keeps running until the server crashes or you kill the process. That's why we put `log.Fatal(err)` after it — if `ListenAndServe` ever returns, something went wrong, and we want to log the error and exit.

---

## 2.3 The Handler Interface — Why Everything in Go HTTP Works

You just saw `mux.HandleFunc("/", home)` register a function as a handler.

But Go also has `mux.Handle("/", something)` — notice no `Func` at the end.

What's the difference? And why does this matter?

To answer this, we need to understand the `http.Handler` interface.

### The interface

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}
```

That's the entire interface. Any type in Go that has a `ServeHTTP(ResponseWriter, *Request)` method **is** a handler. It can be registered with the mux, it can be passed to `ListenAndServe` — anything that expects a `Handler`.

**Why is this important?** Because it means the entire Go HTTP ecosystem is built on one simple contract. The mux is a `Handler`. Your handlers are `Handler`s. Middleware (which we'll cover) wraps one `Handler` with another `Handler`. Everything fits together because everything implements the same interface.

Compare this to Spring Boot where the framework uses reflection and annotations to figure out what's a controller, what's a filter, what's a bean. In Go, it's just: does it have `ServeHTTP`? Then it's a handler.

### Implementing the interface yourself

```go
type HomeHandler struct{}

func (h HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Hello from HomeHandler"))
}

func main() {
    mux := http.NewServeMux()
    mux.Handle("/", HomeHandler{})  // mux.Handle, not HandleFunc
    http.ListenAndServe(":8080", mux)
}
```

`HomeHandler` has a `ServeHTTP` method, so it satisfies `http.Handler`. The mux accepts it with `mux.Handle`.

### So what is `HandleFunc` then?

The problem with the struct approach above is — you have to create a struct just to hold one function. That's verbose. Most of the time you just want to register a plain function.

But a plain function doesn't have a `ServeHTTP` method. So how do you register it?

Go provides `http.HandlerFunc` — a type that solves this:

```go
// This is from Go's standard library
type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    f(w, r)
}
```

`HandlerFunc` is a **type** whose underlying type is `func(ResponseWriter, *Request)`. And it has a `ServeHTTP` method — which just calls itself.

So when you write:

```go
mux.HandleFunc("/", home)
```

Go is doing this internally:

```go
mux.Handle("/", http.HandlerFunc(home))
```

It wraps your function in `HandlerFunc` to give it a `ServeHTTP` method — making it satisfy `http.Handler`. Then registers it with the mux.

`HandleFunc` is just a shortcut for this conversion + registration.

### Why the mux itself is a Handler

Here's something interesting: `http.ListenAndServe(":8080", mux)` takes an `http.Handler` as the second argument. But we're passing a mux.

That works because **the mux itself is a Handler**. It has a `ServeHTTP` method. When a request comes in, `ListenAndServe` calls `mux.ServeHTTP(w, r)`. The mux then figures out which sub-handler to call, and calls *its* `ServeHTTP`.

So the whole system is just `ServeHTTP` calling `ServeHTTP` calling `ServeHTTP` — a chain of handlers. This is the mental model that makes middleware click (Module 7).

---

## 2.4 URL Routing — How the Mux Matches Paths

The mux matches request paths to handlers using **pattern matching**. There are two kinds of patterns.

### Fixed paths

```go
mux.HandleFunc("/users", handleUsers)
mux.HandleFunc("/about", handleAbout)
```

A fixed path has **no trailing slash**. It only matches the exact URL. `/users` matches `/users` and nothing else. `/users/` (with trailing slash) would NOT match.

### Subtree paths

```go
mux.HandleFunc("/static/", serveStatic)
mux.HandleFunc("/", catchAll)
```

A subtree path **has a trailing slash**. It matches the path AND anything below it. `/static/` matches `/static/`, `/static/main.css`, `/static/img/logo.png` — anything starting with `/static/`.

The `/` pattern is a subtree path too. It matches everything — every URL that isn't matched by a more specific pattern. This makes it a catch-all.

### Longest match wins

If multiple patterns match a URL, the most specific one (longest) wins.

```go
mux.HandleFunc("/", catchAll)        // matches everything
mux.HandleFunc("/users", handleUsers) // matches /users exactly
```

A request to `/users` matches both. The mux picks `/users` because it's longer/more specific.

### The `/` catch-all problem

Because `/` matches everything, if you register it, every unmatched URL goes to your home handler — not to a 404. This is a common gotcha:

```go
func home(w http.ResponseWriter, r *http.Request) {
    // BUG: /anything/not/registered lands here too
    w.Write([]byte("Home page"))
}
```

Fix it by checking the path explicitly:

```go
func home(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return  // important — stop execution here
    }
    w.Write([]byte("Home page"))
}
```

The `return` is critical. Without it, even after calling `http.NotFound`, execution continues and you'd write the home page body too.

### Don't use `DefaultServeMux`

You might see Go code that does this:

```go
http.HandleFunc("/", home)     // no mux variable
http.ListenAndServe(":8080", nil) // nil = use default mux
```

This uses a global mux called `DefaultServeMux`. The problem: any third-party package you import can register routes on it silently. Some packages (like `net/http/pprof` for profiling) do this automatically on import. In production, you'd be exposing internal profiling endpoints to the internet without knowing.

Always create your own mux:

```go
mux := http.NewServeMux() // your mux, nobody else can touch it
```

---

## 2.5 The Request — Reading What the Client Sent

The `*http.Request` struct contains everything the client sent. Let's look at the parts you'll use constantly.

### The HTTP method

```go
r.Method // "GET", "POST", "PUT", "DELETE", etc.
```

In Spring Boot you'd use `@GetMapping` to restrict a handler to GET. In Go, the basic `ServeMux` doesn't do method filtering — you do it yourself:

```go
func handleSnippet(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        w.Header().Set("Allow", http.MethodPost)
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }
    // handle POST
}
```

Use the constants (`http.MethodPost`, `http.MethodGet`) instead of strings like `"POST"` — they prevent typos and make the code self-documenting.

### The URL

```go
r.URL.Path     // "/users/42"
r.URL.RawQuery // "sort=asc&page=2"
```

To read query parameters (the `?key=value` part of the URL):

```go
// URL: /search?q=golang&page=2

q := r.URL.Query()       // parses the query string into a map
search := q.Get("q")    // "golang"
page := q.Get("page")   // "2" — always a string
missing := q.Get("foo") // "" — safe, no panic if key doesn't exist
```

Query params are always strings. If you need an integer:

```go
pageStr := r.URL.Query().Get("page")
page, err := strconv.Atoi(pageStr)
if err != nil || page < 1 {
    http.NotFound(w, r)
    return
}
```

Never trust query parameters. Always validate them.

### Headers

```go
contentType := r.Header.Get("Content-Type") // "application/json"
auth := r.Header.Get("Authorization")        // "Bearer abc123"
```

`Header.Get` is case-insensitive — `"content-type"` and `"Content-Type"` return the same thing.

### The body

```go
defer r.Body.Close() // always close the body when done

body, err := io.ReadAll(r.Body)
if err != nil {
    http.Error(w, "failed to read body", http.StatusBadRequest)
    return
}
// body is []byte
```

Two things to always do with the body:
1. `defer r.Body.Close()` — the body is a network stream, you must close it when done
2. Validate/limit its size — a client could send a 10GB body and crash your server

We'll cover body limiting and JSON parsing in Module 4.

---

## 2.6 The Response — Writing Back to the Client

`http.ResponseWriter` is how you send a response. It has three methods:

```go
w.Header()          // returns the header map — set headers here
w.WriteHeader(code) // sends the status code
w.Write([]byte)     // sends the body
```

### The order matters — a lot

HTTP responses have a specific structure:

```
HTTP/1.1 200 OK          ← status line
Content-Type: text/html  ← headers
                         ← blank line separating headers from body
<html>...</html>         ← body
```

Once the status line and headers are sent, they cannot be changed. Go enforces this:

**Rule: Set headers → WriteHeader → Write. In that order.**

```go
// CORRECT
w.Header().Set("Content-Type", "application/json")  // 1. set headers
w.WriteHeader(http.StatusCreated)                    // 2. send status
w.Write([]byte(`{"id": 1}`))                         // 3. send body
```

What happens if you get the order wrong?

```go
// BUG
w.Write([]byte("hello"))                    // implicitly sends 200 + headers
w.WriteHeader(http.StatusInternalServerError) // TOO LATE — Go logs a warning, status ignored
w.Header().Set("X-Custom", "value")         // TOO LATE — header already sent, silently ignored
```

The first call to `Write` automatically sends `WriteHeader(200)` if you haven't called it yet. After that, headers are gone. Status is gone.

The warning you'll see in the logs: `superfluous response.WriteHeader call`. If you see this, you have a write-order bug.

### Setting headers

```go
w.Header().Set("Content-Type", "application/json")  // set (overwrites)
w.Header().Add("Cache-Control", "no-store")          // add (appends)
w.Header().Del("X-Unwanted")                         // delete
```

Always set headers **before** calling `WriteHeader` or `Write`.

### Status codes

```go
w.WriteHeader(http.StatusOK)                  // 200
w.WriteHeader(http.StatusCreated)             // 201
w.WriteHeader(http.StatusBadRequest)          // 400
w.WriteHeader(http.StatusNotFound)            // 404
w.WriteHeader(http.StatusInternalServerError) // 500
```

If you don't call `WriteHeader` at all, Go sends 200 automatically when you first call `Write`. So you only need to call `WriteHeader` explicitly when you're sending a non-200 status.

### `http.Error` — the shortcut for error responses

For error responses, instead of manually setting headers + WriteHeader + Write:

```go
http.Error(w, "not found", http.StatusNotFound)
```

This sets `Content-Type: text/plain`, calls `WriteHeader(404)`, and writes the message. Use this for any non-200 plain-text response.

### Content-Type sniffing — the JSON gotcha

If you don't set `Content-Type`, Go tries to guess it by looking at the first 512 bytes of your response body. It works for HTML and images. **It does not work for JSON** — Go can't tell JSON from plain text, so it sets `Content-Type: text/plain`.

Always set it explicitly for JSON:

```go
w.Header().Set("Content-Type", "application/json")
w.Write([]byte(`{"name": "alice"}`))
```

This is one of the most common bugs in Go JSON APIs written by beginners.

---

## 2.7 Structuring Your App — The `application` Struct Pattern

Right now our handler is a standalone function. But in a real app, handlers need access to shared things — a database connection, a logger, a config. How do you give them access?

In Spring Boot, you'd use `@Autowired`. Go has no DI container. Instead, the idiomatic pattern is: **put your dependencies in a struct, make your handlers methods on that struct**.

```go
type application struct {
    logger *log.Logger
    db     *sql.DB     // we'll add this in Module 6
}

func (app *application) home(w http.ResponseWriter, r *http.Request) {
    // app.logger is available here
    // app.db is available here
    app.logger.Print("home handler called")
    w.Write([]byte("Home"))
}

func (app *application) about(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("About"))
}
```

And in `main`, you create the struct and wire everything together:

```go
func main() {
    logger := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)

    app := &application{
        logger: logger,
    }

    mux := http.NewServeMux()
    mux.HandleFunc("/", app.home)     // app.home is a method — satisfies handler signature
    mux.HandleFunc("/about", app.about)

    http.ListenAndServe(":8080", mux)
}
```

`app.home` is a **method value** — Go automatically binds `app` to the method call. When the mux calls `app.home(w, r)`, it's equivalent to `application.home(app, w, r)`. The handler gets access to everything in `app`.

This is the Go equivalent of `@Autowired` — explicit, type-safe, and you can see exactly what each handler depends on.

Why not use global variables for the logger and db? You could, but:
- It makes testing harder — you can't swap out the logger or db in tests
- It's harder to reason about — globals can be changed from anywhere
- The struct approach makes dependencies explicit and visible

---

## 2.8 Putting It All Together — A Working Server

Let's build a complete, working server using everything from this module.

```go
package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "strconv"
)

type application struct {
    logger *log.Logger
}

func (app *application) home(w http.ResponseWriter, r *http.Request) {
    // Restrict to exact "/" path — avoid catch-all behaviour
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    w.Write([]byte("Welcome to the home page"))
}

func (app *application) getUser(w http.ResponseWriter, r *http.Request) {
    // Only allow GET
    if r.Method != http.MethodGet {
        w.Header().Set("Allow", http.MethodGet)
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }

    // Read and validate query param
    idStr := r.URL.Query().Get("id")
    id, err := strconv.Atoi(idStr)
    if err != nil || id < 1 {
        http.Error(w, "Invalid or missing id", http.StatusBadRequest)
        return
    }

    app.logger.Printf("fetching user id=%d", id)

    // Set header BEFORE writing
    w.Header().Set("Content-Type", "application/json")
    fmt.Fprintf(w, `{"id": %d, "name": "Alice"}`, id)
}

func (app *application) createUser(w http.ResponseWriter, r *http.Request) {
    // Only allow POST
    if r.Method != http.MethodPost {
        w.Header().Set("Allow", http.MethodPost)
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }

    // Acknowledge creation
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated) // 201
    w.Write([]byte(`{"message": "user created"}`))
}

func main() {
    logger := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)

    app := &application{logger: logger}

    mux := http.NewServeMux()
    mux.HandleFunc("/", app.home)
    mux.HandleFunc("/user", app.getUser)
    mux.HandleFunc("/user/create", app.createUser)

    logger.Print("Starting server on :8080")
    err := http.ListenAndServe(":8080", mux)
    log.Fatal(err)
}
```

Run this and test it:
```bash
curl http://localhost:8080/
curl "http://localhost:8080/user?id=5"
curl -X POST http://localhost:8080/user/create
curl -X DELETE http://localhost:8080/user  # should get 405
curl http://localhost:8080/unknown          # should get 404
```

Every line in this server should make sense to you now.

---

## Drills

### Drill 2.1 — Handler interface directly
Without using `HandleFunc`, create a struct `StatusHandler` that implements `http.Handler` directly (with a `ServeHTTP` method). It should read a `code` field from the struct and always respond with that status code and the status text as the body. Register it on two different routes with different codes — e.g. `/teapot` returns 418, `/ok` returns 200.

### Drill 2.2 — Write order bug
Write a handler that intentionally triggers the "superfluous response.WriteHeader call" warning. Run it, hit the endpoint, see the warning in your logs. Then explain in a comment *why* the warning happened and fix it.

### Drill 2.3 — Query param validation
Write a handler for `/search` that reads three query params: `q` (string, required), `page` (int, must be > 0, defaults to 1 if missing), `limit` (int, must be between 1 and 100, defaults to 10). Return a 400 with a descriptive error message for each validation failure. Return a JSON object with the validated values on success.

---

## Mini Project — Multi-route API with Application Struct

Build a server that manages an in-memory list of books.

**Routes:**
```
GET  /           → {"status": "ok"}
GET  /books      → list all books as JSON array
GET  /book?id=1  → return single book by id, or 404
POST /book       → read body as plain text (the book title), add it, return 201 with the new book as JSON
```

**Requirements:**
- `application` struct holds the logger and a `[]Book` slice
- `Book` struct has `ID int` and `Title string`
- All handlers are methods on `application`
- Correct `Content-Type: application/json` on all JSON responses
- Correct status codes (200, 201, 400, 404, 405)
- Method checking on every route
- `/` must return 404 for any path that isn't exactly `"/"`
- `POST /book` must return 400 if the body is empty

**What this tests:**
- `application` struct and dependency injection
- `Handler` interface understanding (you should know what `mux.HandleFunc` is doing)
- Write ordering
- Method validation
- Query param validation
- Reading the request body

---

## Book Reference (Let's Go by Alex Edwards)

Read these chapters after finishing the mini project:
- **Chapter 2.2** — Web application basics (matches section 2.2 above)
- **Chapter 2.3** — Routing requests (matches section 2.4 above)
- **Chapter 2.4** — Customizing HTTP headers (matches section 2.6 above)
- **Chapter 2.5** — URL query strings (matches section 2.5 above)
- **Chapter 2.9** — The http.Handler interface (matches section 2.3 above — the book calls this "a bit complicated", but you now understand it)
- **Chapter 3.3** — Dependency injection (matches section 2.7 above)

You'll find the book builds the same concepts into a real app called Snippetbox. Read it as a second pass to see these patterns in context.