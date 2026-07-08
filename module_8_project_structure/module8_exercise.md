One exercise, teaches the whole thing.

---

**Build a Notes API from scratch with proper structure**

Not refactoring — start fresh. Create a new folder, run `go mod init`, build this from zero.

**What it does:**
- `POST /notes` — create a note (just a string of text)
- `GET /notes/{id}` — get a note by ID
- `DELETE /notes/{id}` — delete a note

**The rules (this is what you're actually learning):**

Store notes in memory — a `map[int]string`, no database. That's intentional — it removes SQL complexity so you focus purely on structure.

Your project must look like this when done:

```
notes-api/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   └── notes/
│       ├── handler.go
│       ├── service.go
│       └── store.go
├── .env
├── .gitignore
└── go.mod
```

**What goes where — strictly enforced:**

`store.go` — owns the map and a mutex. Has `Save`, `Get`, `Delete` methods. Nothing else touches the map.

`service.go` — owns one rule: a note cannot be empty. Has `Create`, `Get`, `Delete` methods. Calls the store. Never touches `http` anything.

`handler.go` — owns JSON encoding, status codes, reading the request body. Calls the service. Never touches the map directly.

`main.go` — creates the store, creates the service with the store, creates the handler with the service, registers routes, reads `PORT` from `.env` via godotenv, starts the server. Nothing else.

**When you're done, run these checks:**

```bash
# 1. Create a note
curl -X POST localhost:4000/notes \
  -H "Content-Type: application/json" \
  -d '{"text":"buy milk"}' 
# expect: 201, {"id":1}

# 2. Get it back
curl localhost:4000/notes/1
# expect: 200, {"id":1,"text":"buy milk"}

# 3. Delete it
curl -X DELETE localhost:4000/notes/1
# expect: 204

# 4. Get deleted note
curl localhost:4000/notes/1
# expect: 404

# 5. Empty note
curl -X POST localhost:4000/notes \
  -H "Content-Type: application/json" \
  -d '{"text":""}'
# expect: 400

# 6. Layer check — service must not know about HTTP
grep -r "net/http" internal/notes/service.go
# expect: no output

# 7. Map must live in one place only
grep -r "map\[int\]" internal/
# expect: only store.go
```

All 7 checks pass = you understand the structure. Any one fails = something is in the wrong layer.

Start with `store.go` first, then `service.go`, then `handler.go`, then `main.go`. That order matters — each layer only knows what's below it.

Come back when you're done or when you're stuck on a specific part.