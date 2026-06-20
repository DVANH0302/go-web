# Module 1 — Go Fundamentals (Web-Relevant Subset)

> **Who this is for:** You know Java and Spring Boot. You've seen basic Go syntax. This module moves fast on the obvious, and slow on the things that will genuinely trip you up when building web apps.

---

## 1.1 How a Go Program is Structured

In Java, your entry point is a class with a `main` method. In Go, it's simpler — no class needed.

```go
package main  // every executable must be in package "main"

import "fmt"  // import only what you use — the compiler rejects unused imports

func main() {
    fmt.Println("Hello, web")
}
```

**`package` is not the same as a Java package.** In Go:
- A **package** is a folder. All `.go` files in the same folder share the same package name.
- `package main` is special — it signals "this is a runnable binary, not a library".
- Everything else is just a library package, e.g. `package handlers`, `package db`.

**Visibility is controlled by capitalisation, not `public`/`private`.**

```go
func ServeHTTP() {}   // exported — visible outside the package (like public)
func buildQuery() {}  // unexported — package-private (like private/package-private)

type User struct {
    Name  string  // exported field
    email string  // unexported field
}
```

This matters for web apps constantly — your handler types need exported methods, your internal helpers don't.

---

## 1.2 Types, Variables, and Zero Values

### Declaration styles

```go
// Explicit type
var name string = "alice"

// Inferred type (most common inside functions)
name := "alice"

// Multiple assignment
x, y := 10, 20

// Package-level — must use var, := doesn't work here
var db *sql.DB
```

### Zero values — this is important and very un-Java

In Go, every type has a **zero value** — the value it gets when declared but not assigned. There is no `null` for value types.

| Type | Zero value |
|---|---|
| `int`, `float64` | `0` |
| `string` | `""` |
| `bool` | `false` |
| `pointer`, `slice`, `map`, `channel`, `func` | `nil` |
| `struct` | all fields zeroed |

Why does this matter in web apps?

```go
type Config struct {
    Port    int    // zero = 0
    Debug   bool   // zero = false
    BaseURL string // zero = ""
}

// This is valid and safe — no NullPointerException waiting to explode
var cfg Config
fmt.Println(cfg.Port) // prints 0
```

You'll use this pattern constantly when parsing config from environment variables — the zero value gives you safe defaults.

### Basic types you'll use in web apps

```go
string      // UTF-8, immutable
int         // platform-sized integer (use this by default)
int64       // explicit 64-bit (common for database IDs)
float64     // doubles
bool
byte        // alias for uint8 — used in []byte for HTTP bodies
[]byte      // raw bytes — reading request bodies, writing responses
```

---

## 1.3 Functions — Multiple Returns and Errors as Values

### Multiple return values

Go functions can return more than one value. This is the core of how errors work.

```go
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("cannot divide by zero")
    }
    return a / b, nil
}

result, err := divide(10, 2)
if err != nil {
    // handle it
}
```

Compare this to Java: you'd either throw an exception or return a wrapper type (`Optional`, `Result`). Go makes errors **explicit in the return signature** — you can't ignore them without actively choosing to (using `_`).

### Errors are values — not exceptions

```go
// error is just an interface with one method:
type error interface {
    Error() string
}
```

This means errors can be passed around, stored, wrapped, and compared like any other value. There is no `try/catch`. You handle errors where they happen.

```go
// Idiomatic Go error handling
data, err := io.ReadAll(r.Body)
if err != nil {
    http.Error(w, "failed to read body", http.StatusBadRequest)
    return
}
```

The `return` after handling an error is critical. Without it, execution continues — a very common Go beginner bug.

### Named return values (used in deferred cleanup)

```go
func openFile(path string) (f *os.File, err error) {
    f, err = os.Open(path)
    return // "naked return" — returns f and err as named
}
```

You'll see this in HTTP middleware and deferred cleanup. Don't overuse it — it hurts readability.

---

## 1.4 Structs and Methods

Go has no classes. It has **structs** (data) and **methods** (functions attached to types).

```go
type User struct {
    ID    int64
    Name  string
    Email string
}

// Method on User — the receiver (u User) is like "this" in Java
func (u User) DisplayName() string {
    return u.Name + " <" + u.Email + ">"
}

// Pointer receiver — use when the method needs to mutate the struct
func (u *User) Anonymise() {
    u.Name = "anon"
    u.Email = ""
}
```

**Value receiver vs pointer receiver** — rule of thumb for web apps:
- Use **pointer receiver** (`*User`) if the method modifies the struct, or if the struct is large (avoids copying).
- Use **value receiver** (`User`) for small, read-only methods.
- Be consistent on a type — don't mix both.

### Struct embedding — Go's composition over inheritance

```go
type Base struct {
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Post struct {
    Base           // embedded — Post now has CreatedAt and UpdatedAt directly
    Title   string
    Content string
}

p := Post{Title: "Hello"}
fmt.Println(p.CreatedAt) // accessed directly, not p.Base.CreatedAt
```

This is how Go does "inheritance" — not through subclassing but through embedding. You'll use this in handler types that share common behaviour.

---

## 1.5 Interfaces — The Most Important Concept in Go Web Dev

This is where Go diverges most sharply from Java. **Interfaces are implemented implicitly** — no `implements` keyword.

```go
// Define an interface
type Animal interface {
    Sound() string
}

// Dog implements Animal — automatically, just by having the method
type Dog struct{}
func (d Dog) Sound() string { return "woof" }

// Cat also implements Animal
type Cat struct{}
func (c Cat) Sound() string { return "meow" }

func makeNoise(a Animal) {
    fmt.Println(a.Sound())
}
```

`Dog` and `Cat` never mention `Animal`. This is called **structural typing** or **duck typing** — if it has the right methods, it satisfies the interface.

### Why this is everything in Go web apps

The entire `net/http` package is built on one interface:

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}
```

Any type that has a `ServeHTTP(ResponseWriter, *Request)` method **is** an HTTP handler. This is how middleware works, how testing works, and how the whole ecosystem plugs together.

```go
// Your custom handler type
type AppHandler struct {
    DB *sql.DB
}

// Implementing the Handler interface
func (h AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // handle the request, use h.DB
}

// Register it — no special annotation, no Spring @Controller, nothing
http.Handle("/users", AppHandler{DB: db})
```

### The empty interface and `any`

```go
// interface{} (or its alias "any" in Go 1.18+) accepts any type
func log(v any) {
    fmt.Println(v)
}

log("hello")   // fine
log(42)        // fine
log(User{})    // fine
```

You'll see `any` in JSON decoding and generic utilities. Avoid it in your own code when you can — it loses type safety.

---

## 1.6 Pointers — When and Why

Go has pointers, but they're less scary than C. No pointer arithmetic. Just `&` (address of) and `*` (dereference).

```go
name := "alice"
p := &name      // p is a *string — pointer to name
*p = "bob"      // dereference to change the value
fmt.Println(name) // prints "bob"
```

In web apps, you use pointers mainly for two reasons:

**1. Optional values (nullable fields)**

```go
type Profile struct {
    Bio     *string  // nil means "not set" — pointer makes it nullable
    Age     *int
}
```

This is common when mapping JSON or database rows where a field can be absent vs empty.

**2. Sharing state across handlers**

```go
type App struct {
    DB     *sql.DB      // shared database connection pool
    Logger *slog.Logger // shared logger
}

func (a *App) handleUsers(w http.ResponseWriter, r *http.Request) {
    // a.DB is the same pool across all requests
}
```

If you passed `App` by value (no `*`), each handler call would get a copy of the struct. With `*App`, all handlers share the same instance.

---

## 1.7 Goroutines and Channels — Concurrency Mental Model

Go handles HTTP concurrency differently from Java. In Spring Boot, each request gets a thread from a thread pool. In Go, each request gets a **goroutine** — a lightweight concurrent unit managed by the Go runtime, not the OS.

```go
// Starting a goroutine — just prefix a function call with "go"
go func() {
    sendWelcomeEmail(user)
}()
// execution continues immediately — doesn't wait
```

The Go runtime can run millions of goroutines on a handful of OS threads. This is why Go HTTP servers handle high concurrency efficiently.

### What this means for your handlers

Every HTTP request runs in its own goroutine. So your handler code is **always concurrent**. This means:

```go
// DANGER — this is a data race
var counter int

func handler(w http.ResponseWriter, r *http.Request) {
    counter++ // multiple goroutines hitting this simultaneously = race condition
}
```

Fix it with `sync.Mutex` or `sync/atomic`:

```go
var (
    counter int
    mu      sync.Mutex
)

func handler(w http.ResponseWriter, r *http.Request) {
    mu.Lock()
    counter++
    mu.Unlock()
}
```

### Channels — communicating between goroutines

```go
ch := make(chan string)

go func() {
    ch <- "result" // send into channel
}()

msg := <-ch // receive from channel — blocks until something arrives
fmt.Println(msg)
```

For web apps, you'll use channels less often than goroutines directly. But you'll see them in background workers, graceful shutdown, and timeout handling.

### Goroutine leak — the thing to avoid

```go
// BAD — goroutine runs forever if nobody reads from ch
ch := make(chan string)
go func() {
    result := doSlowThing()
    ch <- result // blocks forever if nobody receives
}()
// if you return from the handler without reading ch, goroutine is stuck
```

Always give goroutines a way to exit — use `context.Context` for cancellation (covered in Module 2).

---

## Drill Exercises

These are small and focused. Do them in the Go Playground (play.golang.org) or locally.

### Drill 1.1 — Visibility
Create a package-level struct `Config` with three fields: one exported string, one unexported int, one exported bool. Write an exported constructor function `NewConfig` that returns a `*Config` with some defaults set.

### Drill 1.2 — Zero values
Declare a `User` struct with `Name string`, `Age int`, `Active bool`. Create one without assigning any fields. Print each field and observe the zero values. Then write a method `IsComplete() bool` that returns true only if Name is non-empty and Age is greater than 0.

### Drill 1.3 — Errors as values
Write a function `parseAge(s string) (int, error)` that:
- Uses `strconv.Atoi` to convert a string to int
- Returns an error if the conversion fails
- Returns an error if the age is less than 0 or greater than 150
- Returns the age and nil otherwise

Call it with `"25"`, `"abc"`, `"-5"`, and `"200"` and handle each error appropriately.

### Drill 1.4 — Interfaces
Define an interface `Storer` with two methods: `Save(id string, data []byte) error` and `Load(id string) ([]byte, error)`. Write two structs that implement it: `MemoryStore` (stores in a `map[string][]byte`) and `NullStore` (does nothing, always returns nil). Write a function `processData(s Storer, id string, data []byte)` that saves then loads.

### Drill 1.5 — Goroutines
Write a program that launches 5 goroutines, each sleeping for a random duration between 100ms and 500ms, then sending their goroutine number on a channel. The main function should collect all 5 results and print them in the order they arrive.

---

## Mini Project — Simple CLI HTTP Tester

Build a small command-line tool that takes a URL as an argument, makes an HTTP GET request, and prints:
- The status code
- The `Content-Type` header
- The first 200 characters of the body

This forces you to use: structs, error handling, string formatting, and reading from an `io.Reader` — all patterns you'll use constantly in the web modules.

```
Usage: go run main.go https://example.com

Status: 200
Content-Type: text/html; charset=UTF-8
Body preview:
<!doctype html><html><head><title>Example Domain</title>...
```

**Hints:**
- `http.Get(url)` returns `(*http.Response, error)`
- `resp.Body` is an `io.ReadCloser` — always `defer resp.Body.Close()`
- `io.ReadAll(resp.Body)` reads the whole body as `[]byte`
- `os.Args[1]` gives you the first CLI argument

---

## Key Takeaways Before Module 2

| Go concept | Java equivalent | Key difference |
|---|---|---|
| `package` visibility | `public`/`private` | Capitalisation, not keywords |
| Zero values | Null / uninitialized | Always safe, never null for value types |
| Multiple returns | `throws` / wrapper types | Errors are return values, not exceptions |
| Struct + methods | Class | No inheritance — use embedding and interfaces |
| Implicit interfaces | `implements` | If you have the methods, you satisfy the interface |
| Goroutines | Threads | Cheaper, runtime-managed, always concurrent in handlers |

**Module 2** is where this all clicks — you'll use every single one of these in your first real HTTP server.