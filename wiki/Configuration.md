# Configuration and Behavior

## Options

All constructors (`New`, `NewSlog`, `zerologmw.New`, `logrusmw.New`) accept
`logmiddleware.Option` values:

| Option | Effect |
|--------|--------|
| `WithRequestIDGenerator(func() string)` | Replaces the default random UUID (v4) generator. The function must be safe for concurrent use. |
| `WithClock(func() time.Time)` | Replaces `time.Now` for measuring durations. Mainly for tests. |

Passing `nil` to either option keeps the default.

## Behavior details

- **Request ID:** The middleware generates a new ID for every request. Any
  ID already in the incoming context, or an `X-Request-Id` header on the
  request, is ignored. The ID is set on the response with `Header().Set`
  before your handler runs, so your handler can override it.
- **Status code:** The first status of 200 or higher passed to `WriteHeader`
  is recorded. Informational 1xx responses, such as 103 Early Hints, are
  skipped. If the handler writes a body or flushes without calling
  `WriteHeader`, or writes nothing at all, the status is 200, which is what
  net/http sends.
- **Duration:** The time from when the middleware receives the request until
  the wrapped handler returns.
- **Panics:** If the handler panics, the panic propagates and no response log
  is written. Put a recovery middleware *inside* this one if you want those
  requests logged.
- **Streaming:** The wrapped `ResponseWriter` implements `http.Flusher` and
  `Unwrap`, so `w.(http.Flusher)` and `http.NewResponseController(w)` keep
  working.
- **Log level:** Response logs are written at debug level.
