# Writing a Custom Logger Adapter

The middleware calls your logger through two small interfaces:

```go
type Logger interface {
    Start(ctx context.Context, req RequestInfo) (context.Context, RequestLogger)
}

type RequestLogger interface {
    Finish(resp ResponseInfo)
}
```

- `Start` runs once per request, before your handler. `req` contains
  `RequestID`, `Method` and `Path`. Return `ctx` extended with whatever
  request-scoped logger your library uses, plus a `RequestLogger` for that
  request. The context you return is the one the handler receives.
- `Finish` runs once after the handler returns. `resp` contains `StatusCode`
  and `Duration`. It is **not** called if the handler panics.
- `Logger` must be safe for concurrent use. A `RequestLogger` is only used by
  one request.

To keep output consistent with the bundled adapters, use the exported
`Field*` constants and `ResponseMessage`.

## Example

```go
type accessLogger struct{ out *log.Logger }

func (a *accessLogger) Start(ctx context.Context, req logmiddleware.RequestInfo) (context.Context, logmiddleware.RequestLogger) {
    return ctx, &accessRequest{out: a.out, req: req}
}

type accessRequest struct {
    out *log.Logger
    req logmiddleware.RequestInfo
}

func (a *accessRequest) Finish(resp logmiddleware.ResponseInfo) {
    a.out.Printf("%s %s %s -> %d in %s", a.req.RequestID, a.req.Method, a.req.Path, resp.StatusCode, resp.Duration)
}

mw := logmiddleware.New(&accessLogger{out: log.Default()})
```

See `examples/custom` for a runnable version, and the `zerologmw` and
`logrusmw` packages for adapters that attach a logger to the context.

## Reading request info from a context

Before calling `Start`, the middleware stores the `RequestInfo` in the
context. Hooks or handlers that only see a context can read it back with
`logmiddleware.RequestInfoFromContext(ctx)`. This is how
`logmiddleware.ContextHandler` and the `ContextHook` types work.

## Testing code that uses the middleware

Because the logger is injected, tests can pass a fake `Logger` that records the
`RequestInfo` and `ResponseInfo` it receives. Combine it with
`WithRequestIDGenerator` and `WithClock` for deterministic assertions. To test
code that logs with a request context without running the middleware, build
the context with `logmiddleware.ContextWithRequestInfo`.
