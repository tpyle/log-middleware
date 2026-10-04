package logmiddleware

import (
	"context"
	"log/slog"
)

// SlogLogger is a [Logger] that writes through a [*slog.Logger].
//
// Each request gets a child logger carrying the requestId, method and path
// attributes, stored in the context for retrieval with [SlogFromContext].
// Responses are logged at [slog.LevelDebug].
type SlogLogger struct {
	logger *slog.Logger
}

var _ Logger = (*SlogLogger)(nil)

// NewSlogLogger returns a [Logger] backed by l. If l is nil, [slog.Default] is
// used, looked up on every request so later calls to [slog.SetDefault] apply.
func NewSlogLogger(l *slog.Logger) *SlogLogger {
	return &SlogLogger{logger: l}
}

// NewSlog is shorthand for New(NewSlogLogger(l), opts...).
func NewSlog(l *slog.Logger, opts ...Option) *LogMiddleware {
	return New(NewSlogLogger(l), opts...)
}

// Start implements [Logger].
func (s *SlogLogger) Start(ctx context.Context, req RequestInfo) (context.Context, RequestLogger) {
	base := s.logger
	if base == nil {
		base = slog.Default()
	}
	l := base.With(
		slog.String(FieldRequestID, req.RequestID),
		slog.String(FieldMethod, req.Method),
		slog.String(FieldPath, req.Path),
	)
	ctx = ContextWithSlog(ctx, l)
	return ctx, &slogRequestLogger{ctx: ctx, logger: l}
}

type slogRequestLogger struct {
	ctx    context.Context
	logger *slog.Logger
}

func (s *slogRequestLogger) Finish(resp ResponseInfo) {
	s.logger.LogAttrs(s.ctx, slog.LevelDebug, ResponseMessage,
		slog.Int(FieldStatusCode, resp.StatusCode),
		slog.Duration(FieldDuration, resp.Duration),
	)
}

type slogKeyType struct{}

var slogKey slogKeyType

// ContextWithSlog returns a copy of ctx carrying l, retrievable with
// [SlogFromContext].
func ContextWithSlog(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, slogKey, l)
}

// SlogFromContext returns the logger stored in ctx by the middleware (or by
// [ContextWithSlog]). If there is none it returns [slog.Default], so the result
// is always safe to use.
func SlogFromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(slogKey).(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return slog.Default()
}
