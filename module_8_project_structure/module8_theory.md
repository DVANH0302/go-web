# Module 8 — Project Structure & Configuration (Theory)

## 1. The problem

Your book API works, but everything lives in one giant `main.go`: routes, handlers, SQL, validation, hardcoded port and DSN. This hurts because:

- You can't find anything — HTTP code, business rules, and SQL are interleaved.
- You can't test business logic without spinning up HTTP and a database.
- Changing config (port, DB password) means editing code and recompiling.
- Two people can't work on the same file without constant merge conflicts.

## 2. The standard Go layout

```
bookapi/
├── cmd/
│   └── api/
│       └── main.go        ← entry point ONLY: wiring, startup
├── internal/
│   ├── books/             ← domain logic for books
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── repository.go
│   └── database/
│       └── database.go    ← opening the connection pool
├── .env
├── go.mod
```

- **`cmd/`** — one folder per executable. `cmd/api` builds the API binary; later you could add `cmd/cli`. Files here should be *thin* — just wiring.
- **`internal/`** — all your actual application code. The Go compiler **enforces** that packages under `internal/` cannot be imported by code outside your module. This is not a convention — the build fails. It prevents outsiders from depending on your internals, so you're free to refactor them.
- **`pkg/`** — optional folder for code you *intend* others to import. Most apps don't need it. Don't create it "just in case".

## 3. The three layers (separation of concerns)

| Layer | File | Knows about | Never touches |
|---|---|---|---|
| **Handler** | `handler.go` | HTTP: parse request, write JSON, status codes | SQL |
| **Service** | `service.go` | Business rules: validation, ID generation, "only owner can delete" | `http.Request`, SQL |
| **Repository** | `repository.go` | SQL: queries, `Scan`, `ErrNoRows` | HTTP |

The flow is always: **handler → service → repository → database**.

Why the pain of splitting? Because each layer can now change or be tested independently. Swap PostgreSQL for SQLite → only `repository.go` changes. Add a validation rule → only `service.go` changes. In Module 10 you'll mock the repository to test the service with zero database.

## 4. Configuration

**Hardcoding is bad** because dev, test, and production need *different* values (port, DSN, secrets), and secrets must never live in source code / git.

**The Config struct** — centralise every setting in one place:

```go
type Config struct {
    Port string
    DSN  string
}
```

One struct, passed down from `main`. No scattered magic strings.

**Environment variables** — `os.Getenv("PORT")` reads from the process environment. Gotcha: it returns `""` if unset, so you always write a helper that falls back to a default.

**`.env` files + godotenv** — exporting env vars by hand every session is tedious. A `.env` file holds `KEY=value` lines; `godotenv.Load()` reads it into the environment at startup. Rule: **`.env` goes in `.gitignore`** — it holds secrets.

**Flags vs env vars** (Alex Edwards' take):
- Flags: typed (`flag.Int`), have defaults, free `-help` output. Great for things a human sets at launch (`-addr=:9999`).
- Env vars: the standard for containers/cloud deployment, and for secrets.
- Best of both: `go run ./cmd/api -addr=$API_ADDR` — env var passed as a flag.

For our API we'll use env vars + defaults, since that's what production deployment tooling expects.

## 5. What `main.go` should look like at the end

Read config → open DB → build repository → build service → build handler → register routes → start server. **Nothing else.** If `main.go` contains an `if` statement about books, something is in the wrong layer.
