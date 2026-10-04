# Automatic Request Fields

The request-scoped loggers (`SlogFromContext`, `zerolog.Ctx`,
`logrusmw.FromContext`) require code to fetch the logger from the context. If
you'd rather keep logging with a logger you already have, and have
`requestId`, `method` and `path` added automatically, install the context
handler or hook for your logger.

In each case, a log call gets the request fields when it **passes the request
context** to the logging library. Go has no goroutine-local storage, so a log
call that doesn't receive the context can't learn which request it belongs to.
In return, the code doing the logging never imports this package.

| Logger | Install once | Log calls that get request fields |
|--------|--------------|-----------------------------------|
| slog | `slog.SetDefault(slog.New(logmiddleware.NewContextHandler(h)))` | `slog.InfoContext(ctx, …)`, `logger.ErrorContext(ctx, …)`, `logger.LogAttrs(ctx, …)` |
| logrus | `logger.AddHook(logrusmw.ContextHook{})` | `logger.WithContext(ctx).Info(…)` |
| zerolog | `logger = logger.Hook(zerologmw.ContextHook{})` | `logger.Info().Ctx(ctx).Msg(…)`, or a logger built with `With().Ctx(ctx)` |

Calls without a request context, such as `slog.Info(…)` or `logger.Info(…)`, or
calls with a context that didn't go through the middleware, are logged
unchanged. It is safe to install these globally.

## slog

```go
func main() {
    h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
    slog.SetDefault(slog.New(logmiddleware.NewContextHandler(h)))

    mw := logmiddleware.NewSlog(nil) // nil: use slog.Default()
    http.ListenAndServe(":8080", mw.Handler(mux))
}

// Elsewhere, with no import of log-middleware:
func (r *UserRepo) Get(ctx context.Context, id string) (*User, error) {
    slog.DebugContext(ctx, "querying user", "userId", id)
    …
}
```

Notes:

- **No duplicates:** Loggers from `SlogFromContext` already carry
  `requestId`. The handler sees that the field was bound with `With` and
  doesn't add it again, so you can use both styles in one program.
- **Groups:** slog adds handler-supplied attributes to the group that is open.
  After `logger.WithGroup("db")`, the fields appear as `db.requestId`, etc.
  OpenTelemetry's slog bridge works the same way.
- **Double wrapping:** `NewContextHandler` returns its argument unchanged if
  it is already a `*ContextHandler`.
- **Wrapping the current default:** Use
  `NewContextHandler(slog.Default().Handler())` and then call `SetDefault`.
  Passing `nil` panics, rather than wrapping a handler that would point back
  at itself.

## logrus

```go
logger := logrus.New()
logger.AddHook(logrusmw.ContextHook{})
mw := logrusmw.New(logger)

// Elsewhere:
logger.WithContext(ctx).WithField("userId", id).Info("loaded user")
```

The hook never overwrites a field that is already set on the entry. Entries
from `logrusmw.FromContext` already carry the request fields, so they are left
as they are.

To add the hook to the global logger, use `logrus.AddHook(logrusmw.ContextHook{})`.

## zerolog

```go
logger := zerolog.New(os.Stdout).Hook(zerologmw.ContextHook{})

// Elsewhere:
logger.Info().Ctx(ctx).Str("userId", id).Msg("loaded user")
```

**Watch for duplicates:** zerolog doesn't de-duplicate fields, and the
logger returned by `zerolog.Ctx(ctx)` already carries the request fields. If
that logger was derived from a hooked logger and you also call `.Ctx(ctx)` on
its events, the fields are written twice. For any one log call, use either
`zerolog.Ctx(ctx)` or `.Ctx(ctx)` on a hooked logger, not both.

## Contexts outside HTTP

Background jobs and tests can create a context that these handlers recognize:

```go
ctx = logmiddleware.ContextWithRequestInfo(ctx, logmiddleware.RequestInfo{RequestID: jobID})
```

`logmiddleware.RequestInfoFromContext(ctx)` reads it back.
