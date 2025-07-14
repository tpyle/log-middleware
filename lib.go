package logmiddleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
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

type requestIDHook struct {
	requestIDKey logRequestIdKey
}

func (h *requestIDHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *requestIDHook) Fire(entry *logrus.Entry) error {
	ctx := entry.Context
	if ctx != nil {
		if requestID, ok := ctx.Value(h.requestIDKey).(string); ok {
			if entry.Data == nil {
				entry.Data = logrus.Fields{}
			}
			entry.Data["requestId"] = requestID
		}
	}
	return nil
}

func init() {
	logrus.AddHook(&requestIDHook{requestIDKey: requestIDKey})
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

func LogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestId := uuid.New().String()

		ctx := context.WithValue(r.Context(), requestIDKey, requestId)

		logrus.WithContext(ctx).WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
		}).Debug("HTTP request received")

		wrapper := &responseWriterWrapper{
			ResponseWriter: w,
			requestId:      requestId,
		}

		wrapper.Header().Add("X-Request-Id", requestId)
		next.ServeHTTP(wrapper, r.WithContext(ctx))

		logrus.WithContext(ctx).WithFields(logrus.Fields{
			"method":     r.Method,
			"path":       r.URL.Path,
			"statusCode": wrapper.statusCode,
			"duration":   time.Since(start),
		}).Debug("HTTP response sent")
	})
}
