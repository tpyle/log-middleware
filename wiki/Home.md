# log-middleware

`github.com/tpyle/log-middleware/v3` is HTTP middleware for Go that:

- assigns every request a unique ID (a random UUID by default),
- returns that ID in the `X-Request-Id` response header,
- stores the ID and a request-scoped logger in the request context, and
- logs every completed response at debug level with its status code and duration.

The core package depends only on the standard library and logs with `log/slog`.
zerolog and logrus are supported through adapter subpackages. Their dependencies
are compiled into your program only if you import the adapter.

| Logger | Import | Constructor | Get the logger in a handler |
|--------|--------|-------------|-----------------------------|
| `log/slog` | `github.com/tpyle/log-middleware/v3` | `logmiddleware.NewSlog(l)` | `logmiddleware.SlogFromContext(ctx)` |
| zerolog | `github.com/tpyle/log-middleware/v3/zerologmw` | `zerologmw.New(&l)` | `zerolog.Ctx(ctx)` |
| logrus | `github.com/tpyle/log-middleware/v3/logrusmw` | `logrusmw.New(l)` | `logrusmw.FromContext(ctx)` |
| anything else | `github.com/tpyle/log-middleware/v3` | `logmiddleware.New(yourLogger)` | your choice |

## Pages

- [Getting Started](Getting-Started.md)
- [Using slog](Slog.md)
- [Using zerolog](Zerolog.md)
- [Using logrus](Logrus.md)
- [Automatic Request Fields](Automatic-Request-Fields.md)
- [Writing a Custom Logger Adapter](Custom-Loggers.md)
- [Configuration and Behavior](Configuration.md)
- [Migrating to v3](Migrating-to-v3.md)

To add request fields to log calls that use your own logger instead of the
request-scoped one, see [Automatic Request Fields](Automatic-Request-Fields.md).
For example, `slog.InfoContext(ctx, …)` can get them without importing this
package.

Runnable programs for each logger are in the repository's `examples/` directory.
