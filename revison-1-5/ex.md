# Big Exercise — Library Lending API

**Purpose:** one project that forces Modules 1–5 to work together. Nothing here is new theory — it's the same concepts you already learned, combined the way they actually get combined in a real service.

No solution code below. Build it, paste what you have when you get stuck or when it works, and I'll review.

---

## The domain

A library lending system. Three entities:

- **Book** — `id, title, author, total_copies, available_copies`
- **Member** — `id, name, email`
- **Loan** — `id, book_id, member_id, borrowed_at, due_at, returned_at (nullable)`

The interesting part: borrowing and returning a book must be **atomic**. Borrowing means "check a copy is available AND create the loan AND decrement `available_copies`" — all three, or none. That's your transaction from Module 5, for a reason that actually matters (two members can't take the last copy at once).

---

## Endpoints

| Method | Path | Does |
|---|---|---|
| `POST` | `/books` | create a book |
| `GET` | `/books` | list all books |
| `GET` | `/books/{id}` | get one book |
| `DELETE` | `/books/{id}` | delete a book |
| `POST` | `/members` | create a member |
| `POST` | `/loans` | borrow a book (body: `book_id`, `member_id`) |
| `POST` | `/loans/{id}/return` | return a book |
| `GET` | `/members/{id}/loans` | list a member's loans (active + returned) |

---

## What each module contributes

**Module 1 — Fundamentals**
- Define a `BookRepository` interface (`GetAll`, `GetByID`, `Create`, `Delete`) and a `LoanRepository` interface (`Create`, `Return`, `ListByMember`) before writing any Postgres code. Your Postgres structs implement these interfaces — this is the same `Storer` pattern from your Module 1 drill.
- All repository methods return `(result, error)` — no panics for "not found" or bad input.
- Write one background goroutine: every 30 seconds, scan for loans where `due_at < now()` and `returned_at IS NULL`, and log the overdue ones. This runs concurrently with your HTTP server — think about how you start it in `main` without blocking `ListenAndServe`.

**Module 2 — `net/http`**
- Build your own `*http.ServeMux`, never the default one.
- A `routes()` method on your `application` struct that returns `http.Handler` — keep `main` clean.
- Get the write-order right in every handler: headers → `WriteHeader` → body.

**Module 3 — Routing**
- Every route above uses Go 1.22 method-prefixed patterns and `{id}`/`{param}` path values. No manual `r.Method` switches, no query-string IDs.

**Module 4 — JSON**
- Every response goes through one `writeJSON` helper using an envelope: `{"book": {...}}` or `{"error": "..."}`.
- Every request body goes through one `decodeJSON` helper.
- Struct tags: `available_copies` etc. in snake_case JSON, `json:"-"` on anything that shouldn't leave the server (there isn't much here, but think about whether `Member.Email` should ever be excluded — trick yourself into deciding, not just copying).
- Borrowing a book that has 0 `available_copies` → `409 Conflict` with an error envelope, not a 500.
- Not found (bad book/member/loan ID) → `404`.

**Module 5 — Database**
- Migrations: `.up.sql` / `.down.sql` for `books`, `members`, `loans`.
- Connection pool configured (`SetMaxOpenConns` etc.), `db.Ping()` on startup, fail fast if the DB isn't reachable.
- `POST /loans` (borrow) is a transaction:
  1. `SELECT available_copies FROM books WHERE id = $1 FOR UPDATE` (row lock — look up why `FOR UPDATE` matters here, it's new but follows directly from what you know about isolation)
  2. if `0`, roll back, return `409`
  3. `UPDATE books SET available_copies = available_copies - 1 WHERE id = $1`
  4. `INSERT INTO loans (...) VALUES (...) RETURNING id`
  5. commit
- `POST /loans/{id}/return` is also a transaction: set `returned_at = now()`, increment `available_copies` back. Both statements or neither.
- Every `GetByID`-style lookup uses `errors.Is(err, sql.ErrNoRows)`.
- Every query uses `$1, $2` placeholders — nowhere does user input touch a string concatenation.

---

## Schema to start from

```sql
CREATE TABLE books (
    id               SERIAL PRIMARY KEY,
    title            TEXT NOT NULL,
    author           TEXT NOT NULL,
    total_copies     INT NOT NULL,
    available_copies INT NOT NULL
);

CREATE TABLE members (
    id    SERIAL PRIMARY KEY,
    name  TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE
);

CREATE TABLE loans (
    id          SERIAL PRIMARY KEY,
    book_id     INT NOT NULL REFERENCES books(id),
    member_id   INT NOT NULL REFERENCES members(id),
    borrowed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    due_at      TIMESTAMPTZ NOT NULL,
    returned_at TIMESTAMPTZ
);
```

---

## Order to build it in

1. Migrations + `openDB` + `Ping` — confirm you can connect before writing anything else.
2. `BookRepository` interface + Postgres implementation + `GET/POST/DELETE /books` — this is a re-run of Module 5's mini project, should be fast.
3. `MemberRepository` + `POST /members` — same shape, less thinking.
4. The borrow transaction (`POST /loans`) — this is the hard part. Get it working *without* the row lock first, then add `FOR UPDATE` and explain to yourself why it's needed.
5. The return transaction.
6. `GET /members/{id}/loans`.
7. The background overdue-scanner goroutine last — it touches every table but nothing about it is new.

---

## How you'll know it's right

```bash
# create a book with 1 copy
curl -X POST localhost:8080/books -d '{"title":"Dune","author":"Herbert","total_copies":1,"available_copies":1}'

# create two members
curl -X POST localhost:8080/members -d '{"name":"Alice","email":"a@x.com"}'
curl -X POST localhost:8080/members -d '{"name":"Bob","email":"b@x.com"}'

# Alice borrows the only copy
curl -X POST localhost:8080/loans -d '{"book_id":1,"member_id":1}'   # 201

# Bob tries to borrow the same book
curl -X POST localhost:8080/loans -d '{"book_id":1,"member_id":2}'   # 409, not 500, not a phantom loan

# Alice returns it
curl -X POST localhost:8080/loans/1/return

# Bob can now borrow it
curl -X POST localhost:8080/loans -d '{"book_id":1,"member_id":2}'   # 201
```

If Bob's first attempt ever succeeds, or ever leaves `available_copies` at a wrong number after a crash mid-request, your transaction isn't atomic — that's the whole point of this exercise.

---

Start with step 1. Paste your `openDB` + migration code when it's running, and we'll go from there.