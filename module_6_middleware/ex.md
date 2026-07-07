# Module 6 — Middleware (Exercises)

1. Write `withLogging` that prints "before"/"after" on every request. Confirm order in terminal.
2. Extend it with a `responseRecorder` so it also prints the status code.
3. Add duration to the log line: method, path, status, duration — one line per request.
4. Add a route that deliberately panics (`panic("boom")`). Write `withRecovery` — confirm the server keeps running and returns 500 instead of crashing.
5. Add `withCORS`. Hit your API from a browser `fetch()` call, see the CORS error, add the middleware, confirm it's fixed.
6. **Mini project:** chain `withRecovery`, `withLogging`, `withCORS` cleanly onto your book API's mux using a `chain` helper.
