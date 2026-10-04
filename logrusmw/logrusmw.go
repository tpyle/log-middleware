// Package logrusmw adapts [github.com/sirupsen/logrus] to logmiddleware.
//
// Each request gets a [*logrus.Entry] carrying the requestId, method and path
// fields, stored in the request context. Logrus has no context-logger API of
// its own, so handlers retrieve the entry with [FromContext]. Responses are
// logged at debug level.
package logrusmw

import (
	"context"

	"github.com/sirupsen/logrus"

	logmiddleware "github.com/tpyle/log-middleware/v3"
)

// Logger is a [logmiddleware.Logger] backed by logrus.
type Logger struct {
	logger logrus.FieldLogger
}

var _ logmiddleware.Logger = (*Logger)(nil)

// NewLogger returns a [logmiddleware.Logger] backed by l, which may be a
// [*logrus.Logger] or a [*logrus.Entry] with preset fields. If l is nil,
// [logrus.StandardLogger] is used.
func NewLogger(l logrus.FieldLogger) *Logger {
	return &Logger{logger: l}
}

// New is shorthand for logmiddleware.New(NewLogger(l), opts...).
func New(l logrus.FieldLogger, opts ...logmiddleware.Option) *logmiddleware.LogMiddleware {
	return logmiddleware.New(NewLogger(l), opts...)
}

// Start implements [logmiddleware.Logger].
func (lg *Logger) Start(ctx context.Context, req logmiddleware.RequestInfo) (context.Context, logmiddleware.RequestLogger) {
	base := lg.logger
	if base == nil {
		base = logrus.StandardLogger()
	}
	entry := base.WithFields(logrus.Fields{
		logmiddleware.FieldRequestID: req.RequestID,
		logmiddleware.FieldMethod:    req.Method,
		logmiddleware.FieldPath:      req.Path,
	}).WithContext(ctx)
	return ContextWithEntry(ctx, entry), &requestLogger{entry: entry}
}

type requestLogger struct {
	entry *logrus.Entry
}

func (r *requestLogger) Finish(resp logmiddleware.ResponseInfo) {
	r.entry.WithFields(logrus.Fields{
		logmiddleware.FieldStatusCode: resp.StatusCode,
		logmiddleware.FieldDuration:   resp.Duration,
	}).Debug(logmiddleware.ResponseMessage)
}

type entryKeyType struct{}

var entryKey entryKeyType

// ContextWithEntry returns a copy of ctx carrying e, retrievable with
// [FromContext].
func ContextWithEntry(ctx context.Context, e *logrus.Entry) context.Context {
	return context.WithValue(ctx, entryKey, e)
}

// FromContext returns the entry stored in ctx by the middleware (or by
// [ContextWithEntry]). If there is none it returns a new entry on
// [logrus.StandardLogger] bound to ctx, so the result is always safe to use.
func FromContext(ctx context.Context) *logrus.Entry {
	if ctx != nil {
		if e, ok := ctx.Value(entryKey).(*logrus.Entry); ok && e != nil {
			return e
		}
	}
	e := logrus.NewEntry(logrus.StandardLogger())
	if ctx != nil {
		e = e.WithContext(ctx)
	}
	return e
}

// ContextHook is a [logrus.Hook] that adds the requestId, method and path of
// the current request to entries logged with a request context, e.g.
//
//	logger.AddHook(logrusmw.ContextHook{})
//	logger.WithContext(ctx).Info("loaded user")
//
// It lets code log with the plain logrus API and still get request fields.
// Entries without a request context are unchanged, and fields already set on
// the entry (such as those on entries from [FromContext]) are not overwritten.
type ContextHook struct{}

var _ logrus.Hook = ContextHook{}

// Levels implements [logrus.Hook]; the hook fires at every level.
func (ContextHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire implements [logrus.Hook].
func (ContextHook) Fire(e *logrus.Entry) error {
	info, ok := logmiddleware.RequestInfoFromContext(e.Context)
	if !ok {
		return nil
	}
	if e.Data == nil {
		e.Data = logrus.Fields{}
	}
	for k, v := range map[string]string{
		logmiddleware.FieldRequestID: info.RequestID,
		logmiddleware.FieldMethod:    info.Method,
		logmiddleware.FieldPath:      info.Path,
	} {
		if _, exists := e.Data[k]; !exists {
			e.Data[k] = v
		}
	}
	return nil
}
