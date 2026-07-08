# Module 8 — Exercises

Each drill refactors one part of YOUR book API (from Modules 5–7). Do them in order — each builds on the last. No copy-pasting from the code file; type it and understand it.

## Drill 1 — Project layout
Create the folder structure:
```
cmd/api/
internal/books/
internal/database/
internal/config/
```
Move your current `main.go` into `cmd/api/main.go`. Fix the run command (`go run ./cmd/api`) and confirm the server still starts.

✅ Check: the project root contains no `.go` files.

## Drill 2 — Repository
Move ALL database logic into `internal/books/repository.go`: `GetAll`, `GetByID`, `Create`, `Update`, `Delete`.

✅ Check: grep your handlers for `SELECT`, `INSERT`, `Scan` — zero hits.

## Drill 3 — Service
Move business logic into `internal/books/service.go`: validation rules, any ID/ownership logic. Handlers call the service; the service calls the repository.

✅ Check: your handler for `POST /books` contains no validation `if` statements.

## Drill 4 — Config struct
Create `internal/config/config.go` with a `Config` struct holding `Port` and `DSN`. Read both from environment variables with sensible defaults. Delete every hardcoded port/DSN string from the rest of the codebase.

✅ Check: `PORT=9999 go run ./cmd/api` starts the server on :9999; plain `go run ./cmd/api` starts it on the default.

## Drill 5 — .env + godotenv
Add a `.env` file with `PORT` and `DB_DSN`. Load it with `godotenv.Load()` at the top of `main`. Add `.env` to `.gitignore`.

✅ Check: change `PORT` in `.env`, restart, server picks it up — no `export` needed.

## Mini project — full restructure
Finish the job: `main.go` does nothing but load config → open DB → wire repo/service/handler → register routes → listen. All auth code from Module 7 moves into `internal/auth/` following the same three-layer split.

✅ Check (all must pass):
1. `go run ./cmd/api` works and every endpoint from Module 7 still behaves the same (test with curl).
2. `main.go` is under ~60 lines.
3. No file mixes layers: nothing imports both `net/http` and `database/sql`.

## Stretch question (answer in one paragraph, no code)
A teammate creates `pkg/books` instead of `internal/books` "so it's reusable later". What can now go wrong that `internal/` would have prevented?
