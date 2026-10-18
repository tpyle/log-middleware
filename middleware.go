// Package logmiddleware provides HTTP middleware that assigns every request a
// unique ID, attaches a request-scoped logger to the request context, and logs
// each completed response.
//
// The package depends only on the standard library and logs through
// [log/slog] out of the box (see [NewSlog]). Adapters for third-party loggers
// are separate modules, so those loggers only enter the dependency graph of
// programs that use them:
//
//   - github.com/tpyle/log-middleware/zerologmw/v3 for zerolog
//   - github.com/tpyle/log-middleware/logrusmw/v3 for logrus
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
	logger    Logger
	requestID RequestIDGenerator
	now       func() time.Time
}

// RequestIDGenerator produces the ID for an incoming request. The ID is stored
// in the request context, passed to [Logger.Start] and sent to the client in
// the [RequestIDHeader] response header.
//
// Implementations must be safe for concurrent use. Implementations that take
// the ID from client-controlled input, such as an incoming header, should
// validate it (for example with [ValidRequestID]), since it is written to logs
// and echoed back in the response. [RequestIDFromHeader] does this for the
// common case of reusing an incoming header.
type RequestIDGenerator interface {
	GenerateRequestID(r *http.Request) string
}

// RequestIDGeneratorFunc adapts an ordinary function to a [RequestIDGenerator].
type RequestIDGeneratorFunc func(r *http.Request) string

// GenerateRequestID returns f(r).
func (f RequestIDGeneratorFunc) GenerateRequestID(r *http.Request) string {
	return f(r)
}

// UUIDRequestID returns a new random (version 4) UUID and ignores r. It is the
// default request ID generator; it is exported so custom generators can fall
// back to it.
func UUIDRequestID(*http.Request) string {
	return uuid.NewV4().String()
}

// MaxRequestIDLength is the longest ID accepted by [ValidRequestID].
const MaxRequestIDLength = 128

// ValidRequestID reports whether id is safe to use as a request ID taken from
// client-controlled input. A valid ID is 1 to [MaxRequestIDLength] bytes of
// ASCII letters, digits and the characters - _ . : + / =. This admits common
// formats such as UUIDs, ULIDs, base64, W3C traceparent values and AWS trace
// IDs, while rejecting whitespace, control characters, quotes and other
// characters that could be used to forge or break log lines.
func ValidRequestID(id string) bool {
	if len(id) == 0 || len(id) > MaxRequestIDLength {
		return false
	}
	for i := 0; i < len(id); i++ {
		switch c := id[i]; {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case c == '-', c == '_', c == '.', c == ':', c == '+', c == '/', c == '=':
		default:
			return false
		}
	}
	return true
}

// RequestIDFromHeader returns a [RequestIDGenerator] that uses the value of
// the named request header when it passes [ValidRequestID], and otherwise
// returns fallback's ID. If the header appears more than once, only the first
// value is considered. Pass [RequestIDHeader] to reuse IDs set by an upstream
// proxy or caller, and RequestIDGeneratorFunc([UUIDRequestID]) as fallback to
// keep the default behavior for requests without one.
//
// It panics if fallback is nil.
func RequestIDFromHeader(header string, fallback RequestIDGenerator) RequestIDGenerator {
	if f, ok := fallback.(RequestIDGeneratorFunc); fallback == nil || (ok && f == nil) {
		panic("logmiddleware: nil fallback RequestIDGenerator")
	}
	return RequestIDGeneratorFunc(func(r *http.Request) string {
		if id := r.Header.Get(header); ValidRequestID(id) {
			return id
		}
		return fallback.GenerateRequestID(r)
	})
}

// Option configures a [LogMiddleware].
type Option func(*LogMiddleware)

// WithRequestIDGenerator replaces the default random UUID request ID generator
// with one that does not need the request. The function must be safe for
// concurrent use. To derive the ID from the request, use
// [WithRequestIDFromRequest].
func WithRequestIDGenerator(generate func() string) Option {
	if generate == nil {
		return func(*LogMiddleware) {}
	}
	return WithRequestIDFromRequest(RequestIDGeneratorFunc(func(*http.Request) string {
		return generate()
	}))
}

// WithRequestIDFromRequest replaces the default random UUID request ID
// generator with g, which is called once per request with the incoming
// request. A nil g, including a nil [RequestIDGeneratorFunc], keeps the
// current generator.
func WithRequestIDFromRequest(g RequestIDGenerator) Option {
	return func(lm *LogMiddleware) {
		if f, ok := g.(RequestIDGeneratorFunc); g == nil || (ok && f == nil) {
			return
		}
		lm.requestID = g
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
		logger:    logger,
		requestID: RequestIDGeneratorFunc(UUIDRequestID),
		now:       time.Now,
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
		requestID := lm.requestID.GenerateRequestID(r)

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
