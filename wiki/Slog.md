# Using slog

slog support is built into the core package and adds no dependencies.

```go
mw := logmiddleware.NewSlog(logger)       // logger is a *slog.Logger
```

If you pass `nil`, the middleware uses `slog.Default()`. It looks the default
up on every request, so a later `slog.SetDefault` call takes effect.

## In handlers

```go
func handler(w http.ResponseWriter, r *http.Request) {
    log := logmiddleware.SlogFromContext(r.Context())
    log.Info("loaded user", "userId", id)   // also carries requestId, method, path
}
```

`SlogFromContext` never returns nil. If the context has no logger, it returns
`slog.Default()`. To put a logger in a context yourself, for example in tests,
use `logmiddleware.ContextWithSlog`.

## Without SlogFromContext

Wrap your handler with `logmiddleware.NewContextHandler`. Any slog call made
with the request context, such as `slog.InfoContext(ctx, …)`, then gets the
request fields, even from code that doesn't import this package. See
[Automatic Request Fields](Automatic-Request-Fields.md).

## Context-aware handlers

The response log is written with `Logger.LogAttrs(ctx, ...)` using the request
context. A `slog.Handler` that reads values from the context, such as trace IDs,
therefore sees the request's context.

## Output fields

| Field | Type in JSON output |
|-------|------------------|
| `requestId`, `method`, `path` | string |
| `statusCode` | int |
| `duration` | int nanoseconds (`slog.Duration`); the text handler prints e.g. `1.5ms` |

## Already using zerolog or logrus?

If your application already uses zerolog or logrus, use the
[zerolog](Zerolog.md) or [logrus](Logrus.md) adapter rather than bridging
through a slog handler. The adapters put the native logger in the context, so
`zerolog.Ctx` and friends keep working.
