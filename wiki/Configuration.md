# Configuration and Behavior

## Options

All constructors (`New`, `NewSlog`, `zerologmw.New`, `logrusmw.New`) accept
`logmiddleware.Option` values:

| Option | Effect |
|--------|--------|
| `WithRequestIDGenerator(func() string)` | Replaces the default random UUID (v4) generator. The function must be safe for concurrent use. |
| `WithRequestIDFromRequest(RequestIDGenerator)` | Replaces the default generator with one that receives the incoming `*http.Request`. Must be safe for concurrent use. |
| `WithClock(func() time.Time)` | Replaces `time.Now` for measuring durations. Mainly for tests. |

Passing `nil` to any option leaves that setting unchanged: the default, or
whatever an earlier option set. If more than one request ID
option is given, the last one wins.

## Custom request IDs

`RequestIDGenerator` is a one-method interface:

```go
type RequestIDGenerator interface {
    GenerateRequestID(r *http.Request) string
}
```

`RequestIDGeneratorFunc` adapts a plain function, the same way
`http.HandlerFunc` adapts a function to `http.Handler`. The default generator
is exported as `UUIDRequestID`, so custom generators can fall back to it.

### Reusing an incoming header

`RequestIDFromHeader` covers the most common case: reusing an ID set by an
upstream proxy or caller.

```go
ids := logmiddleware.RequestIDFromHeader(logmiddleware.RequestIDHeader,
    logmiddleware.RequestIDGeneratorFunc(logmiddleware.UUIDRequestID))
mw := logmiddleware.NewSlog(logger, logmiddleware.WithRequestIDFromRequest(ids))
```

It uses the header's first value if it passes `ValidRequestID`, and calls
the fallback otherwise. The fallback is required, and passing `nil` panics:
you choose what happens when there is no usable ID. Use any header name, such
as `X-Correlation-Id`.

Only reuse a header that your own infrastructure controls. If clients can
reach the server directly, or your proxy passes the header through unchanged,
any client can pick its own request ID, including one already used by another
request, which muddles log correlation. Configure the proxy to strip or
overwrite the header on incoming requests. For example, in nginx:
`proxy_set_header X-Request-Id $request_id;`.

IDs reused from a header are not guaranteed to be unique.

### Validation

The ID is written to every log line for the request and echoed back in the
`X-Request-Id` response header. An unchecked header lets clients inject
arbitrary text into your logs, so validate anything taken from the request.
`ValidRequestID(id)` accepts IDs that:

- are 1 to `MaxRequestIDLength` (128) bytes long, and
- contain only ASCII letters, digits and `- _ . : + / =`.

This allows UUIDs, ULIDs, base64, W3C `traceparent` values and AWS trace IDs.
It rejects whitespace, control characters, quotes, backslashes and non-ASCII
text. If you need different rules, write your own `RequestIDGenerator`.

The middleware does not validate or replace what a generator returns. It
uses the value as-is, including an empty string. See `examples/requestid`
for a runnable server.

## Behavior details

- **Request ID:** The middleware generates a new ID for every request. Any
  ID already in the incoming context is ignored, and so is an `X-Request-Id`
  header on the request unless a custom generator reads it (see
  [Custom request IDs](#custom-request-ids)). The ID is set on the response with `Header().Set`
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
