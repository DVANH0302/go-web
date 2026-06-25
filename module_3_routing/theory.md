# Module 3 — Routing

> **Goal:** Understand how routing works deeply in Go — from the basic ServeMux you already know, to Go 1.22's new features that eliminate most reasons to use a third-party router.

---

## 3.1 What You Already Know — and What's Missing

In Module 2 you built a working API. But you hit two frustrating limitations:

**1. No method routing in the mux**

You had to check `r.Method` manually in every handler:

```go
func (a *application) bookHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    // actual logic
}
```

**2. No path parameters**

To get a book by ID you used query params (`/book?id=1`) instead of a clean URL (`/book/1`). That's because the old `ServeMux` had no way to extract values from URL paths.

Both of these were real limitations that pushed people toward third-party routers like `gorilla/mux` or `chi`.

**Go 1.22 fixed both of these.** If you're on Go 1.22 or later (you should be), the standard `ServeMux` can do method routing and path parameters natively.

---

## 3.2 Go 1.22 Enhanced ServeMux — Method Routing

Before Go 1.22, patterns were just paths:

```go
mux.HandleFunc("/books", handler) // old — any method hits this
```

From Go 1.22, you can prefix the pattern with an HTTP method:

```go
mux.HandleFunc("GET /books", handler)    // only GET
mux.HandleFunc("POST /books", handler)   // only POST
mux.HandleFunc("DELETE /books", handler) // only DELETE
```

**Under the hood**, the mux now parses the pattern as `METHOD PATH` — it splits on the space. When a request arrives, it matches both the method AND the path. If the path matches but the method doesn't, the mux automatically returns `405 Method Not Allowed` — you don't have to write that check yourself.

This means your mini project's `bookHandler` goes from this:

```go
func (a *application) bookHandler(w http.ResponseWriter, r *http.Request) {
    allowedMethods := []string{http.MethodGet, http.MethodPost, http.MethodDelete}
    if !slices.Contains(allowedMethods, r.Method) {
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }
    switch r.Method {
    case http.MethodGet:
        // get logic
    case http.MethodPost:
        // post logic
    case http.MethodDelete:
        // delete logic
    }
}
```

To this — one handler per method, clean and focused:

```go
mux.HandleFunc("GET /book", app.getBook)
mux.HandleFunc("POST /book", app.createBook)
mux.HandleFunc("DELETE /book", app.deleteBook)

func (a *application) getBook(w http.ResponseWriter, r *http.Request) {
    // only GET lands here — no method check needed
}

func (a *application) createBook(w http.ResponseWriter, r *http.Request) {
    // only POST lands here
}
```

Each handler has one job. Much cleaner.

### What happens with unmatched methods

If you register `GET /books` and a `POST /books` request comes in:

```
GET /books registered
POST /books NOT registered

Request: POST /books
→ mux finds /books matches the path
→ but POST doesn't match GET
→ mux automatically responds: 405 Method Not Allowed
→ mux sets the Allow header: "GET" (what's actually registered)
```

You get correct 405 behaviour for free. No code needed.

### `GET` also matches `HEAD`

When you register `GET /books`, the mux automatically handles `HEAD /books` too. `HEAD` is identical to `GET` but the response has no body — the mux strips the body for you. This is correct HTTP behaviour and you get it for free.

---

### Drill 3.2-A

Rewrite your mini project's `bookHandler` using Go 1.22 method routing. You should have three separate handler functions — `getBook`, `createBook`, `deleteBook` — each registered with its own method prefix. Remove all `r.Method` checks from inside the handlers. Test that sending the wrong method returns 405 automatically.

---

## 3.3 Path Parameters — `/book/{id}`

The second big Go 1.22 addition is **wildcards** in URL patterns — the `{name}` syntax.

```go
mux.HandleFunc("GET /book/{id}", app.getBook)
```

`{id}` is a wildcard — it matches any single path segment. A request to `/book/42` matches, and you can extract `"42"` from the URL:

```go
func (a *application) getBook(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id") // "42" — extracted from the URL
    // id is always a string — convert if needed
    bookID, err := strconv.Atoi(id)
    if err != nil {
        http.Error(w, "invalid id", http.StatusBadRequest)
        return
    }
}
```

`r.PathValue("id")` is a new method on `*http.Request` added in Go 1.22. It returns the value of the named wildcard from the URL.

### How wildcards work under the hood

When you register `GET /book/{id}`, the mux stores a pattern with a wildcard segment. When a request arrives for `/book/42`:

```
Pattern:  /book/{id}
Request:  /book/42

Segment 1: "book" == "book" ✅
Segment 2: "42" matches {id} ✅ — stored as id="42"
```

The mux stores the extracted values on the request context (using `r.WithContext` internally). `r.PathValue("id")` reads them back out of the context.

### Wildcard rules

```go
// single segment wildcard — matches one path segment only
"GET /book/{id}"     // matches /book/42, /book/abc
                     // does NOT match /book/42/extra

// trailing slash wildcard — matches everything after
"GET /files/{path...}" // matches /files/a, /files/a/b, /files/a/b/c
```

The `{name...}` syntax (with `...`) is a **remainder wildcard** — it matches multiple segments. Used for things like file paths.

### Exact vs wildcard priority

If you have both a specific path and a wildcard, the specific one always wins:

```go
mux.HandleFunc("GET /book/new", app.newBookForm)  // specific
mux.HandleFunc("GET /book/{id}", app.getBook)     // wildcard

// GET /book/new → hits newBookForm (specific wins)
// GET /book/42  → hits getBook (wildcard)
// GET /book/abc → hits getBook (wildcard)
```

The mux always picks the most specific matching pattern — same longest-match rule from Module 2, extended to wildcards.

---

### Drill 3.3-A

Rewrite your mini project's `GET /book?id=1` to use `GET /book/{id}` path parameters instead. Extract the ID with `r.PathValue("id")`. Test:

```bash
curl http://localhost:8080/book/1   # should return book with ID 1
curl http://localhost:8080/book/abc # should return 400 — invalid id
curl http://localhost:8080/book/999 # should return 404 — not found
```

---

## 3.4 Conflicting Patterns — What the Mux Rejects

Go 1.22's mux is stricter about conflicting patterns. It panics at startup if you register patterns that could match the same request ambiguously.

```go
// These conflict — both could match GET /book/42
mux.HandleFunc("GET /book/{id}", getBook)
mux.HandleFunc("GET /book/{name}", getByName) // PANIC at startup
```

Two wildcards in the same position conflict. The mux detects this when you call `HandleFunc` and panics immediately — not at request time. This is intentional: better to crash at startup than silently route requests wrong.

```go
// These do NOT conflict — different specificity
mux.HandleFunc("GET /book/new", newBookForm)  // specific segment
mux.HandleFunc("GET /book/{id}", getBook)     // wildcard — loses to specific
```

This is safe — specific always beats wildcard.

---

### Drill 3.4-A

Try registering two conflicting patterns and observe the panic message. Read the message carefully — it tells you exactly which patterns conflict and why.

---

## 3.5 Cleaning Up Routes — The `routes()` Method

As your mux grows, registering routes in `main` gets messy. The idiomatic Go pattern (from Alex Edwards' book) is to move route registration to a `routes()` method on `application`:

```go
// routes.go
func (app *application) routes() http.Handler {
    mux := http.NewServeMux()

    mux.HandleFunc("GET /", app.home)
    mux.HandleFunc("GET /books", app.listBooks)
    mux.HandleFunc("POST /books", app.createBook)
    mux.HandleFunc("GET /books/{id}", app.getBook)
    mux.HandleFunc("PUT /books/{id}", app.updateBook)
    mux.HandleFunc("DELETE /books/{id}", app.deleteBook)

    return mux
}

// main.go
func main() {
    app := &application{...}
    
    server := &http.Server{
        Addr:    ":8080",
        Handler: app.routes(),
    }
    log.Fatal(server.ListenAndServe())
}
```

Notice `routes()` returns `http.Handler` not `*http.ServeMux`. This is intentional — in Module 7, you'll wrap the mux in middleware and return that instead. Returning the interface means you can change the implementation without changing the caller.

---

### Drill 3.5-A

Move all your route registrations from `main` into a `routes()` method on `application`. `main` should only create the `application` struct and start the server. All routing logic should be in `routes()`.

---

## 3.6 When to Use a Third-Party Router

With Go 1.22, the standard `ServeMux` handles:
- ✅ Method routing (`GET /books`)
- ✅ Path parameters (`/books/{id}`)
- ✅ Wildcard suffixes (`/files/{path...}`)
- ✅ Automatic 405 responses
- ✅ Automatic HEAD handling

So when do you still need a third-party router?

**1. Regex constraints on path params**

```go
// you want /book/{id} to only match numeric IDs
// ServeMux can't do this — {id} matches anything
// chi or gorilla/mux can constrain: /book/{id:[0-9]+}
```

With `ServeMux` you validate the value yourself after extracting it. With chi you constrain it in the pattern. Both work — it's a preference.

**2. Route grouping with shared middleware**

```go
// chi — apply auth middleware to all /admin routes
r.Route("/admin", func(r chi.Router) {
    r.Use(requireAuth)
    r.Get("/users", listUsers)
    r.Get("/settings", showSettings)
})
```

With `ServeMux` you apply middleware to the whole mux or wrap individual handlers. Chi's grouping is more ergonomic for complex apps.

**3. Existing codebase using a router**

If you join a team using `gorilla/mux` or `chi`, use what's there. Don't introduce a second router.

---

### The honest recommendation

For new projects on Go 1.22+: start with `net/http` and the standard `ServeMux`. It handles 90% of real-world routing needs. If you find yourself fighting it — needing regex constraints, complex middleware grouping — add `chi`. It's the lightest option and feels almost identical to `net/http`.

Don't add a router because it's familiar from another language. Add it when you have a specific need it solves.

---

## 3.7 Putting It All Together — Refactored Book API

Here is your mini project from Module 2 rewritten with Go 1.22 routing:

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "os"
    "strconv"
    "sync"
)

type Book struct {
    ID    int    `json:"id"`
    Title string `json:"title"`
}

type application struct {
    logger *log.Logger
    mu     sync.RWMutex
    books  []Book
    nextID int
}

func (a *application) routes() http.Handler {
    mux := http.NewServeMux()

    mux.HandleFunc("GET /", a.home)
    mux.HandleFunc("GET /books", a.listBooks)
    mux.HandleFunc("POST /books", a.createBook)
    mux.HandleFunc("GET /books/{id}", a.getBook)
    mux.HandleFunc("DELETE /books/{id}", a.deleteBook)

    return mux
}

func (a *application) home(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"status":"ok"}`))
}

func (a *application) listBooks(w http.ResponseWriter, r *http.Request) {
    a.mu.RLock()
    defer a.mu.RUnlock()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(a.books)
}

func (a *application) getBook(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(r.PathValue("id"))
    if err != nil || id < 0 {
        http.Error(w, "invalid id", http.StatusBadRequest)
        return
    }

    a.mu.RLock()
    var found *Book
    for i := range a.books {
        if a.books[i].ID == id {
            found = &a.books[i]
            break
        }
    }
    a.mu.RUnlock()

    if found == nil {
        http.Error(w, "book not found", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(found)
}

func (a *application) createBook(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Title string `json:"title"`
    }

    err := json.NewDecoder(r.Body).Decode(&input)
    if err != nil || input.Title == "" {
        http.Error(w, "title is required", http.StatusBadRequest)
        return
    }

    a.mu.Lock()
    book := Book{ID: a.nextID, Title: input.Title}
    a.books = append(a.books, book)
    a.nextID++
    a.mu.Unlock()

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(book)
}

func (a *application) deleteBook(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(r.PathValue("id"))
    if err != nil || id < 0 {
        http.Error(w, "invalid id", http.StatusBadRequest)
        return
    }

    a.mu.Lock()
    found := false
    for i := range a.books {
        if a.books[i].ID == id {
            a.books = append(a.books[:i], a.books[i+1:]...)
            found = true
            break
        }
    }
    a.mu.Unlock()

    if !found {
        http.Error(w, "book not found", http.StatusNotFound)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

func main() {
    app := &application{
        logger: log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime),
        books:  []Book{},
        nextID: 1,
    }

    log.Fatal(http.ListenAndServe(":8080", app.routes()))
}
```

Compare this to Module 2's version:
- No `switch r.Method` anywhere
- No manual 405 responses
- Clean `/books/{id}` URLs instead of `/book?id=1`
- `routes()` method keeps `main` clean
- JSON struct tags (`json:"id"`) — we'll cover these fully in Module 4

---

## Mini Project — Refactor and Extend

Take your Module 2 mini project and apply everything from this module:

**1. Method routing** — replace all `switch r.Method` with separate registered handlers per method

**2. Path parameters** — change `GET /book?id=1` to `GET /books/{id}` and `DELETE /books/{id}`

**3. `routes()` method** — move all route registration out of `main`

**4. Add one new route:** `PUT /books/{id}` — update a book's title. Read the new title from the JSON body, find the book by ID, update it, return the updated book as JSON. Return 404 if not found.

**Test commands:**
```bash
curl http://localhost:8080/
curl http://localhost:8080/books
curl -X POST http://localhost:8080/books \
  -H "Content-Type: application/json" \
  -d '{"title": "The Go Programming Language"}'
curl http://localhost:8080/books/1
curl -X PUT http://localhost:8080/books/1 \
  -H "Content-Type: application/json" \
  -d '{"title": "Updated Title"}'
curl -X DELETE http://localhost:8080/books/1
curl http://localhost:8080/books
curl -X POST http://localhost:8080/books/1  # expect 405
curl http://localhost:8080/books/999        # expect 404
```

---

## Book Reference

*Let's Go* was written before Go 1.22, so it doesn't cover the new `ServeMux` features. The routing patterns Alex uses are the older style. When you read his routing chapters, mentally translate:

- His `?id=1` query params → your `/{id}` path params  
- His manual method checks → your method-prefixed patterns

Everything else — the `routes()` method pattern, the `application` struct, the `http.Handler` return type — is identical to what he does.

Read **Chapter 2.3** (routing) to see the older patterns, then appreciate how much Go 1.22 cleaned things up.