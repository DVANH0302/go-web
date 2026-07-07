# Module 7 — Authentication & Authorization (Theory)

## The Problem

HTTP is stateless. The server has no memory of who you are between requests.

You send `POST /login` with your credentials. The server checks them, says "yes, that's you."
Then you send `DELETE /books/5`. From the server's point of view, this is a brand new,
anonymous request. It has no idea it's the same person who just logged in.

So how does the server know who you are on the *second* request? That's the entire
problem this module solves.

---

## Two Classic Solutions

### 1. Sessions
- Server generates a random session ID after login.
- Server stores that ID somewhere (memory, Redis, DB) mapped to the user.
- Server sends the ID to the client as a cookie.
- On every future request, the client sends the cookie back, and the server looks up
  the session to find out who's making the request.

### 2. JWT (JSON Web Token)
- Server creates a signed token that *contains* the user's identity directly.
- Server hands the token to the client (usually in the response body).
- Client sends the token back on every request, usually in an `Authorization` header.
- Server does **not** store anything. It just verifies the signature and trusts what's inside.

### The tradeoff

| | Sessions | JWT |
|---|---|---|
| Server storage | Yes — needs a session store | No — fully stateless |
| Revoking access early | Easy — delete the session | Hard — valid until expiry |
| Scaling across servers | Needs a shared session store | Trivial — any server can verify it |
| Typical use case | Traditional server-rendered web apps | APIs, especially distributed ones |

We're building an API, so we'll use **JWT**.

---

## JWT Internals

A JWT is a single string made of three parts separated by dots:

```
header.payload.signature
```

### Header
A small JSON object describing the token, e.g.:
```json
{ "alg": "HS256", "typ": "JWT" }
```
This says: "this token is signed with HMAC-SHA256."

### Payload (claims)
A JSON object holding the actual data — e.g. user ID, expiry time:
```json
{ "user_id": 42, "exp": 1751932800 }
```

### Signature
The header and payload are base64-encoded and concatenated, then run through
an HMAC function using a **secret key only the server knows**:

```
signature = HMAC-SHA256(base64(header) + "." + base64(payload), secretKey)
```

The final token is:
```
base64(header) + "." + base64(payload) + "." + signature
```

### Critical point: base64 is NOT encryption

Anyone can decode the header and payload — paste a JWT into jwt.io and you'll see
the claims in plain text. **JWTs are not secret, they're tamper-evident.**

If someone edits the payload (say, changes `user_id` from 42 to 1), the signature
no longer matches. When the server recomputes the signature and compares it, the
mismatch reveals the tampering, and the server rejects the token.

This is the whole point: the client can *read* the token, but cannot *forge* or
*alter* it without knowing the secret key.

---

## Passwords: Hashing, Not Encryption

You **never** store a user's plain password in the database — not even "just for now,"
not even in a test project. If your database ever leaks, plain passwords leak with it.

**Hashing vs encryption:**
- Encryption is reversible (with the right key you get the original back).
- Hashing is one-way — you cannot get the original password back from the hash.

To check a login, you don't decrypt the stored value — you hash the *incoming*
password the same way and compare hashes.

### Why bcrypt specifically

Fast hash functions (like SHA-256) are actually *bad* for passwords — they're built
for speed, which means an attacker with a leaked hash database can try billions of
guesses per second.

`bcrypt` is deliberately slow, and has a configurable "cost" factor. This means
brute-forcing becomes computationally expensive even at scale. It also automatically
handles salting (adding random data per password) so two identical passwords never
produce the same hash.

---

## The Full Auth Flow (step by step, no code yet)

1. **Register**: user submits email + password.
2. Server hashes the password with bcrypt.
3. Server stores email + password hash in the database (never the raw password).
4. **Login**: user submits email + password again.
5. Server looks up the user by email, hashes the incoming password the same way,
   and compares it to the stored hash.
6. If it matches, server generates a JWT containing the user's ID and an expiry time,
   signs it with the secret key, and returns it to the client.
7. Client stores the token (e.g. in memory or local storage) and sends it in the
   `Authorization: Bearer <token>` header on every subsequent request.
8. **Middleware** intercepts every request to a protected route: it extracts the
   token, verifies the signature, checks the expiry, and if valid, extracts the
   user ID from the claims.
9. Middleware attaches the user ID to the request's `context.Context`.
10. The handler reads the user ID out of the context — it never has to parse the
    token itself.

---

## Context: How Middleware Talks to Handlers

Go's `context.Context` is a way to attach request-scoped values that travel with
the request through the chain of middleware and into the handler.

Middleware does roughly:
```go
ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
r = r.WithContext(ctx)
next.ServeHTTP(w, r)
```

The handler later does:
```go
userID := r.Context().Value(userIDKey).(int)
```

This is exactly the same context you'll have seen passed around in database calls —
same mechanism, different payload.

---

## Authentication vs Authorization

These sound similar but answer two different questions:

- **Authentication** — *Who are you?* (Is this a valid, logged-in user?)
- **Authorization** — *What are you allowed to do?* (Can this specific user delete
  this specific book?)

A request can be authenticated (valid token, known user) but still fail authorization
(that user doesn't own the resource they're trying to modify).

In our book API, this shows up as: "only the owner of a book can delete it" — that
check happens *after* authentication succeeds, inside the handler or service layer.

---

## What's Next

The code file walks through implementing each of these pieces in Go: `bcrypt` for
hashing, generating and validating JWTs, the auth middleware, and reading the user
out of context. The exercises file has small, focused drills for each piece before
you wire it all together.
