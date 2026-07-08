# Module 7 — Authentication & Authorization (Code)

This walks through implementing each piece from the theory file. Assumes a book API
with a `users` table and a JWT library — we'll use `github.com/golang-jwt/jwt/v5` and
`golang.org/x/crypto/bcrypt`, the two standard choices in the Go ecosystem.

```
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto/bcrypt
```

---

## 1. Password Hashing

```go
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword hashes a plaintext password for storage.
func HashPassword(password string) (string, error) {
    // bcrypt.DefaultCost is 10 — a good balance of security vs speed.
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return "", err
    }
    return string(hash), nil
}

// CheckPassword compares a plaintext password against a stored bcrypt hash.
// Returns nil if they match, an error otherwise.
func CheckPassword(hash, password string) error {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
```

Notice `CheckPassword` never "decrypts" anything — it hashes the incoming password
internally and lets bcrypt compare hashes safely (in constant time, to avoid timing
attacks).

---

## 2. Generating a JWT

```go
package auth

import (
    "time"

    "github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("replace-this-with-an-env-var-in-real-life")

type Claims struct {
    UserID int `json:"user_id"`
    jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT for the given user ID, valid for 24 hours.
func GenerateToken(userID int) (string, error) {
    claims := Claims{
        UserID: userID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(jwtSecret)
}
```

`jwt.RegisteredClaims` gives you standard fields like `exp` (expiry) for free —
we're embedding it into our own `Claims` struct alongside `UserID`.

---

## 3. Validating a JWT

```go
// ValidateToken parses and validates a JWT string, returning the claims if valid.
func ValidateToken(tokenString string) (*Claims, error) {
    claims := &Claims{}

    token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
        return jwtSecret, nil
    })
    if err != nil {
        return nil, err // covers expired, malformed, or tampered tokens
    }

    if !token.Valid {
        return nil, jwt.ErrTokenInvalidClaims
    }

    return claims, nil
}
```

`jwt.ParseWithClaims` handles the signature check internally — if the token was
tampered with, or the secret doesn't match, `err` will be non-nil. Expiry is checked
automatically too.

---

## 4. Register and Login Handlers

```go
func (app *application) registerHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Email    string `json:"email"`
        Password string `json:"password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        app.clientError(w, http.StatusBadRequest)
        return
    }

    hash, err := auth.HashPassword(input.Password)
    if err != nil {
        app.serverError(w, err)
        return
    }

    id, err := app.users.Create(input.Email, hash)
    if err != nil {
        app.serverError(w, err)
        return
    }

    writeJSON(w, http.StatusCreated, map[string]int{"id": id})
}

func (app *application) loginHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Email    string `json:"email"`
        Password string `json:"password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        app.clientError(w, http.StatusBadRequest)
        return
    }

    user, err := app.users.GetByEmail(input.Email)
    if err != nil {
        app.clientError(w, http.StatusUnauthorized) // don't reveal "email not found"
        return
    }

    if err := auth.CheckPassword(user.PasswordHash, input.Password); err != nil {
        app.clientError(w, http.StatusUnauthorized)
        return
    }

    token, err := auth.GenerateToken(user.ID)
    if err != nil {
        app.serverError(w, err)
        return
    }

    writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
```

Notice both the "email not found" and "wrong password" cases return the exact same
401 — deliberately. If you're more specific, you leak which emails are registered.

---

## 5. Auth Middleware

```go
type contextKey string

const userIDKey contextKey = "userID"

func (app *application) requireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            app.clientError(w, http.StatusUnauthorized)
            return
        }

        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) != 2 || parts[0] != "Bearer" {
            app.clientError(w, http.StatusUnauthorized)
            return
        }

        claims, err := auth.ValidateToken(parts[1])
        if err != nil {
            app.clientError(w, http.StatusUnauthorized)
            return
        }

        ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

This slots into the same middleware chain from Module 6 — it's just another
`func(http.Handler) http.Handler`, nothing new about its shape.

---

## 6. Reading the User in a Handler

```go
func (app *application) deleteBookHandler(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value(userIDKey).(int)

    bookID, err := strconv.Atoi(r.PathValue("id"))
    if err != nil {
        app.clientError(w, http.StatusBadRequest)
        return
    }

    book, err := app.books.GetByID(bookID)
    if err != nil {
        app.notFound(w)
        return
    }

    // Authorization check — separate from authentication.
    if book.OwnerID != userID {
        app.clientError(w, http.StatusForbidden)
        return
    }

    if err := app.books.Delete(bookID); err != nil {
        app.serverError(w, err)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}
```

Note the two distinct failure modes:
- No token / invalid token → middleware returns `401 Unauthorized` before the
  handler even runs.
- Valid token, but wrong user → handler returns `403 Forbidden`. This is
  authorization, not authentication.

---

## 7. Wiring It Into Routes

```go
mux.Handle("POST /register", http.HandlerFunc(app.registerHandler))
mux.Handle("POST /login", http.HandlerFunc(app.loginHandler))

mux.Handle("DELETE /books/{id}", app.requireAuth(http.HandlerFunc(app.deleteBookHandler)))
```

Public routes stay unwrapped. Protected routes get wrapped with `requireAuth`,
exactly like the logging/recovery/CORS middleware from Module 6.
