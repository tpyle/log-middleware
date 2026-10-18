# Go Log Middleware v3

HTTP middleware for Go that gives every request an ID, attaches a
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
IDs are random UUIDs by default. To reuse a valid incoming `X-Request-Id` set
by a proxy you control (the proxy must strip or overwrite any value sent by
clients, or clients can choose their own IDs):

```go
ids := logmiddleware.RequestIDFromHeader(logmiddleware.RequestIDHeader,
    logmiddleware.RequestIDGeneratorFunc(logmiddleware.UUIDRequestID))
mw := logmiddleware.NewSlog(logger, logmiddleware.WithRequestIDFromRequest(ids))
```

`WithRequestIDFromRequest` accepts any `RequestIDGenerator`. See
[wiki/Configuration.md](wiki/Configuration.md#custom-request-ids).

## Output

The middleware logs one line per request, at **debug** level:

```json
{"level":"DEBUG","msg":"HTTP response sent","requestId":"5616…","method":"GET","path":"/hello","statusCode":200,"duration":68979}
```

## Documentation

- [wiki/](wiki/Home.md): guides for each logger, configuration, and migration
  from v1 and v2 ([Migrating to v3](wiki/Migrating-to-v3.md))
- [examples/](examples/): runnable servers for slog, zerolog, logrus, a
  custom logger and custom request IDs (`go run ./examples/slog`)
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

Go versions each module by its own tags, so each module is released only when
its code changes. The adapters' version numbers can therefore lag behind the
core's. An adapter's `require` of the core is a minimum, so users can always
upgrade the core on its own. The `examples` module is never tagged.

1. Find which modules changed since their last release, for example
   `git diff v3.1.0 HEAD -- zerologmw`. The core is the repository root,
   excluding the adapter and examples directories.
2. If an adapter needs a newer core than its `require` line names (for
   example, because it uses a new core API), release the core first, then set
   the new core version in that adapter's `go.mod` and merge.
3. Tag the merge commit for each changed module and push the tags. Adapter
   tags are prefixed with their directory:
   ```bash
   git tag -a vX.Y.Z -m vX.Y.Z                      # core
   git tag -a zerologmw/vX.Y.Z -m zerologmw/vX.Y.Z  # only if zerologmw changed
   git tag -a logrusmw/vX.Y.Z -m logrusmw/vX.Y.Z    # only if logrusmw changed
   git push origin <tags>
   ```
4. Check from a scratch module that each new version resolves, for example
   `go list -m github.com/tpyle/log-middleware/v3@vX.Y.Z`.

## License

MIT. See [LICENSE](LICENSE).
