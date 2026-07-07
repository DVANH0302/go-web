# Go's `context.Context` (Theory)

## The Problem It Solves

Imagine a single incoming HTTP request that needs to:
1. Query a database
2. Call an external API
3. Spawn a few goroutines to fetch things in parallel

All of that work exists **only to serve this one request**. If the client
disconnects, or a timeout fires, every one of those in-flight operations is now
doing pointless work — nobody will ever read the result.

Without some shared mechanism, you'd have to manually thread "please stop" and
"here's some request data" through every function call, every goroutine, by
hand, every time. `context.Context` exists to solve two problems at once:

1. **Cancellation / deadlines** — a way to signal "stop, this work is no
   longer needed" that propagates automatically through a whole call tree.
2. **Request-scoped values** — a way to carry data (like an authenticated
   user ID) alongside a request without changing every function signature to
   accept an extra parameter.

---

## It's a Tree, Not a Single Chain

A `context.Context` is an **interface**:

```go
type Context interface {
    Deadline() (deadline time.Time, ok bool)
    Done() <-chan struct{}
    Err() error
    Value(key any) any
}
```

You never mutate a context. Every "derive a new context" function takes a
parent and returns a **brand new child** that wraps it:

```go
ctx2 := context.WithValue(ctx1, key, value)
ctx3, cancel := context.WithCancel(ctx2)
ctx4, cancel := context.WithTimeout(ctx3, 5*time.Second)
```

Each of these wraps the previous one, like a node pointing at its parent.
Going **upward** from any node to the root is a single chain. Going
**downward**, one parent can have **many children** — call `WithValue` (or
`WithCancel`/`WithTimeout`) on the same parent multiple times and you get
multiple independent children:

```
requestCtx
   ├── dbQueryCtx      (child 1)
   ├── externalAPICtx  (child 2)
   └── cacheCtx        (child 3)
```

So structurally it's a **tree**: one parent per node, but a node can have any
number of children. There is no operation to give a context two parents —
every "WithX" function takes exactly one parent and returns exactly one child.
If you call `WithValue` on two different parents, you get two completely
separate, unrelated contexts — not one context with two parents.

---

## Values: `WithValue`

```go
ctx := context.WithValue(parent, key, value)
```

Internally this is roughly:

```go
type valueCtx struct {
    Context      // embedded parent
    key, val any
}

func (c *valueCtx) Value(key any) any {
    if c.key == key {
        return c.val
    }
    return c.Context.Value(key) // ask the parent
}
```

Looking up a value walks **upward**: check this node, if no match ask the
parent, then the grandparent, and so on until something matches or you hit
the root (which returns `nil`).

**Why not just use a map?**
- Immutability — nothing downstream can accidentally overwrite what something
  upstream set, since each layer is a new object.
- Type-safe keys — you're expected to use unexported custom key types (not
  plain strings), so two packages can't accidentally collide by using the
  same key name.
- It shares the same interface as cancellation/deadlines, so one object
  carries both concerns through the same call chain.

---

## Cancellation: `WithCancel`, `WithTimeout`, `WithDeadline`

```go
ctx, cancel := context.WithTimeout(parent, 5*time.Second)
defer cancel()
```

Each of these gives you a context with a `done` channel. Nothing is deleted
or "removed" from anything when cancellation happens — a channel gets
**closed**, which is a one-way broadcast signal in Go: once closed, any
`<-ctx.Done()` read returns immediately, and stays that way forever.

```go
select {
case <-ctx.Done():
    // stop working, ctx.Err() explains why
case result := <-workDone:
    // finished normally
}
```

`ctx.Err()` tells you why it stopped:
- `context.Canceled` — someone called `cancel()` manually
- `context.DeadlineExceeded` — a timeout/deadline was reached

**Propagation is the key feature.** When a parent is cancelled, it walks its
internal registry of children and cancels each of them too — which cancels
their children, and so on. This is intentional: everything spawned to serve
a request should stop when the request itself is no longer needed. There's
no legitimate case where a child should keep working after its parent (the
reason it existed) has been cancelled.

**Cancellation is cooperative, not forced.** Closing a channel doesn't kill a
goroutine by itself — your code has to actively check `ctx.Done()` or
`ctx.Err()` and choose to stop. A goroutine that never checks context will
keep running regardless.

---

## Where Does a Request's Context Live?

`*http.Request` carries a context internally, accessed via:

```go
func (r *Request) Context() context.Context
func (r *Request) WithContext(ctx context.Context) *Request
```

`WithContext` doesn't mutate the request — it returns a **shallow copy** with
the new context attached. This is why middleware always looks like:

```go
ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
r = r.WithContext(ctx)   // r now refers to a new *Request
next.ServeHTTP(w, r)
```

You reassign `r` to the copy and pass that copy onward. The original request
object is untouched.

---

## Rule of Thumb: `context.Background()`

If you genuinely need something to keep running independent of the request's
lifetime (e.g., "log this even after the client disconnected"), you don't
derive from the request's context — you start fresh with `context.Background()`,
an unrelated root context with no parent and nothing to cancel it.

---

## Summary

- `Context` is a tree: one parent, any number of children, no way to have two parents.
- `WithValue` chains upward for lookups; `Value()` walks toward the root.
- `WithCancel`/`WithTimeout` close a channel that propagates downward to every
  descendant automatically.
- Nothing is "removed" — cancellation is a broadcast signal, cleanup of the
  actual objects is ordinary Go garbage collection once nothing references them.
- Cancellation is cooperative — code must check it, nothing stops a goroutine
  by force.



Fair — let me slow down and go one at a time, since I've been throwing names around assuming they were all equally clear.

There are really **two families** here, plus one that's not even part of `context` package at all. Let's separate them cleanly.

## Family 1: Functions that create/derive a context

These all live in the `context` package and all do the same basic thing — take a parent context, return a new child context. They differ only in *what extra behavior* that child has.

### `context.WithValue(parent, key, value)`
**Purpose:** attach a piece of data to the context, nothing else.
**Use when:** you want to carry request-scoped data (user ID, request ID, tenant ID) down through function calls.
**Does NOT** give you cancellation — the child just inherits whatever cancellation behavior the parent already had.

```go
ctx := context.WithValue(parentCtx, userIDKey, 42)
```

### `context.WithCancel(parent)`
**Purpose:** get a context you can cancel **manually**, on your own schedule — no timer involved.
**Use when:** you want to stop something based on an *event*, not a fixed duration — e.g. "cancel this if the user clicks Stop" or "cancel worker B if worker A already found the answer."
**Returns two things:** the new context, and a `cancel()` function you call yourself.

```go
ctx, cancel := context.WithCancel(parentCtx)
defer cancel() // always call this, even if you "let it expire" some other way
```

### `context.WithTimeout(parent, duration)`
**Purpose:** get a context that cancels **itself automatically** after a fixed duration.
**Use when:** you know how long something should be allowed to take — e.g. "this DB query should time out after 3 seconds," "this HTTP call should give up after 5 seconds."

```go
ctx, cancel := context.WithTimeout(parentCtx, 3*time.Second)
defer cancel()
```

Internally it's really just `WithCancel` + a `time.AfterFunc` that calls `cancel()` for you when the timer fires. Same shape, just automated.

### `context.WithDeadline(parent, time.Time)`
**Purpose:** identical to `WithTimeout`, except you give an absolute point in time instead of a duration.
**Use when:** you have a specific moment in mind — e.g. "this must finish by 5:00pm" rather than "give it 3 seconds from now." `WithTimeout` is actually implemented as `WithDeadline(parent, time.Now().Add(duration))` under the hood — it's just the more convenient version for the common case.

**So all four of these answer: "what NEW capability does this child context add to its parent?"**

| Function | Adds |
|---|---|
| `WithValue` | a key/value pair |
| `WithCancel` | a manual cancel switch |
| `WithTimeout` | an automatic cancel-after-duration |
| `WithDeadline` | an automatic cancel-at-a-specific-time |

You can stack them — e.g. `WithValue` on top of a `WithTimeout` context — because each just wraps whatever came before it.

---

## Family 2 (not really a family, just one thing): `request.WithContext(ctx)`

This one is completely different and easy to confuse because of the similar name. It's a method on `*http.Request`, **not** on `context.Context` itself, and it's not about creating a context — it's about **attaching** a context you already built to an HTTP request.

```go
func (r *Request) WithContext(ctx context.Context) *Request
```

**Purpose:** `*http.Request` carries a context internally (accessible via `r.Context()`), but you can't mutate that field directly — Go doesn't let you write `r.ctx = newCtx`. So `WithContext` gives you back a **copy of the request** with the new context swapped in.

**Use when:** in middleware, after you've built a new context (usually via `WithValue`), and you need to get that context "into" the request so the next handler sees it.

```go
// build a new context with a value in it
ctx := context.WithValue(r.Context(), userIDKey, 42)

// attach that context to a NEW copy of the request
r = r.WithContext(ctx)

// pass the new request onward
next.ServeHTTP(w, r)
```

So the flow in middleware is always: **derive** a context (`WithValue`/`WithCancel`/etc. — Family 1), then **attach** it to the request (`r.WithContext` — the one-off method), then pass the new request down.

---

## Quick decision guide

- Need to carry data (user ID, request ID)? → `WithValue`
- Need to cancel on your own trigger/event? → `WithCancel`
- Need an automatic time limit, relative ("in 3 seconds")? → `WithTimeout`
- Need an automatic time limit, absolute ("by 5pm")? → `WithDeadline`
- Need to get a context you built into an `*http.Request`? → `r.WithContext(ctx)`

Want me to add this comparison as a section in the context md files, or is the inline explanation enough?