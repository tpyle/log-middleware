# Go Log Middleware v3

HTTP middleware for Go that gives every request a unique ID, attaches a
request-scoped logger to the request context, and logs each response with its
status code and duration.

The core module depends only on the standard library and logs with
[`log/slog`](https://pkg.go.dev/log/slog). Adapters for
[zerolog](https://github.com/rs/zerolog) and
[logrus](https://github.com/sirupsen/logrus) are separate Go modules, so those
libraries only appear in your `go.mod`, build and SBOM if you use the adapter.

## Install

```bash
go get github.com/tpyle/log-middleware/v3

# only if you use them:
go get github.com/tpyle/log-middleware/zerologmw/v3
go get github.com/tpyle/log-middleware/logrusmw/v3
```

Requires Go 1.27+. v2 (zerolog only) and v1 (logrus only) remain available at
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
import "github.com/tpyle/log-middleware/zerologmw/v3"

logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel)
mw := zerologmw.New(&logger)
// in handlers: zerolog.Ctx(r.Context()).Info().Msg("hello")
```

### logrus

```go
import "github.com/tpyle/log-middleware/logrusmw/v3"

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

The repository contains four Go modules: the core (repository root),
`zerologmw`, `logrusmw` and `examples`. Each nested module uses `replace`
directives to build against the local code, so changes can be made and tested
across modules together. Run checks in each module:

```bash
for m in . zerologmw logrusmw examples; do
  (cd $m && go mod tidy -diff && go vet ./... && go test -race -cover ./... && golangci-lint run ./...)
done
```

### Releasing

The core and both adapters are always released together with the same
version number. Go versions each module by its own tags, so a release is three
tags on the same commit:

1. Set the new version in the `require` lines for this repository's modules in
   `zerologmw/go.mod`, `logrusmw/go.mod` and `examples/go.mod`, and merge.
2. Tag the merge commit and push the tags:
   ```bash
   V=vX.Y.Z
   git tag -a $V -m $V && git tag -a zerologmw/$V -m $V && git tag -a logrusmw/$V -m $V
   git push origin $V zerologmw/$V logrusmw/$V
   ```
3. Check from a scratch module that
   `go get github.com/tpyle/log-middleware/zerologmw/v3@$V` resolves.

## License

MIT. See [LICENSE](LICENSE).
