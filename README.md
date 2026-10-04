# Go Log Middleware v3

HTTP middleware for Go that gives every request a unique ID, attaches a
request-scoped logger to the request context, and logs each response with its
status code and duration.

The core package depends only on the standard library and logs with
[`log/slog`](https://pkg.go.dev/log/slog). Adapters for
[zerolog](https://github.com/rs/zerolog) and
[logrus](https://github.com/sirupsen/logrus) live in subpackages, so those
libraries are only compiled into your program if you import the adapter.

## Install

```bash
go get github.com/tpyle/log-middleware/v3
```

Requires Go 1.26+. v2 (zerolog only) and v1 (logrus only) remain available at
`github.com/tpyle/log-middleware/v2` and `github.com/tpyle/log-middleware`.

## Usage

### slog

```go
import logmiddleware "github.com/tpyle/log-middleware/v3"

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
mw := logmiddleware.NewSlog(logger)

mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
    // Carries requestId, method and path automatically.
    logmiddleware.SlogFromContext(r.Context()).Info("hello")
})
http.ListenAndServe(":8080", mw.Handler(mux))
```

### zerolog

```go
import "github.com/tpyle/log-middleware/v3/zerologmw"

logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel)
mw := zerologmw.New(&logger)
// in handlers: zerolog.Ctx(r.Context()).Info().Msg("hello")
```

### logrus

```go
import "github.com/tpyle/log-middleware/v3/logrusmw"

logger := logrus.New()
logger.SetLevel(logrus.DebugLevel)
mw := logrusmw.New(logger)
// in handlers: logrusmw.FromContext(r.Context()).Info("hello")
```

### Request fields without the request-scoped logger

To add `requestId`, `method` and `path` to log calls that pass the request
context, install a context handler or hook once:

```go
slog.SetDefault(slog.New(logmiddleware.NewContextHandler(handler)))   // slog.InfoContext(ctx, ...)
logrusLogger.AddHook(logrusmw.ContextHook{})                          // logger.WithContext(ctx).Info(...)
zerologLogger = zerologLogger.Hook(zerologmw.ContextHook{})           // logger.Info().Ctx(ctx).Msg(...)
```

Code that logs this way doesn't need to import this package. See
[wiki/Automatic-Request-Fields.md](wiki/Automatic-Request-Fields.md).

### Any other logger

Implement `logmiddleware.Logger` and pass it to `logmiddleware.New`. See
[wiki/Custom-Loggers.md](wiki/Custom-Loggers.md).

### Request IDs

```go
id := logmiddleware.GetRequestIdFromRequest(r)   // or GetRequestIdFromContext(ctx)
```

The ID is also returned to the client in the `X-Request-Id` response header.

## Output

The middleware logs one line per request, at **debug** level:

```json
{"level":"DEBUG","msg":"HTTP response sent","requestId":"5616…","method":"GET","path":"/hello","statusCode":200,"duration":68979}
```

## Documentation

- [wiki/](wiki/Home.md): guides for each logger, configuration, and migration
  from v1 and v2 ([Migrating to v3](wiki/Migrating-to-v3.md))
- [examples/](examples/): runnable servers for slog, zerolog, logrus and a
  custom logger (`go run ./examples/slog`)
- API reference: <https://pkg.go.dev/github.com/tpyle/log-middleware/v3>

## Development

```bash
go test -race -cover ./...
go test -bench=. ./...
golangci-lint run ./...
```

## License

MIT. See [LICENSE](LICENSE).
