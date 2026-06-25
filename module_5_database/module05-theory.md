# Module 05 — Database (PostgreSQL): Theory

---

## 1. Why `database/sql` Is an Interface

Go's standard library does not ship with a PostgreSQL driver, a MySQL driver, or any database driver at all. What it ships is `database/sql` — a **pure interface layer** that defines what database operations *look like*, with zero knowledge of any specific database.

```
Your Go code
    ↓ calls
database/sql (interface)
    ↓ delegates to
pgx driver (implementation)
    ↓ speaks TCP to
PostgreSQL server
```

The driver registers itself when you import it:

```go
import _ "github.com/jackc/pgx/v5/stdlib"
```

The blank import `_` means "run this package's `init()` function but don't use any of its exported names." That `init()` calls `sql.Register("pgx", &pgxDriver{})` — from that moment, `database/sql` knows how to talk to PostgreSQL via that driver.

**Why this design matters:**

- You can swap PostgreSQL for SQLite in tests by changing one import and one connection string. All `db.Query(...)`, `rows.Scan(...)`, `db.Exec(...)` calls stay identical.
- It's the same philosophy as `io.Reader` — program to the interface, not the implementation.
- In large companies, this lets teams write shared database tooling (logging, tracing, retry logic) that works regardless of which database the team uses.

**The trade-off:** Some PostgreSQL-specific features (array types, JSONB, LISTEN/NOTIFY) are awkward through `database/sql`. For those, teams sometimes use `pgx` directly. For this module we use `database/sql` + `pgx` driver — the right starting point.

---

## 2. The Connection Pool — How It Really Works

### The problem `sql.Open` solves

Every network connection has a cost:
- TCP three-way handshake
- PostgreSQL authentication (SSL negotiation, password check, session setup)
- Memory allocation on both client and server

If your API opens a new connection for every HTTP request and closes it when done, a server handling 500 requests/second is doing 500 connects and 500 disconnects per second to PostgreSQL. PostgreSQL has a default max of 100 connections. You'd hit that ceiling almost immediately.

### What the pool does

`sql.Open` creates a **connection pool manager**. It does not open any connection to the database yet — it just creates the object that *will manage* connections.

```
sql.Open("pgx", dsn)
  → creates pool manager
  → zero actual connections right now
  → even if dsn is wrong, no error here

db.Ping()
  → pool opens one real connection
  → authenticates with PostgreSQL
  → THIS is where you find out if dsn is wrong
```

The pool maintains two groups of connections:

```
┌─────────────────────────────────┐
│         Connection Pool         │
│                                 │
│  In-use:  [conn1] [conn2]       │  ← borrowed by active queries
│  Idle:    [conn3] [conn4]       │  ← waiting for next request
│                                 │
└─────────────────────────────────┘
```

When a query comes in:
1. Pool checks if any idle connection exists → gives it to you
2. If no idle connection but total < MaxOpenConns → opens a new one
3. If total == MaxOpenConns → **your goroutine blocks** and waits
4. If waiting too long → `context deadline exceeded` error

### The four tuning knobs

```go
db.SetMaxOpenConns(25)
// Maximum number of connections (idle + in-use) at once.
// If 25 are in use and request 26 comes in, it waits.
// Default: unlimited (dangerous — can overwhelm PostgreSQL)

db.SetMaxIdleConns(25)
// Maximum connections sitting idle in the pool.
// If MaxOpenConns=25 and MaxIdleConns=25, pool stays warm — 
// no teardown/setup between bursts of traffic.
// Default: 2 (too low for most production apps)

db.SetConnMaxLifetime(5 * time.Minute)
// A connection is retired after this duration, even if idle.
// Prevents stale connections after PostgreSQL restarts or 
// firewall rules cut old connections silently.
// Without this: you get "broken pipe" errors after PostgreSQL restarts.

db.SetConnMaxIdleTime(1 * time.Minute)
// A connection is retired if idle for this long.
// Saves resources during traffic lulls.
// Added in Go 1.15.
```

**Production recommendation:** Start with `MaxOpenConns(25)`, `MaxIdleConns(25)`, `ConnMaxLifetime(5 * time.Minute)`. Tune based on observed `db.Stats()` under load.

### Why `db` is safe to share across goroutines

`*sql.DB` is fully goroutine-safe. The pool has an internal mutex. Every HTTP handler can call `db.Query(...)` simultaneously — the pool serializes access to connections internally. You create **one** `*sql.DB` for your whole application and pass it around.

---

## 3. Querying — `Query` vs `QueryRow` vs `Exec`

Go gives you three methods for running SQL, each for a different situation:

### `db.QueryRow` — exactly one row expected

```go
row := db.QueryRow("SELECT id, title FROM books WHERE id = $1", id)
```

- Returns a `*sql.Row` immediately (no error yet)
- Error is deferred until you call `.Scan()`
- If zero rows match: `Scan` returns `sql.ErrNoRows`
- If more than one row matches: only the first is returned, rest discarded

Use for: `SELECT` by primary key, any query where you expect exactly one result.

### `db.Query` — zero or more rows

```go
rows, err := db.Query("SELECT id, title, author FROM books")
if err != nil {
    // connection failed, syntax error, etc.
}
defer rows.Close() // CRITICAL — explained below
```

- Returns `*sql.Rows` and an error
- Error here means the query never started (no connection, bad SQL, etc.)
- You then iterate with `rows.Next()` and call `rows.Scan()` on each row
- After the loop, check `rows.Err()` — errors *during* iteration land there

Use for: any `SELECT` that returns a list.

### `db.Exec` — no rows returned

```go
result, err := db.Exec(
    "INSERT INTO books (title, author) VALUES ($1, $2)",
    "The Go Programming Language", "Donovan",
)
```

- Returns `sql.Result` with `LastInsertId()` and `RowsAffected()`
- Note: PostgreSQL does not support `LastInsertId()` — use `RETURNING id` with `QueryRow` instead
- Use for: `INSERT`, `UPDATE`, `DELETE`, `CREATE TABLE`

---

## 4. `Scan` — Why You Pass Pointers

`Scan` is the function that moves data from a database row into your Go variables. The key question beginners ask: why do you pass `&book.Title` instead of `book.Title`?

Because `Scan` needs to *write into* your variables. If you pass `book.Title` (a string value), Scan receives a copy — any write to the copy doesn't affect the original. If you pass `&book.Title` (a pointer to the string), Scan can write through the pointer directly into your struct field.

```go
// What Scan does internally (simplified):
func (r *Row) Scan(dest ...any) error {
    for i, col := range r.columns {
        // col is raw bytes from PostgreSQL wire protocol
        // dest[i] is your pointer, e.g. *string or *int
        switch d := dest[i].(type) {
        case *string:
            *d = string(col)   // write through pointer
        case *int:
            *d = parseInt(col) // write through pointer
        }
    }
}
```

**Column order matters.** Scan maps columns positionally — first column to first argument, second to second, etc. If your SELECT returns `id, title, author` but you scan into `&title, &id, &author`, you'll silently put the id value into title. Always match scan arguments to SELECT column order.

```go
// RIGHT — matches SELECT id, title, author
rows.Scan(&book.ID, &book.Title, &book.Author)

// WRONG — silent data corruption
rows.Scan(&book.Title, &book.ID, &book.Author)
```

**Nullable columns.** If a column can be NULL in PostgreSQL, you cannot scan it into a plain `string` or `int` — Scan will return an error. You must use `sql.NullString`, `sql.NullInt64`, etc.:

```go
var description sql.NullString
rows.Scan(&book.ID, &book.Title, &description)
if description.Valid {
    book.Description = description.String
}
```

---

## 5. `sql.ErrNoRows` — Not a Real Error

When `QueryRow` finds no matching rows, it returns `sql.ErrNoRows` from `.Scan()`. This is one of Go's more surprising design decisions — it models "not found" as an error even though it's a perfectly normal, expected outcome.

Why? Because `QueryRow` returns `*sql.Row` immediately without checking results (the query is lazy). By the time you call `.Scan()`, the only way to signal "nothing was found" is via the error return value.

**The right way to handle it:**

```go
book, err := getByID(db, 42)
if errors.Is(err, sql.ErrNoRows) {
    // This is not a crash — it's a 404
    http.NotFound(w, r)
    return
}
if err != nil {
    // This IS a real problem — database is down, query is broken, etc.
    http.Error(w, "Internal Server Error", 500)
    return
}
```

Use `errors.Is` not `err == sql.ErrNoRows` because in later modules you'll wrap errors and `==` breaks on wrapped errors.

---

## 6. SQL Injection — The Most Dangerous Mistake

SQL injection is when user input is concatenated directly into a SQL string and executed as code. It has been the most common web vulnerability for over 20 years.

**Vulnerable code:**

```go
// NEVER DO THIS
query := "SELECT * FROM books WHERE title = '" + userInput + "'"
db.Query(query)
```

If `userInput` is `' OR '1'='1`, the resulting SQL is:

```sql
SELECT * FROM books WHERE title = '' OR '1'='1'
```

That returns every row in the table. If the input is `'; DROP TABLE books; --`, you lose your data.

**Safe code — parameterised queries:**

```go
// ALWAYS DO THIS
db.Query("SELECT * FROM books WHERE title = $1", userInput)
```

PostgreSQL placeholders are `$1`, `$2`, `$3` (MySQL uses `?`). The driver sends the SQL and the parameters *separately* over the wire. PostgreSQL receives them as distinct things — it can never mistake a parameter value for SQL syntax. The parameter is always treated as data, never as code.

**Rule:** Never use `fmt.Sprintf` or string concatenation to build SQL. Always use `$1`, `$2` placeholders.

---

## 7. Transactions — ACID and When You Need Them

A transaction groups multiple SQL statements into a single atomic unit. Either all statements succeed together, or none of them do.

**ACID** (what transactions guarantee):

| Letter | Meaning | Simple version |
|--------|---------|----------------|
| **A**tomicity | All or nothing | If step 2 fails, step 1 is undone |
| **C**onsistency | DB goes from one valid state to another | You can't end up with half a transfer |
| **I**solation | Concurrent transactions don't see each other mid-flight | Two transfers running at once don't interfere |
| **D**urability | Committed data survives crashes | Once you see "commit", it's on disk |

**When you need a transaction:**

You need a transaction any time you have two or more statements that must succeed or fail *together*. The classic example is a bank transfer:

```
Step 1: Deduct $100 from account A
Step 2: Add $100 to account B
```

If step 1 succeeds and your server crashes before step 2, you've lost $100. A transaction ensures both steps happen atomically — PostgreSQL will roll back step 1 if step 2 never commits.

In your book API: if you're moving a book between categories and updating a "last modified" audit log simultaneously, that's two statements that need a transaction.

**What happens if you forget to commit:**

```go
tx, _ := db.Begin()
tx.Exec("UPDATE books SET title = $1 WHERE id = $2", "New Title", 1)
// forgot tx.Commit()
// tx.Rollback() is called by defer
// → UPDATE never happened
```

Transactions that aren't committed are automatically rolled back when the connection is returned to the pool. Always use `defer tx.Rollback()` as a safety net, and call `tx.Commit()` explicitly when done. The `Rollback` after a successful `Commit` is a no-op, so the pattern is safe:

```go
tx, err := db.Begin()
defer tx.Rollback() // no-op if Commit was called

// ... do work ...

tx.Commit() // this wins; defer Rollback does nothing
```

---

## 8. Migrations — Why Not Hardcode `CREATE TABLE`

A migration is a versioned SQL file that describes a single change to your database schema. The alternative — running `db.Exec("CREATE TABLE ...")` in your `main.go` — has serious problems in production:

**Problems with hardcoded schema in Go:**

- Rerunning the app drops and recreates tables, destroying data
- You can't see the history of schema changes in git
- Multiple developers can't coordinate schema changes safely
- Deploying a schema change requires careful manual steps

**What migrations give you:**

```
migrations/
  000001_create_books.up.sql
  000001_create_books.down.sql
  000002_add_author_column.up.sql
  000002_add_author_column.down.sql
```

Each migration has an `up` (apply the change) and a `down` (undo it). A migration tool (like `golang-migrate`) tracks which migrations have been applied in a `schema_migrations` table. Running `migrate up` applies only the unapplied ones. Running `migrate down` rolls back one step.

**The key insight:** Your schema lives in version control as SQL files, not buried in Go code. Every change is tracked, reversible, and reproducible across dev/staging/production.

For this module we'll write migrations manually as `.sql` files and run them with `psql`. Module 8 (Project Structure) introduces `golang-migrate` properly.

---

## 9. `defer rows.Close()` — The Rule You Cannot Forget

When you call `db.Query(...)`, PostgreSQL starts sending rows over the network. The driver buffers them using a database connection. That connection stays **checked out from the pool** until you close the rows.

```go
rows, err := db.Query("SELECT ...")
// ← connection is now checked out from pool

defer rows.Close()
// ← this returns the connection to the pool

for rows.Next() {
    rows.Scan(...)
}
```

If you forget `rows.Close()`:
- The connection is never returned to the pool
- If this happens in a loop or under load, the pool drains
- New queries start waiting for connections → timeouts → your API stops responding

`rows.Close()` is idempotent (safe to call multiple times) and `rows.Next()` returning `false` after normal iteration calls it automatically — but the `defer` is a safety net for early returns and error paths.

**Rule:** Always `defer rows.Close()` immediately after checking the error from `db.Query()`.

---

## 10. `RETURNING` — Getting the New ID from PostgreSQL

MySQL has `LastInsertId()` on `sql.Result`. PostgreSQL does not reliably support this. Instead, PostgreSQL has the `RETURNING` clause:

```sql
INSERT INTO books (title, author) VALUES ($1, $2) RETURNING id
```

This treats the INSERT like a SELECT — it returns a row with the newly generated `id`. You handle it with `QueryRow` + `Scan`:

```go
var newID int
err := db.QueryRow(
    "INSERT INTO books (title, author) VALUES ($1, $2) RETURNING id",
    title, author,
).Scan(&newID)
```

This is idiomatic PostgreSQL in Go. Any time you need the value generated by a `DEFAULT` or `SERIAL` column after an insert, use `RETURNING`.
