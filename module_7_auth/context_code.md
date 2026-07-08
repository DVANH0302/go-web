# Go's `context.Context` (Code)

## 1. Passing a Value Through Middleware Into a Handler

```go
package main

import (
    "context"
    "net/http"
)

// Unexported key type prevents collisions with keys from other packages.
type contextKey string

const userIDKey contextKey = "userID"

func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        userID := 42 // pretend this came from validating a JWT

        ctx := context.WithValue(r.Context(), userIDKey, userID)
        r = r.WithContext(ctx) // new *Request, original untouched

        next.ServeHTTP(w, r)
    })
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := r.Context().Value(userIDKey).(int)
    if !ok {
        http.Error(w, "no user in context", http.StatusInternalServerError)
        return
    }
    w.Write([]byte("your user id is " + string(rune(userID))))
}
```

Note the type assertion `.(int)` with the `ok` form — never assume `Value()`
returns the right type blindly, since it returns `any`.

---

## 2. Watching Values Walk Up the Chain

```go
package main

import (
    "context"
    "fmt"
)

func main() {
    ctx := context.Background()
    ctx = context.WithValue(ctx, contextKey("A"), "first")
    ctx = context.WithValue(ctx, contextKey("B"), "second")

    fmt.Println(ctx.Value(contextKey("A"))) // "first"  — found by walking up
    fmt.Println(ctx.Value(contextKey("B"))) // "second" — found at this node
    fmt.Println(ctx.Value(contextKey("C"))) // nil      — not found anywhere
}
```

Each `WithValue` call wraps the previous context. Looking up `"A"` means:
check the outermost node (holds `"B"`) → no match → ask parent (holds `"A"`)
→ match found.

---

## 3. Cancellation Propagating to Multiple Children

```go
package main

import (
    "context"
    "fmt"
    "time"
)

func worker(ctx context.Context, name string) {
    select {
    case <-ctx.Done():
        fmt.Println(name, "stopped:", ctx.Err())
    case <-time.After(10 * time.Second):
        fmt.Println(name, "finished normally")
    }
}

func main() {
    parent, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    // Three independent children of the same parent.
    child1, cancel1 := context.WithCancel(parent)
    child2, cancel2 := context.WithCancel(parent)
    child3, cancel3 := context.WithCancel(parent)
    defer cancel1()
    defer cancel2()
    defer cancel3()

    go worker(child1, "worker-1")
    go worker(child2, "worker-2")
    go worker(child3, "worker-3")

    time.Sleep(3 * time.Second)
}
```

Expected output after ~2 seconds — all three stop together, because the
parent's timeout cancelled all three children automatically:
```
worker-1 stopped: context deadline exceeded
worker-2 stopped: context deadline exceeded
worker-3 stopped: context deadline exceeded
```

---

## 4. Manual Cancellation (`WithCancel`)

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())

    go func() {
        time.Sleep(1 * time.Second)
        cancel() // manually trigger cancellation
    }()

    <-ctx.Done()
    fmt.Println("cancelled:", ctx.Err()) // "context canceled"
}
```

`cancel()` closes the internal `done` channel yourself, rather than waiting
for a timer. Always `defer cancel()` even when using `WithTimeout` — it
releases resources associated with the context immediately once you're done,
instead of waiting for the timer to fire on its own.

---

## 5. Using Context With a Database Query

```go
func (app *application) getBookHandler(w http.ResponseWriter, r *http.Request) {
    // r.Context() carries any deadline set by the server or middleware.
    ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
    defer cancel()

    var book Book
    err := app.db.QueryRowContext(ctx, "SELECT id, title FROM books WHERE id = $1", id).
        Scan(&book.ID, &book.Title)
    if err != nil {
        if ctx.Err() != nil {
            app.serverError(w, fmt.Errorf("query timed out: %w", ctx.Err()))
            return
        }
        app.serverError(w, err)
        return
    }
    // ...
}
```

If the query takes longer than 3 seconds, `QueryRowContext` aborts and
returns an error tied to the context deadline — the database driver itself
checks the context internally.

---

## 6. Why Two Parents Isn't Possible — There's No Such Function

```go
// This does NOT exist in the standard library:
// context.WithValue(parentA, parentB, key, value)

// What you actually get if you try to "combine" two contexts:
child1 := context.WithValue(parentA, contextKey("k"), "v")
child2 := context.WithValue(parentB, contextKey("k"), "v")
// child1 and child2 are two separate, unrelated contexts.
// Neither has any relationship to the other's parent.
```

Every context-deriving function (`WithValue`, `WithCancel`, `WithTimeout`,
`WithDeadline`) takes exactly one parent argument and returns exactly one
child. There's no constructor shaped to accept two parents, so it's not that
you'd get a runtime error — the operation simply has no way to be expressed.
