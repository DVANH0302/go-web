# Module 7 — Authentication & Authorization (Exercises)

Small, focused drills. Do them in order — each one builds on the last. No mini
project bundling yet — this is auth only.

---

### Drill 1 — Password hashing

Write `HashPassword` and `CheckPassword` functions using `bcrypt`.

- Hash the string `"correct-horse-battery-staple"`.
- Print the resulting hash — notice it's different every time you run it, even
  with the same input. Why?
- Call `CheckPassword` with the correct password → confirm it returns `nil`.
- Call `CheckPassword` with a wrong password → confirm it returns an error.

---

### Drill 2 — Generate a JWT

Write `GenerateToken(userID int) (string, error)` that creates a signed JWT
containing the user ID and a 24-hour expiry.

- Generate a token for user ID `42`.
- Print the token string.
- Paste it into [jwt.io](https://jwt.io) and confirm you can read the `user_id`
  and `exp` claims in the decoded payload — even though you never gave jwt.io
  your secret key. This is the "readable but not forgeable" property in action.

---

### Drill 3 — Validate a JWT

Write `ValidateToken(tokenString string) (*Claims, error)`.

Test it against three inputs:
1. A valid, freshly generated token → should succeed and return the correct
   `user_id`.
2. A manually expired token (hack: generate one with `ExpiresAt` set to a time
   in the past) → should return an error.
3. A tampered token — take a valid token string and change one character in the
   middle (payload section) → should return an error (signature mismatch).

---

### Drill 4 — Auth middleware, single route

Add `requireAuth` middleware to **one** route only — pick something simple like
`GET /profile`.

- Hit the route with no `Authorization` header → confirm you get `401`.
- Hit it with `Authorization: Bearer garbage` → confirm `401`.
- Hit it with a valid token (from Drill 2) → confirm `200`.

---

### Drill 5 — Read the user from context

Inside the handler for the route from Drill 4, read the user ID out of
`r.Context()` and return it in the JSON response body, e.g.:

```json
{ "user_id": 42 }
```

Confirm the ID in the response matches the ID you put into the token.

---

### Drill 6 — Full register → login → protected route flow (curl)

Wire up `POST /register` and `POST /login` for real, backed by your `users`
table. Then, using `curl`:

```bash
# 1. Register
curl -X POST localhost:8080/register \
  -d '{"email":"you@example.com","password":"hunter2"}'

# 2. Login — copy the token from the response
curl -X POST localhost:8080/login \
  -d '{"email":"you@example.com","password":"hunter2"}'

# 3. Hit the protected route with the token
curl localhost:8080/profile \
  -H "Authorization: Bearer <paste-token-here>"
```

Confirm each step works, and that step 3 fails without the header.

---

### Drill 7 — Authorization check (ownership)

Add an `owner_id` concept to one resource (reuse `books` if you have it from
earlier modules, or a dummy table).

- Create the resource as user A.
- Log in as user B, try to delete user A's resource → confirm you get `403`,
  **not** `401` (the token is valid — the user just isn't authorized).
- Log in as user A, delete the same resource → confirm it succeeds.

---

## Done?

Once all seven drills pass, you've built every piece the code file walks
through: hashing, token generation, validation, middleware, context passing,
and the authentication/authorization distinction. That's Module 7.
