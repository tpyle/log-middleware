# Migrating to v3

v3 moves logger support out of the core package so that it no longer requires
zerolog. The core package now uses `log/slog`, and zerolog moved to an adapter
subpackage.

## From v2 (zerolog)

```diff
-import logmiddleware "github.com/tpyle/log-middleware/v2"
+import logmiddleware "github.com/tpyle/log-middleware/v3"
+import "github.com/tpyle/log-middleware/v3/zerologmw"

-mw := logmiddleware.NewLogMiddleware(&logger)
+mw := zerologmw.New(&logger)
```

Nothing else changes for handlers. `zerolog.Ctx(r.Context())`,
`GetRequestIdFromRequest` and `GetRequestIdFromContext` work as before.

### Behavior changes

- If a handler never called `WriteHeader`, v2 logged `statusCode` as `0`.
  v3 logs `200`, the status net/http actually sends.
- The `X-Request-Id` header is now set with `Set` instead of `Add`.
- The wrapped `ResponseWriter` now supports `http.Flusher` and
  `http.ResponseController`.
- The minimum Go version is now 1.26.

## From v1 (logrus)

v1 used the global logrus logger and installed a global hook. v3 takes the
logger explicitly:

```diff
-import logmiddleware "github.com/tpyle/log-middleware"
+import "github.com/tpyle/log-middleware/v3/logrusmw"

-http.Handle("/", logmiddleware.LogMiddleware(handler))
+mw := logrusmw.New(logrus.StandardLogger())
+http.Handle("/", mw.Handler(handler))
```

v1 installed a hook on the global logrus logger in `init()`, so
`logrus.WithContext(ctx)` calls got the request ID. v3 doesn't change global
state on import. To keep those calls working as they are, add the hook
yourself:

```go
logrus.AddHook(logrusmw.ContextHook{})
```

The hook now also adds `method` and `path`. Alternatively, switch handlers to
the request-scoped entry:

```diff
-logrus.WithContext(r.Context()).Info("processing")
+logrusmw.FromContext(r.Context()).Info("processing")
```

v3 no longer logs a separate "HTTP request received" line. It logs only the
"HTTP response sent" line.

## Switching to slog

```go
mw := logmiddleware.NewSlog(slogLogger)
// in handlers:
logmiddleware.SlogFromContext(r.Context()).Info("processing")
```
