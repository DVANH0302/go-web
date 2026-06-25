# Module 05 — Database (PostgreSQL): Cheatsheet

---

## Setup at a Glance

```bash
go get github.com/jackc/pgx/v5/stdlib   # install driver
```

```go
import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"  // blank import registers the driver
)
```

```go
// DSN format for PostgreSQL
"postgres://USER:PASSWORD@HOST:PORT/DBNAME?sslmode=disable"
```

---

## Open & Configure the Pool

```go
db, err := sql.Open("pgx", dsn)     // does NOT connect yet
db.Ping()                            // THIS actually connects and authenticates

db.SetMaxOpenConns(25)               // max total connections (idle + in-use)
db.SetMaxIdleConns(25)               // max idle connections kept warm
db.SetConnMaxLifetime(5 * time.Minute) // retire connections after N time
db.SetConnMaxIdleTime(1 * time.Minute) // retire idle connections after N time
defer db.Close()                     // call in main(), not in handlers
```

---

## The 3 Query Methods — When to Use Each

| Method | Returns | Use When |
|--------|---------|----------|
| `db.QueryRow(sql, args...)` | `*sql.Row` | Expecting exactly 1 row (GET by ID) |
| `db.Query(sql, args...)` | `*sql.Rows, error` | Expecting 0 or more rows (list) |
| `db.Exec(sql, args...)` | `sql.Result, error` | No rows returned (INSERT/UPDATE/DELETE) |

---

## QueryRow + Scan

```go
var b Book
err := db.QueryRow(
    "SELECT id, title, author FROM books WHERE id = $1", id,
).Scan(&b.ID, &b.Title, &b.Author)   // pointers! order matches SELECT

if errors.Is(err, sql.ErrNoRows) { /* not found → 404 */ }
if err != nil                    { /* real error → 500 */ }
```

---

## Query + Rows

```go
rows, err := db.Query("SELECT id, title, author FROM books")
if err != nil { /* query failed to start */ }
defer rows.Close()   // ← NEVER FORGET — returns connection to pool

var books []Book
for rows.Next() {
    var b Book
    rows.Scan(&b.ID, &b.Title, &b.Author)  // order matches SELECT
    books = append(books, b)
}
if err := rows.Err(); err != nil { /* error during iteration */ }
```

---

## Insert with RETURNING (PostgreSQL way)

```go
// PostgreSQL doesn't support LastInsertId() — use RETURNING instead
var newID int
err := db.QueryRow(
    "INSERT INTO books (title, author) VALUES ($1, $2) RETURNING id",
    title, author,
).Scan(&newID)
```

---

## Delete with RowsAffected

```go
result, err := db.Exec("DELETE FROM books WHERE id = $1", id)
n, _         := result.RowsAffected()
if n == 0 { /* id didn't exist */ }
```

---

## Transactions

```go
tx, err := db.Begin()
if err != nil { return err }
defer tx.Rollback()   // no-op if Commit was called; safety net otherwise

_, err = tx.Exec("UPDATE ...")
if err != nil { return err }   // Rollback fires via defer

_, err = tx.Exec("INSERT ...")
if err != nil { return err }   // Rollback fires via defer

return tx.Commit()   // makes everything permanent; defer Rollback is now no-op
```

---

## ErrNoRows — The "Not Found" Sentinel

```go
import "errors"

// sql.ErrNoRows is returned by QueryRow.Scan() when zero rows match
if errors.Is(err, sql.ErrNoRows) {
    // NOT a crash — it's a 404
    http.NotFound(w, r)
    return
}
```

**Use `errors.Is`, not `err == sql.ErrNoRows`** — `==` breaks when errors are wrapped.

---

## Placeholders

| Database | Placeholder style |
|----------|-------------------|
| PostgreSQL | `$1, $2, $3` |
| MySQL | `?, ?, ?` |
| SQLite | `?, ?, ?` or `$1, $2` |

```go
// RIGHT — parameterised, injection-proof
db.Query("SELECT * FROM books WHERE title = $1", userInput)

// WRONG — string concatenation, SQL injection risk
db.Query("SELECT * FROM books WHERE title = '" + userInput + "'")
```

---

## Nullable Columns

```go
// If a column can be NULL in PostgreSQL, you can't scan into a plain string.
// Use sql.NullString, sql.NullInt64, sql.NullTime, etc.
var desc sql.NullString
rows.Scan(&b.ID, &b.Title, &desc)
if desc.Valid {
    b.Description = desc.String   // only access .String if .Valid is true
}
```

---

## Pool Diagnostics

```go
stats := db.Stats()
fmt.Println(stats.OpenConnections)   // total open connections right now
fmt.Println(stats.InUse)             // connections currently running a query
fmt.Println(stats.Idle)              // connections waiting in the pool
fmt.Println(stats.WaitCount)         // requests that had to wait for a connection
fmt.Println(stats.WaitDuration)      // total time spent waiting for connections
```

---

## Migration File Convention

```
migrations/
  000001_create_books.up.sql    ← apply this change
  000001_create_books.down.sql  ← undo this change
  000002_add_year_column.up.sql
  000002_add_year_column.down.sql
```

```sql
-- 000001_create_books.up.sql
CREATE TABLE IF NOT EXISTS books (
    id      SERIAL PRIMARY KEY,
    title   TEXT NOT NULL,
    author  TEXT NOT NULL,
    year    INT  NOT NULL,
    created TIMESTAMPTZ DEFAULT NOW()
);

-- 000001_create_books.down.sql
DROP TABLE IF EXISTS books;
```

```bash
psql -U postgres -d booksdb -f migrations/000001_create_books.up.sql
```

---

## Common Errors & What They Mean

| Error message | Cause | Fix |
|---------------|-------|-----|
| `pq: password authentication failed` | Wrong password in DSN | Check DSN credentials |
| `dial tcp: connection refused` | PostgreSQL not running or wrong port | Start PostgreSQL, check port |
| `pq: database "xyz" does not exist` | Database not created | Run `CREATE DATABASE xyz` |
| `sql: no rows in result set` | `QueryRow` found nothing | Handle with `errors.Is(err, sql.ErrNoRows)` |
| `context deadline exceeded` | Pool exhausted, waited too long | Increase `MaxOpenConns` or find slow query |
| `pq: column "foo" does not exist` | Wrong column name in SQL | Check your schema, rerun migration |
| All connections used, scan returns garbage | Forgot `defer rows.Close()` | Always `defer rows.Close()` after `db.Query()` |

---

## The 5 Rules That Prevent 90% of Bugs

```
1. Always defer rows.Close() immediately after db.Query()
2. Always check rows.Err() after the for rows.Next() loop
3. Always use $1 placeholders — never concatenate user input into SQL
4. Always use errors.Is(err, sql.ErrNoRows) — never err == sql.ErrNoRows
5. Always defer tx.Rollback() in transactions — commit explicitly
```

---

## Module 5 Drills Checklist

- [ ] **Drill 1** — Connect to PostgreSQL, ping it, print success
- [ ] **Drill 2** — Write raw `SELECT *` for all books, print them
- [ ] **Drill 3** — Write `GetByID`, scan one row, handle `ErrNoRows`
- [ ] **Drill 4** — Write `Create`, insert a book, return new ID with `RETURNING`
- [ ] **Drill 5** — Write `Delete`, return error if not found via `RowsAffected`
- [ ] **Drill 6** — Write `Transfer` transaction, roll back if either step fails
- [ ] **Mini Project** — Replace the in-memory `[]Book` slice in your book API with a real PostgreSQL database. Every handler hits the database.
