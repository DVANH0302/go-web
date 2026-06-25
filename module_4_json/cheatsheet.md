# Module 4 — JSON Cheatsheet

## Encoding (Struct → JSON)

| Method | Output | Use when |
|---|---|---|
| `json.Marshal(v)` | `[]byte, error` | You need the JSON as a value (logging, storing) |
| `json.NewEncoder(w).Encode(v)` | writes to `io.Writer` | Writing to HTTP response, file, stdout |

```go
// Marshal
data, err := json.Marshal(book)
fmt.Println(string(data))          // need string() to print

// NewEncoder — preferred in HTTP handlers
json.NewEncoder(w).Encode(book)    // writes directly to connection, no buffer
```

> `NewEncoder` adds a trailing `\n`. `Marshal` does not. Doesn't matter for HTTP.

---

## Decoding (JSON → Struct)

| Method | Input | Use when |
|---|---|---|
| `json.NewDecoder(r).Decode(&v)` | `io.Reader` | HTTP request body (always) |
| `json.Unmarshal(data, &v)` | `[]byte` | You already have the JSON as bytes |

```go
var book Book
err := json.NewDecoder(r.Body).Decode(&book)
```

> Always pass a **pointer** to `Decode` / `Unmarshal` — it needs to write into your struct.

---

## Struct Tags

```go
type Book struct {
    ID          int    `json:"id"`                    // rename key
    Title       string `json:"title"`
    Description string `json:"description,omitempty"` // omit if zero value
    InternalRef string `json:"-"`                     // NEVER in JSON output
    CreatedAt   string `json:"created_at"`            // snake_case
}
```

| Tag | Effect |
|---|---|
| `json:"name"` | Use `name` as the JSON key |
| `json:"name,omitempty"` | Omit field if it's the zero value (`""`, `0`, `false`, `nil`) |
| `json:"-"` | Always exclude — passwords, tokens, internal fields |

---

## Unknown & Missing Fields

```go
// Unknown fields in JSON → silently ignored by default
// To reject them:
dec := json.NewDecoder(r.Body)
dec.DisallowUnknownFields()

// Missing fields in JSON → struct field gets zero value
// Validate manually:
if book.Title == "" {
    http.Error(w, "title is required", http.StatusBadRequest)
}
```

---

## The Three Decoding Errors

```go
err := json.NewDecoder(r.Body).Decode(&input)
if err != nil {
    var syntaxErr   *json.SyntaxError
    var unmarshalErr *json.UnmarshalTypeError

    switch {
    case errors.As(err, &syntaxErr):
        // Bad JSON:  {title: "Go"}  ← missing quotes
        return fmt.Errorf("malformed JSON at position %d", syntaxErr.Offset)

    case errors.As(err, &unmarshalErr):
        // Wrong type: {"id": "not-a-number"}  ← id should be int
        return fmt.Errorf("wrong type for field %q", unmarshalErr.Field)

    case errors.Is(err, io.EOF):
        // Empty body — no bytes at all
        return fmt.Errorf("body must not be empty")

    default:
        return err
    }
}
```

---

## `decodeJSON` Helper (copy this)

```go
func (a *application) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit

    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()

    err := dec.Decode(dst)
    if err != nil {
        var syntaxErr    *json.SyntaxError
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

---

## `writeJSON` Helper + Envelope Pattern (copy this)

```go
type envelope map[string]any

func (a *application) writeJSON(w http.ResponseWriter, status int, data envelope) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    return json.NewEncoder(w).Encode(data)
}
```

```go
// Usage
a.writeJSON(w, http.StatusOK,      envelope{"book":  book})
a.writeJSON(w, http.StatusOK,      envelope{"books": books, "count": len(books)})
a.writeJSON(w, http.StatusCreated, envelope{"book":  newBook})
a.writeJSON(w, http.StatusNotFound, envelope{"error": "book not found"})
```

> **Why envelope?** Wrapping responses (`{"book": {...}}` instead of `{...}`) lets you add metadata (pagination, totals) later without breaking clients.

---

## Nullable Fields — Pointer Trick for PATCH/PUT

```go
type UpdateInput struct {
    Title *string `json:"title"` // nil = "not provided", &"Go" = "provided"
}

var input UpdateInput
json.NewDecoder(r.Body).Decode(&input)

if input.Title != nil {
    book.Title = *input.Title  // dereference to get the string
}
// if nil → field was absent → don't update it
```

| JSON sent | `input.Title` value |
|---|---|
| `{}` | `nil` — not provided |
| `{"title": "Go"}` | `*"Go"` — provided |
| `{"title": ""}` | `*""` — explicitly set to empty |

---

## Header — Always Set Before Writing Body

```go
// CORRECT — set header first, then write
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)
json.NewEncoder(w).Encode(data)

// WRONG — headers are frozen once you call WriteHeader or Write
json.NewEncoder(w).Encode(data)
w.Header().Set("Content-Type", "application/json") // too late, ignored
```

The `writeJSON` helper handles this correctly for you.

---

## Response Status Codes — Quick Reference

| Situation | Code |
|---|---|
| Success, returning data | `200 OK` |
| Created new resource | `201 Created` |
| Success, no body (DELETE) | `204 No Content` |
| Bad input / validation fail | `400 Bad Request` |
| Not found | `404 Not Found` |
| Server blew up | `500 Internal Server Error` |

---

## Complete Handler Pattern

```go
func (a *application) createBook(w http.ResponseWriter, r *http.Request) {
    // 1. Define input shape
    var input struct {
        Title string `json:"title"`
    }

    // 2. Decode — handles all three error cases
    if err := a.decodeJSON(w, r, &input); err != nil {
        a.writeJSON(w, http.StatusBadRequest, envelope{"error": err.Error()})
        return
    }

    // 3. Validate
    if input.Title == "" {
        a.writeJSON(w, http.StatusBadRequest, envelope{"error": "title is required"})
        return
    }

    // 4. Do the work
    book := Book{ID: a.nextID, Title: input.Title}
    a.books = append(a.books, book)
    a.nextID++

    // 5. Respond
    a.writeJSON(w, http.StatusCreated, envelope{"book": book})
}
```

---

## Imports Needed

```go
import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
)
```