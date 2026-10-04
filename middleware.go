// Package logmiddleware provides HTTP middleware that assigns every request a
// unique ID, attaches a request-scoped logger to the request context, and logs
// each completed response.
//
// The package depends only on the standard library and logs through
// [log/slog] out of the box (see [NewSlog]). Adapters for third-party loggers
// are separate modules, so those loggers only enter the dependency graph of
// programs that use them:
//
//   - github.com/tpyle/log-middleware/v3/zerologmw for zerolog
//   - github.com/tpyle/log-middleware/v3/logrusmw for logrus
//
// Any other logger can be supported by implementing [Logger].
package logmiddleware

import (
	"context"
	"net/http"
	"time"

	"uuid"
)

// RequestIDHeader is the response header the middleware sets to the request ID.
const RequestIDHeader = "X-Request-Id"

// ResponseMessage is the message logged when a response has been sent. Adapters
// should use it so output is consistent across loggers.
const ResponseMessage = "HTTP response sent"

// Field names used by the bundled adapters.
const (
	FieldRequestID  = "requestId"
	FieldMethod     = "method"
	FieldPath       = "path"
	FieldStatusCode = "statusCode"
	FieldDuration   = "duration"
)

// RequestInfo describes an incoming request.
type RequestInfo struct {
	// RequestID is the unique ID assigned to the request.
	RequestID string
	// Method is the HTTP method, e.g. "GET".
	Method string
	// Path is the URL path, without the query string.
	Path string
}

// ResponseInfo describes a completed response.
type ResponseInfo struct {
	// StatusCode is the final HTTP status sent to the client. It is
	// [http.StatusOK] if the handler wrote a body or nothing at all without
	// calling WriteHeader, matching net/http's behavior.
	StatusCode int
	// Duration is the time spent in the wrapped handler.
	Duration time.Duration
}

// Logger is the integration point between the middleware and a logging
// library.
//
// Start is called once per request before the wrapped handler runs. It should
// return ctx extended with a request-scoped logger (so handlers can retrieve it
// with the logging library's own context API) and a [RequestLogger] that is
// used to log the response.
//
// Implementations must be safe for concurrent use.
type Logger interface {
	Start(ctx context.Context, req RequestInfo) (context.Context, RequestLogger)
}

// RequestLogger logs the outcome of a single request.
type RequestLogger interface {
	// Finish is called once after the wrapped handler returns. It is not
	// called if the handler panics.
	Finish(resp ResponseInfo)
}

// LogMiddleware wraps HTTP handlers with request logging. Create one with
// [New], [NewSlog], or a constructor from an adapter module.
type LogMiddleware struct {
	logger       Logger
	newRequestID func() string
	now          func() time.Time
}

// Option configures a [LogMiddleware].
type Option func(*LogMiddleware)

// WithRequestIDGenerator replaces the default random UUID request ID generator.
// The function must be safe for concurrent use.
func WithRequestIDGenerator(generate func() string) Option {
	return func(lm *LogMiddleware) {
		if generate != nil {
			lm.newRequestID = generate
		}
	}
}

// WithClock replaces [time.Now] as the source of time used to measure request
// durations. It is mainly useful for deterministic tests.
func WithClock(now func() time.Time) Option {
	return func(lm *LogMiddleware) {
		if now != nil {
			lm.now = now
		}
	}
}

// New returns a middleware that logs through logger. It panics if logger is
// nil.
func New(logger Logger, opts ...Option) *LogMiddleware {
	if logger == nil {
		panic("logmiddleware: nil Logger")
	}
	lm := &LogMiddleware{
		logger:       logger,
		newRequestID: func() string { return uuid.NewV4().String() },
		now:          time.Now,
	}
	for _, opt := range opts {
		opt(lm)
	}
	return lm
}

// Handler wraps next so that each request gets a request ID (stored in the
// context and sent in the [RequestIDHeader] response header) and a
// request-scoped logger, and so that each response is logged.
func (lm *LogMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := lm.now()
		requestID := lm.newRequestID()

		info := RequestInfo{
			RequestID: requestID,
			Method:    r.Method,
			Path:      r.URL.Path,
		}
		ctx, reqLogger := lm.logger.Start(ContextWithRequestInfo(r.Context(), info), info)

		rw := &responseWriter{ResponseWriter: w}
		rw.Header().Set(RequestIDHeader, requestID)
		next.ServeHTTP(rw, r.WithContext(ctx))

		reqLogger.Finish(ResponseInfo{
			StatusCode: rw.status(),
			Duration:   lm.now().Sub(start),
		})
	})
}

type requestInfoKeyType struct{}

var requestInfoKey requestInfoKeyType

// ContextWithRequestInfo returns a copy of ctx carrying info. The middleware
// calls it for every request; it is exported so that tests and non-HTTP code
// (such as background jobs) can produce contexts that the context-aware log
// handlers and hooks recognize.
func ContextWithRequestInfo(ctx context.Context, info RequestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey, info)
}

// RequestInfoFromContext returns the request information stored in ctx by the
// middleware (or by [ContextWithRequestInfo]) and whether it was present.
func RequestInfoFromContext(ctx context.Context) (RequestInfo, bool) {
	if ctx == nil {
		return RequestInfo{}, false
	}
	info, ok := ctx.Value(requestInfoKey).(RequestInfo)
	return info, ok
}

// GetRequestIdFromContext returns the request ID stored in ctx by the
// middleware, or "" if there is none.
func GetRequestIdFromContext(ctx context.Context) string {
	info, _ := RequestInfoFromContext(ctx)
	return info.RequestID
}

// GetRequestIdFromRequest returns the request ID for r, or "" if r was not
// handled by the middleware.
func GetRequestIdFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	return GetRequestIdFromContext(r.Context())
}

// responseWriter records the final status code written by a handler.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader records the first non-informational status code. 1xx responses
// may be sent any number of times before the final status.
func (w *responseWriter) WriteHeader(code int) {
	if w.statusCode == 0 && code >= 200 {
		w.statusCode = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Flush implements [http.Flusher] when the underlying writer supports it, so
// streaming handlers keep working behind the middleware.
func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		if w.statusCode == 0 {
			w.statusCode = http.StatusOK
		}
		f.Flush()
	}
}

// Unwrap lets [http.ResponseController] reach the underlying writer.
func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseWriter) status() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}
