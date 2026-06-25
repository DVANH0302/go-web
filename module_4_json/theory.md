# Module 4 — Working with JSON

> **Goal:** Understand how Go encodes and decodes JSON — what's happening under the hood, the rules struct tags follow, common gotchas, and how to write a clean JSON API layer. After this module you'll fill in the handler logic from Module 3 properly.

---

## 4.1 What JSON Actually Is in Go

JSON is just text. A Go struct like this:

```go
type Book struct {
    ID    int
    Title string
}
```

Has no inherent JSON representation. You have to **encode** it — convert the struct to a JSON string. And to go the other way, you **decode** a JSON string back into a struct.

Go's `encoding/json` package handles both directions. It uses **reflection** under the hood — it inspects your struct's fields at runtime and maps them to JSON keys.

---

## 4.2 Encoding — Struct to JSON

There are two ways to encode a struct to JSON.

### `json.Marshal` — encodes to `[]byte`

```go
book := Book{ID: 1, Title: "The Go Programming Language"}

data, err := json.Marshal(book)
if err != nil {
    // handle error
}

fmt.Println(string(data))
// {"ID":1,"Title":"The Go Programming Language"}
```

`json.Marshal` encodes the struct and returns the result as `[]byte`. To see it as text you convert with `string(data)`.

**Under the hood:** `json.Marshal` reflects over the struct, finds all exported fields, and maps each field name to a JSON key. Notice the keys are `"ID"` and `"Title"` — exactly the field names, capitalised. We'll change this with struct tags.

### `json.NewEncoder` — encodes directly to a writer

```go
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(book)
// writes {"ID":1,"Title":"The Go Programming Language"}\n to w
```

`json.NewEncoder` takes any `io.Writer` and encodes directly to it. For HTTP responses, you pass `w` (the `ResponseWriter`) and it writes straight to the TCP connection — no intermediate `[]byte` buffer.

### Which one to use?

```go
// Use Marshal when you need the JSON as a value (to store, compare, log)
data, _ := json.Marshal(book)
log.Printf("encoded: %s", data)

// Use NewEncoder when writing to a response, file, or stream
json.NewEncoder(w).Encode(book)       // HTTP response
json.NewEncoder(os.Stdout).Encode(book) // stdout
json.NewEncoder(file).Encode(book)    // file
```

In HTTP handlers, **always use `json.NewEncoder(w)`** — it's more efficient because it writes directly to the connection without allocating an intermediate buffer.

---

### Drill 4.2-A

Create a `User` struct with fields `ID int`, `Name string`, `Email string`. Encode it both ways — with `json.Marshal` and `json.NewEncoder(os.Stdout)`. Observe the output of both. Notice `json.NewEncoder` adds a trailing newline — `json.Marshal` does not. Why does that matter for HTTP responses? (Hint: it usually doesn't, but know the difference.)

---

## 4.3 Struct Tags — Controlling JSON Output

By default, `encoding/json` uses the exact field name as the JSON key. That means your JSON looks like Go — `PascalCase`. But JSON convention is `camelCase` or `snake_case`. Struct tags fix this.

### The `json` struct tag

```go
type Book struct {
    ID    int    `json:"id"`
    Title string `json:"title"`
}

book := Book{ID: 1, Title: "Go"}
data, _ := json.Marshal(book)
// {"id":1,"title":"Go"} — lowercase keys, proper JSON convention
```

The backtick syntax is a **struct tag** — metadata attached to a field. The `json:"id"` tag tells `encoding/json`: "use `id` as the key for this field, not `ID`".

### `omitempty` — skip zero-value fields

```go
type Response struct {
    Data  []Book `json:"data"`
    Error string `json:"error,omitempty"` // omitted if empty string
    Total int    `json:"total,omitempty"` // omitted if 0
}

r := Response{Data: []Book{{ID: 1, Title: "Go"}}}
data, _ := json.Marshal(r)
// {"data":[{"id":1,"title":"Go"}]}
// Error and Total are omitted because they're zero values
```

`omitempty` means: if this field is the zero value for its type (`""`, `0`, `false`, `nil`), don't include it in the JSON output.

Without `omitempty`:
```go
// {"data":[...],"error":"","total":0} — noisy, exposes empty fields
```

With `omitempty`:
```go
// {"data":[...]} — clean, only non-zero fields
```

### `-` — always exclude a field

```go
type User struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    Password string `json:"-"` // NEVER included in JSON — even if non-empty
    Token    string `json:"-"` // security-sensitive, always hidden
}
```

`json:"-"` means: never encode this field. It's completely invisible to JSON. Use this for passwords, tokens, internal fields — anything you must never expose in an API response.

### Combining options

```go
type Book struct {
    ID          int     `json:"id"`
    Title       string  `json:"title"`
    Description string  `json:"description,omitempty"` // optional field
    InternalRef string  `json:"-"`                     // never exposed
    CreatedAt   string  `json:"created_at"`            // snake_case
}
```

---

### Drill 4.3-A

Take your `Book` struct and add proper struct tags:
- `ID` → `"id"`
- `Title` → `"title"`

Add a new field `Description string` with `omitempty`. Add an `InternalCode string` that is never exposed.

Create two books — one with a description, one without. Encode both and observe that the empty description is omitted but the one with a value is included. Confirm `InternalCode` never appears.

---

## 4.4 Decoding — JSON to Struct

Decoding is the reverse: taking a JSON string and filling a Go struct with its values.

### `json.NewDecoder` — decodes from a reader

```go
var book Book
err := json.NewDecoder(r.Body).Decode(&book)
if err != nil {
    http.Error(w, "invalid JSON", http.StatusBadRequest)
    return
}
// book.ID and book.Title are now populated
```

`json.NewDecoder` takes any `io.Reader`. For HTTP handlers, you pass `r.Body` — the request body stream. `Decode` reads from the stream and fills the struct you pass as a pointer.

**Why a pointer?** `Decode` needs to modify the struct — it fills in the fields. To modify something in Go, you need a pointer to it.

### `json.Unmarshal` — decodes from `[]byte`

```go
data := []byte(`{"id":1,"title":"Go"}`)
var book Book
err := json.Unmarshal(data, &book)
```

Use `Unmarshal` when you already have the JSON as `[]byte`. Use `NewDecoder` when reading from a stream (HTTP body, file, network connection).

In HTTP handlers, **always use `json.NewDecoder(r.Body)`** — the body is a stream, not a `[]byte`.

### What happens with unknown fields

By default, `json.Decoder` ignores JSON fields that don't match any struct field:

```go
// JSON has "unknown_field" — struct doesn't have it
data := `{"id":1,"title":"Go","unknown_field":"ignored"}`
var book Book
json.Unmarshal([]byte(data), &book)
// book = {ID:1, Title:"Go"} — unknown_field silently ignored
```

This is usually fine. But sometimes you want to reject unknown fields — for strict APIs:

```go
decoder := json.NewDecoder(r.Body)
decoder.DisallowUnknownFields() // returns error if JSON has unknown fields
err := decoder.Decode(&book)
```

### What happens with missing fields

If the JSON is missing a field that the struct has, that field gets its zero value:

```go
data := `{"id":1}` // no "title"
var book Book
json.Unmarshal([]byte(data), &book)
// book = {ID:1, Title:""} — Title gets zero value
```

You need to validate manually if a field is required:

```go
if book.Title == "" {
    http.Error(w, "title is required", http.StatusBadRequest)
    return
}
```

---

### Drill 4.4-A

Write a handler for `POST /books` that:
1. Decodes the JSON body into a `Book` struct
2. Returns 400 if the JSON is malformed
3. Returns 400 if `title` is empty
4. Returns 400 if the body is empty
5. On success, prints the decoded book and returns 201

Test with:
```bash
curl -X POST http://localhost:8080/books \
  -H "Content-Type: application/json" \
  -d '{"title": "The Go Programming Language"}'

curl -X POST http://localhost:8080/books \
  -H "Content-Type: application/json" \
  -d 'not json'  # expect 400

curl -X POST http://localhost:8080/books \
  -H "Content-Type: application/json" \
  -d '{}'  # expect 400 — title is empty
```

---

## 4.5 The Three Decoding Errors You Must Handle

When decoding JSON from an HTTP request, three different errors can occur. Each means something different and should return a different response.

```go
var input struct {
    Title string `json:"title"`
}

err := json.NewDecoder(r.Body).Decode(&input)
if err != nil {
    // Which error is it?
}
```

### Error 1 — Syntax error (malformed JSON)

```go
// client sent: {title: "Go"} — missing quotes on key, invalid JSON
var syntaxErr *json.SyntaxError
if errors.As(err, &syntaxErr) {
    http.Error(w, "malformed JSON", http.StatusBadRequest)
    return
}
```

### Error 2 — Wrong type (type mismatch)

```go
// client sent: {"id": "not-a-number"} — id should be int, got string
var unmarshalErr *json.UnmarshalTypeError
if errors.As(err, &unmarshalErr) {
    msg := fmt.Sprintf("wrong type for field %q", unmarshalErr.Field)
    http.Error(w, msg, http.StatusBadRequest)
    return
}
```

### Error 3 — Empty body

```go
// client sent no body at all
if errors.Is(err, io.EOF) {
    http.Error(w, "body must not be empty", http.StatusBadRequest)
    return
}
```

### Putting it together — a reusable decode helper

Instead of writing these checks in every handler, put them in a helper:

```go
func (app *application) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
    // limit body size
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB

    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()

    err := dec.Decode(dst)
    if err != nil {
        var syntaxErr *json.SyntaxError
        var unmarshalErr *json.UnmarshalTypeError

        switch {
        case errors.As(err, &syntaxErr):
            return fmt.Errorf("malformed JSON at position %d", syntaxErr.Offset)
        case errors.As(err, &unmarshalErr):
            return fmt.Errorf("wrong type for field %q", unmarshalErr.Field)
        case errors.Is(err, io.EOF):
            return fmt.Errorf("body must not be empty")
        default:
            return err
        }
    }
    return nil
}
```

Now in your handlers:

```go
func (a *application) createBook(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Title string `json:"title"`
    }

    err := a.decodeJSON(w, r, &input)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    if input.Title == "" {
        http.Error(w, "title is required", http.StatusBadRequest)
        return
    }

    // ... rest of handler
}
```

---

### Drill 4.5-A

Add the `decodeJSON` helper to your application and use it in your `POST /books` handler. Test all three error cases — malformed JSON, wrong type, empty body. Confirm each returns a 400 with a descriptive message.

---

## 4.6 Writing JSON Responses — A Clean Helper

Just like decoding, writing JSON responses has a pattern you'll repeat in every handler. Extract it into a helper:

```go
func (app *application) writeJSON(w http.ResponseWriter, status int, data any) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    return json.NewEncoder(w).Encode(data)
}
```

Usage in handlers:

```go
// success response
app.writeJSON(w, http.StatusOK, book)

// created response
app.writeJSON(w, http.StatusCreated, book)

// error response — use a consistent error envelope
app.writeJSON(w, http.StatusNotFound, map[string]string{
    "error": "book not found",
})
```

### Envelope pattern — wrap your responses

A common production pattern is to wrap all responses in an envelope:

```go
// instead of returning a raw book:
{"id":1,"title":"Go"}

// wrap it:
{"book":{"id":1,"title":"Go"}}

// or for lists:
{"books":[...],"count":5}
```

This lets you add metadata later (pagination, totals) without breaking clients:

```go
type envelope map[string]any

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    return json.NewEncoder(w).Encode(data)
}

// usage
app.writeJSON(w, http.StatusOK, envelope{"book": book})
app.writeJSON(w, http.StatusOK, envelope{"books": books, "count": len(books)})
app.writeJSON(w, http.StatusNotFound, envelope{"error": "book not found"})
```

---

### Drill 4.6-A

Add the `writeJSON` helper with the envelope pattern. Update all your handlers to use it. Your responses should look like:

```json
// GET /books
{"books":[{"id":1,"title":"Go"},{"id":2,"title":"Clean Code"}],"count":2}

// GET /books/1
{"book":{"id":1,"title":"Go"}}

// POST /books
{"book":{"id":3,"title":"New Book"}}

// Error responses
{"error":"book not found"}
```

---

## 4.7 Nullable Fields — Pointers in JSON

Sometimes a field is optional — it might be present in the JSON or absent. You need to distinguish between "not provided" and "provided as empty".

```go
type UpdateBook struct {
    Title *string `json:"title"` // pointer — nil means "not provided"
}
```

With a pointer:
- `null` in JSON → `nil` pointer in Go → field not provided
- `"Go"` in JSON → `*string` pointing to `"Go"` → field provided

```go
var input UpdateBook
json.Unmarshal([]byte(`{}`), &input)
// input.Title == nil — not provided

json.Unmarshal([]byte(`{"title":"Go"}`), &input)
// input.Title == &"Go" — provided

// dereference to get the value
if input.Title != nil {
    book.Title = *input.Title
}
```

This matters for `PUT`/`PATCH` endpoints where you only want to update fields that were actually sent:

```go
func (a *application) updateBook(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Title *string `json:"title"` // nil if not sent
    }

    a.decodeJSON(w, r, &input)

    // only update fields that were provided
    if input.Title != nil {
        book.Title = *input.Title
    }
    // if input.Title is nil, Title is unchanged
}
```

---

### Drill 4.7-A

Implement `PUT /books/{id}` using a pointer field for `Title`. Test:

```bash
# update title
curl -X PUT http://localhost:8080/books/1 \
  -H "Content-Type: application/json" \
  -d '{"title":"Updated Title"}'

# empty body — should not update anything, return current book
curl -X PUT http://localhost:8080/books/1 \
  -H "Content-Type: application/json" \
  -d '{}'
```

---

## 4.8 Putting It All Together — Complete Handler Logic

Here is the complete book API from Module 3, now with proper JSON handling:

```go
package main

import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
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

type envelope map[string]any

type application struct {
    logger *log.Logger
    mu     sync.RWMutex
    books  []Book
    nextID int
}

// JSON helpers
func (a *application) writeJSON(w http.ResponseWriter, status int, data envelope) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    return json.NewEncoder(w).Encode(data)
}

func (a *application) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()

    err := dec.Decode(dst)
    if err != nil {
        var syntaxErr *json.SyntaxError
        var unmarshalErr *json.UnmarshalTypeError
        switch {
        case errors.As(err, &syntaxErr):
            return fmt.Errorf("malformed JSON at position %d", syntaxErr.Offset)
        case errors.As(err, &unmarshalErr):
            return fmt.Errorf("wrong type for field %q", unmarshalErr.Field)
        case errors.Is(err, io.EOF):
            return fmt.Errorf("body must not be empty")
        default:
            return err
        }
    }
    return nil
}

// Routes
func (a *application) routes() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /", a.home)
    mux.HandleFunc("GET /books", a.listBooks)
    mux.HandleFunc("POST /books", a.createBook)
    mux.HandleFunc("GET /books/{id}", a.getBook)
    mux.HandleFunc("PUT /books/{id}", a.updateBook)
    mux.HandleFunc("DELETE /books/{id}", a.deleteBook)
    return mux
}

// Handlers
func (a *application) home(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    a.writeJSON(w, http.StatusOK, envelope{"status": "ok"})
}

func (a *application) listBooks(w http.ResponseWriter, r *http.Request) {
    a.mu.RLock()
    books := make([]Book, len(a.books))
    copy(books, a.books)
    a.mu.RUnlock()

    a.writeJSON(w, http.StatusOK, envelope{
        "books": books,
        "count": len(books),
    })
}

func (a *application) getBook(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(r.PathValue("id"))
    if err != nil || id < 0 {
        a.writeJSON(w, http.StatusBadRequest, envelope{"error": "invalid id"})
        return
    }

    a.mu.RLock()
    var found *Book
    for i := range a.books {
        if a.books[i].ID == id {
            b := a.books[i] // copy — don't return pointer into locked slice
            found = &b
            break
        }
    }
    a.mu.RUnlock()

    if found == nil {
        a.writeJSON(w, http.StatusNotFound, envelope{"error": "book not found"})
        return
    }

    a.writeJSON(w, http.StatusOK, envelope{"book": found})
}

func (a *application) createBook(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Title string `json:"title"`
    }

    err := a.decodeJSON(w, r, &input)
    if err != nil {
        a.writeJSON(w, http.StatusBadRequest, envelope{"error": err.Error()})
        return
    }

    if input.Title == "" {
        a.writeJSON(w, http.StatusBadRequest, envelope{"error": "title is required"})
        return
    }

    a.mu.Lock()
    book := Book{ID: a.nextID, Title: input.Title}
    a.books = append(a.books, book)
    a.nextID++
    a.mu.Unlock()

    a.writeJSON(w, http.StatusCreated, envelope{"book": book})
}

func (a *application) updateBook(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(r.PathValue("id"))
    if err != nil || id < 0 {
        a.writeJSON(w, http.StatusBadRequest, envelope{"error": "invalid id"})
        return
    }

    var input struct {
        Title *string `json:"title"` // pointer — nil if not provided
    }

    err = a.decodeJSON(w, r, &input)
    if err != nil {
        a.writeJSON(w, http.StatusBadRequest, envelope{"error": err.Error()})
        return
    }

    a.mu.Lock()
    var found *Book
    for i := range a.books {
        if a.books[i].ID == id {
            found = &a.books[i]
            break
        }
    }

    if found == nil {
        a.mu.Unlock()
        a.writeJSON(w, http.StatusNotFound, envelope{"error": "book not found"})
        return
    }

    if input.Title != nil {
        found.Title = *input.Title
    }
    updated := *found // copy before unlock
    a.mu.Unlock()

    a.writeJSON(w, http.StatusOK, envelope{"book": updated})
}

func (a *application) deleteBook(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(r.PathValue("id"))
    if err != nil || id < 0 {
        a.writeJSON(w, http.StatusBadRequest, envelope{"error": "invalid id"})
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
        a.writeJSON(w, http.StatusNotFound, envelope{"error": "book not found"})
        return
    }

    w.WriteHeader(http.StatusNoContent) // 204 — no body
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

---

## Mini Project — Fill In Your Module 3 Handlers

Now go back to your Module 3 skeleton and fill in all the handler logic using everything from this module:

**Requirements:**
- `Book` struct with proper `json` struct tags
- `writeJSON` helper with envelope pattern
- `decodeJSON` helper with all three error cases handled
- `POST /books` — decode body, validate title, return 201 with new book
- `GET /books` — return all books with count
- `GET /books/{id}` — return single book or 404
- `PUT /books/{id}` — update with pointer field, return updated book
- `DELETE /books/{id}` — delete, return 204

**Test commands:**
```bash
curl http://localhost:8080/books
curl -X POST http://localhost:8080/books \
  -H "Content-Type: application/json" \
  -d '{"title":"The Go Programming Language"}'
curl http://localhost:8080/books/1
curl -X PUT http://localhost:8080/books/1 \
  -H "Content-Type: application/json" \
  -d '{"title":"Updated"}'
curl -X DELETE http://localhost:8080/books/1
curl -X POST http://localhost:8080/books \
  -d 'bad json'         # expect 400 — malformed JSON
curl -X POST http://localhost:8080/books \
  -d ''                 # expect 400 — empty body
curl -X POST http://localhost:8080/books \
  -d '{"title":123}'    # expect 400 — wrong type
```

---

## Book Reference

*Let's Go* focuses on HTML templates rather than JSON APIs, so this module goes beyond what the book covers directly. However:

- **Chapter 2.4** — customising headers (our section 4.2 — setting Content-Type)
- **Chapter 4.6** — executing SQL and reading results (same `Scan` pattern as JSON Decode)

For JSON specifically, the Go standard library docs for `encoding/json` are excellent. Read them after finishing this module.