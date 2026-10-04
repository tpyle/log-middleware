// Package zerologmw adapts [github.com/rs/zerolog] to logmiddleware.
//
// Each request gets a child logger carrying the requestId, method and path
// fields, attached to the request context with [zerolog.Logger.WithContext],
// so handlers retrieve it with [zerolog.Ctx]. Responses are logged at debug
// level.
package zerologmw

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	logmiddleware "github.com/tpyle/log-middleware/v3"
)

// Logger is a [logmiddleware.Logger] backed by zerolog.
type Logger struct {
	logger *zerolog.Logger
}

var _ logmiddleware.Logger = (*Logger)(nil)

// NewLogger returns a [logmiddleware.Logger] backed by l. If l is nil, the
// global [log.Logger] is used, looked up on every request.
func NewLogger(l *zerolog.Logger) *Logger {
	return &Logger{logger: l}
}

// New is shorthand for logmiddleware.New(NewLogger(l), opts...).
func New(l *zerolog.Logger, opts ...logmiddleware.Option) *logmiddleware.LogMiddleware {
	return logmiddleware.New(NewLogger(l), opts...)
}

// Start implements [logmiddleware.Logger].
func (z *Logger) Start(ctx context.Context, req logmiddleware.RequestInfo) (context.Context, logmiddleware.RequestLogger) {
	base := z.logger
	if base == nil {
		base = &log.Logger
	}
	l := base.With().
		Str(logmiddleware.FieldRequestID, req.RequestID).
		Str(logmiddleware.FieldMethod, req.Method).
		Str(logmiddleware.FieldPath, req.Path).
		Logger()
	return l.WithContext(ctx), &requestLogger{logger: l}
}

type requestLogger struct {
	logger zerolog.Logger
}

func (r *requestLogger) Finish(resp logmiddleware.ResponseInfo) {
	r.logger.Debug().
		Int(logmiddleware.FieldStatusCode, resp.StatusCode).
		Dur(logmiddleware.FieldDuration, resp.Duration).
		Msg(logmiddleware.ResponseMessage)
}

// ContextHook is a [zerolog.Hook] that adds the requestId, method and path of
// the current request to events logged with a request context, e.g.
//
//	logger = logger.Hook(zerologmw.ContextHook{})
//	logger.Info().Ctx(ctx).Msg("loaded user")
//
// It lets code log with a shared logger and the plain zerolog API and still
// get request fields. Events without a request context are unchanged.
//
// zerolog does not de-duplicate fields, and loggers returned by [zerolog.Ctx]
// for a request already carry these fields. Use either zerolog.Ctx(ctx) or
// .Ctx(ctx) on a hooked logger for a given log call, not both.
type ContextHook struct{}

var _ zerolog.Hook = ContextHook{}

// Run implements [zerolog.Hook].
func (ContextHook) Run(e *zerolog.Event, _ zerolog.Level, _ string) {
	info, ok := logmiddleware.RequestInfoFromContext(e.GetCtx())
	if !ok {
		return
	}
	e.Str(logmiddleware.FieldRequestID, info.RequestID).
		Str(logmiddleware.FieldMethod, info.Method).
		Str(logmiddleware.FieldPath, info.Path)
}
