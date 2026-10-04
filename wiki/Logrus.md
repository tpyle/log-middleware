# Using logrus

```bash
go get github.com/tpyle/log-middleware/v3/logrusmw
```

```go
import (
    "github.com/sirupsen/logrus"
    "github.com/tpyle/log-middleware/v3/logrusmw"
)

logger := logrus.New()
logger.SetLevel(logrus.DebugLevel)
mw := logrusmw.New(logger)
```

`New` accepts any `logrus.FieldLogger`. You can pass a `*logrus.Logger`, or a
`*logrus.Entry` with preset fields (for example
`logger.WithField("service", "api")`). If you pass `nil`, the middleware uses
`logrus.StandardLogger()`.

## In handlers

logrus has no context-logger API of its own, so the adapter provides one:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    logrusmw.FromContext(r.Context()).WithField("userId", id).Info("loaded user")
}
```

`FromContext` never returns nil. If the context has no entry, it returns a new
entry on the standard logger. To put an entry in a context yourself, use
`logrusmw.ContextWithEntry`.

The entries are bound to the request context (`Entry.WithContext`), so logrus
hooks can read context values.

To add request fields to any entry logged with `WithContext(ctx)`, without
calling `FromContext`, add `logrusmw.ContextHook`. See
[Automatic Request Fields](Automatic-Request-Fields.md).

## Output fields

| Field | Type |
|-------|------|
| `requestId`, `method`, `path` | string |
| `statusCode` | int |
| `duration` | `time.Duration` (the text formatter prints `1.5ms`; JSON prints int nanoseconds) |
