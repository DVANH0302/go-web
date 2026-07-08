# Go's `context.Context` (Exercises)

Small, focused drills. Do them in order.

---

### Drill 1 — Values walking up the chain

Write a small `main.go` that:
- Creates `context.Background()`
- Calls `WithValue` three times in a row, each wrapping the previous context,
  with three different keys (`"A"`, `"B"`, `"C"`)
- Prints `ctx.Value()` for all three keys, plus a fourth key you never set

Confirm all three set values are found correctly, and the unset key returns `nil`.

---

### Drill 2 — Manual cancellation

Write a goroutine that:
- Waits on `ctx.Done()` in a `select`
- In `main`, call `cancel()` after 1 second (via a separate goroutine or a
  `time.Sleep`)
- Print `ctx.Err()` after the goroutine unblocks

Confirm you see `context canceled` (not `context deadline exceeded` — that's
the next drill).

---

### Drill 3 — Timeout vs manual cancel

Repeat Drill 2, but this time use `context.WithTimeout(ctx, 2*time.Second)`
instead of `WithCancel`, and don't call `cancel()` manually — let it expire
naturally.

Confirm `ctx.Err()` returns `context.DeadlineExceeded` this time. Notice the
different error depending on *how* the context ended.

---

### Drill 4 — One parent, multiple children

Create one parent context with `WithTimeout` (2 seconds). Derive **three**
separate child contexts from it with `WithCancel`. Launch a goroutine per
child that waits on that child's `Done()` and prints which child stopped and
why.

Confirm all three children report `context deadline exceeded` at
approximately the same time — proving the parent's timeout cancelled all of
them, not just one.

---

### Drill 5 — Context in an HTTP handler

Write a handler that:
- Reads a `userID` you set via a middleware (`WithValue`) using a custom
  unexported key type
- Returns it in the JSON response, e.g. `{"user_id": 42}`

Confirm the value survives from middleware into the handler, and that
changing the key type (e.g. using a plain string key by mistake) still works
technically, but discuss: why is that risky in a larger codebase with many
packages?

---

### Drill 6 — Simulated slow database call

Write a fake `slowQuery(ctx context.Context) error` function that sleeps for
5 seconds, but checks `ctx.Done()` partway through using a `select`, so it
can bail out early if cancelled.

Call it with a context that has a 2-second timeout. Confirm:
- The function returns early (around 2 seconds, not 5)
- The returned error reflects `context.DeadlineExceeded`

This mirrors what `QueryRowContext` does internally with real database
drivers.

---

## Done?

Once all six drills pass, you understand both halves of `context.Context`:
carrying request-scoped values upward through a lookup chain, and propagating
cancellation downward through a tree of children — plus how both plug into
real HTTP handlers and database calls.
