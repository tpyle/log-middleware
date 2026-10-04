# Using zerolog

```go
import (
    "github.com/rs/zerolog"
    "github.com/tpyle/log-middleware/v3/zerologmw"
)

logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).With().Timestamp().Logger()
mw := zerologmw.New(&logger)
```

If you pass `nil`, the middleware uses the global `log.Logger` from
`github.com/rs/zerolog/log`. It looks the global up on every request.

## In handlers

The request-scoped logger is attached with zerolog's own `WithContext`, so use
`zerolog.Ctx`:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    zerolog.Ctx(r.Context()).Info().Str("userId", id).Msg("loaded user")
}
```

To add request fields to events from other loggers, such as one held by a
dependency, use `zerologmw.ContextHook` with `.Ctx(ctx)`. See
[Automatic Request Fields](Automatic-Request-Fields.md).

## Output fields

| Field | Type |
|-------|------|
| `requestId`, `method`, `path` | string |
| `statusCode` | int |
| `duration` | number, in `zerolog.DurationFieldUnit` (milliseconds by default) |

## Using the adapter directly

`zerologmw.NewLogger(&logger)` returns the adapter itself (a
`logmiddleware.Logger`). This is useful when you also want to pass options:

```go
mw := logmiddleware.New(zerologmw.NewLogger(&logger), logmiddleware.WithRequestIDGenerator(myIDs))
// equivalent to:
mw := zerologmw.New(&logger, logmiddleware.WithRequestIDGenerator(myIDs))
```
