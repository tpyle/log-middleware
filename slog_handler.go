package logmiddleware

import (
	"context"
	"log/slog"
)

// ContextHandler is a [slog.Handler] that adds the requestId, method and path
// of the current request to every record logged with a context that went
// through the middleware. It lets code log with the plain slog API, e.g.
//
//	slog.InfoContext(ctx, "loaded user")
//
// and still get request fields, without depending on this package. Records
// logged without a request context (including calls like [slog.Info] that
// take no context) are passed through unchanged.
//
// If requestId is already attached to the logger with [slog.Logger.With] (as
// it is on loggers returned by [SlogFromContext]), the fields are not added a
// second time.
//
// Fields are added where the record's attributes go, so after
// [slog.Logger.WithGroup] they appear inside that group.
type ContextHandler struct {
	inner slog.Handler
	// skip is set once requestId has been bound at the top level via WithAttrs.
	skip bool
	// grouped is set once WithGroup has been called, after which attributes
	// passed to WithAttrs are no longer top level.
	grouped bool
}

var _ slog.Handler = (*ContextHandler)(nil)

// NewContextHandler wraps inner so that request fields are added from the
// context. Wrapping a *ContextHandler returns it unchanged. It panics if inner
// is nil; to wrap the default handler, pass slog.Default().Handler() before
// calling [slog.SetDefault].
func NewContextHandler(inner slog.Handler) *ContextHandler {
	if inner == nil {
		panic("logmiddleware: nil slog.Handler")
	}
	if ch, ok := inner.(*ContextHandler); ok {
		return ch
	}
	return &ContextHandler{inner: inner}
}

// Enabled implements [slog.Handler].
func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle implements [slog.Handler].
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if !h.skip {
		if info, ok := RequestInfoFromContext(ctx); ok {
			r = r.Clone()
			r.AddAttrs(
				slog.String(FieldRequestID, info.RequestID),
				slog.String(FieldMethod, info.Method),
				slog.String(FieldPath, info.Path),
			)
		}
	}
	return h.inner.Handle(ctx, r)
}

// WithAttrs implements [slog.Handler].
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	skip := h.skip
	if !h.grouped {
		for _, a := range attrs {
			if a.Key == FieldRequestID {
				skip = true
				break
			}
		}
	}
	return &ContextHandler{inner: h.inner.WithAttrs(attrs), skip: skip, grouped: h.grouped}
}

// WithGroup implements [slog.Handler].
func (h *ContextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &ContextHandler{inner: h.inner.WithGroup(name), skip: h.skip, grouped: true}
}

// Unwrap returns the wrapped handler.
func (h *ContextHandler) Unwrap() slog.Handler {
	return h.inner
}
