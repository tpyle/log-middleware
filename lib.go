package logmiddleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
	requestId  string
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	if w.statusCode == 0 {
		w.statusCode = statusCode
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

type logRequestIdKey string

const (
	requestIDKey logRequestIdKey = "requestId"
)

type LogMiddleware struct {
	logger *zerolog.Logger
}

func NewLogMiddleware(logger *zerolog.Logger) *LogMiddleware {
	return &LogMiddleware{logger: logger}
}

func GetRequestIdFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestId, ok := ctx.Value(requestIDKey).(string); ok {
		return requestId
	}
	return ""
}

func GetRequestIdFromRequest(r *http.Request) string {
	return GetRequestIdFromContext(r.Context())
}

func (lm *LogMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestId := uuid.New().String()

		ctx := context.WithValue(r.Context(), requestIDKey, requestId)

		logger := lm.logger.With().Str("requestId", requestId).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Logger()
		ctx = logger.WithContext(ctx)

		wrapper := &responseWriterWrapper{
			ResponseWriter: w,
			requestId:      requestId,
		}

		wrapper.Header().Add("X-Request-Id", requestId)
		next.ServeHTTP(wrapper, r.WithContext(ctx))

		logger.Debug().
			Int("statusCode", wrapper.statusCode).
			Dur("duration", time.Since(start)).
			Msg("HTTP response sent")
	})
}
