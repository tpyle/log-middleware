package logmiddleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func TestResponseWriterWrapper_WriteHeader(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"OK status", http.StatusOK},
		{"Not Found status", http.StatusNotFound},
		{"Internal Server Error", http.StatusInternalServerError},
		{"Created status", http.StatusCreated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			wrapper := &responseWriterWrapper{
				ResponseWriter: recorder,
				requestId:      "test-request-id",
			}

			wrapper.WriteHeader(tt.statusCode)

			if wrapper.statusCode != tt.statusCode {
				t.Errorf("Expected statusCode %d, got %d", tt.statusCode, wrapper.statusCode)
			}

			if recorder.Code != tt.statusCode {
				t.Errorf("Expected recorder code %d, got %d", tt.statusCode, recorder.Code)
			}
		})
	}
}

func TestResponseWriterWrapper_Write(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapper := &responseWriterWrapper{
		ResponseWriter: recorder,
		requestId:      "test-request-id",
	}

	data := []byte("test response body")
	n, err := wrapper.Write(data)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	if !bytes.Equal(recorder.Body.Bytes(), data) {
		t.Errorf("Expected body %s, got %s", string(data), recorder.Body.String())
	}
}

func TestResponseWriterWrapper_Header(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapper := &responseWriterWrapper{
		ResponseWriter: recorder,
		requestId:      "test-request-id",
	}

	header := wrapper.Header()
	header.Set("Test-Header", "test-value")

	if header.Get("Test-Header") != "test-value" {
		t.Errorf("Expected header value 'test-value', got '%s'", header.Get("Test-Header"))
	}

	if recorder.Header().Get("Test-Header") != "test-value" {
		t.Errorf("Expected recorder header value 'test-value', got '%s'", recorder.Header().Get("Test-Header"))
	}
}

func TestRequestIDHook_Levels(t *testing.T) {
	hook := &requestIDHook{requestIDKey: requestIDKey}
	levels := hook.Levels()

	if len(levels) != len(logrus.AllLevels) {
		t.Errorf("Expected %d levels, got %d", len(logrus.AllLevels), len(levels))
	}

	for i, level := range levels {
		if level != logrus.AllLevels[i] {
			t.Errorf("Expected level %v at index %d, got %v", logrus.AllLevels[i], i, level)
		}
	}
}

func TestRequestIDHook_Fire_WithRequestID(t *testing.T) {
	hook := &requestIDHook{requestIDKey: requestIDKey}
	requestID := "test-request-id-123"
	ctx := context.WithValue(context.Background(), requestIDKey, requestID)

	entry := &logrus.Entry{
		Context: ctx,
		Data:    logrus.Fields{},
	}

	err := hook.Fire(entry)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if entry.Data["requestId"] != requestID {
		t.Errorf("Expected requestId %s, got %v", requestID, entry.Data["requestId"])
	}
}

func TestRequestIDHook_Fire_WithoutRequestID(t *testing.T) {
	hook := &requestIDHook{requestIDKey: requestIDKey}
	ctx := context.Background()

	entry := &logrus.Entry{
		Context: ctx,
		Data:    logrus.Fields{},
	}

	err := hook.Fire(entry)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if _, exists := entry.Data["requestId"]; exists {
		t.Error("Expected no requestId in entry data")
	}
}

func TestRequestIDHook_Fire_WithNilContext(t *testing.T) {
	hook := &requestIDHook{requestIDKey: requestIDKey}

	entry := &logrus.Entry{
		Context: nil,
		Data:    logrus.Fields{},
	}

	err := hook.Fire(entry)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if _, exists := entry.Data["requestId"]; exists {
		t.Error("Expected no requestId in entry data when context is nil")
	}
}

func TestRequestIDHook_Fire_WithWrongTypeRequestID(t *testing.T) {
	hook := &requestIDHook{requestIDKey: requestIDKey}
	ctx := context.WithValue(context.Background(), requestIDKey, 123) // Wrong type

	entry := &logrus.Entry{
		Context: ctx,
		Data:    logrus.Fields{},
	}

	err := hook.Fire(entry)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if _, exists := entry.Data["requestId"]; exists {
		t.Error("Expected no requestId in entry data when value is wrong type")
	}
}

func TestLogMiddleware_Success(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.DebugLevel)
	defer func() {
		logrus.SetOutput(io.Discard)
	}()

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Wrap with middleware
	middleware := LogMiddleware(testHandler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	// Execute
	middleware.ServeHTTP(recorder, req)

	// Verify response
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	if recorder.Body.String() != "test response" {
		t.Errorf("Expected body 'test response', got '%s'", recorder.Body.String())
	}

	// Verify X-Request-Id header is set
	requestID := recorder.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Error("Expected X-Request-Id header to be set")
	}

	// Verify it's a valid UUID
	if _, err := uuid.Parse(requestID); err != nil {
		t.Errorf("Expected valid UUID in X-Request-Id header, got %s: %v", requestID, err)
	}

	// Verify log output contains expected entries
	logOutput := buf.String()
	if !strings.Contains(logOutput, "HTTP request received") {
		t.Error("Expected log to contain 'HTTP request received'")
	}
	if !strings.Contains(logOutput, "HTTP response sent") {
		t.Error("Expected log to contain 'HTTP response sent'")
	}
	if !strings.Contains(logOutput, "GET") {
		t.Error("Expected log to contain request method")
	}
	if !strings.Contains(logOutput, "/test") {
		t.Error("Expected log to contain request path")
	}
	if !strings.Contains(logOutput, requestID) {
		t.Error("Expected log to contain request ID")
	}
}

func TestLogMiddleware_DifferentMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var buf bytes.Buffer
			logrus.SetOutput(&buf)
			logrus.SetLevel(logrus.DebugLevel)
			defer func() {
				logrus.SetOutput(io.Discard)
			}()

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := LogMiddleware(testHandler)
			req := httptest.NewRequest(method, "/test", nil)
			recorder := httptest.NewRecorder()

			middleware.ServeHTTP(recorder, req)

			logOutput := buf.String()
			if !strings.Contains(logOutput, method) {
				t.Errorf("Expected log to contain method %s", method)
			}
		})
	}
}

func TestLogMiddleware_DifferentPaths(t *testing.T) {
	paths := []string{"/", "/api/users", "/api/users/123", "/health", "/metrics"}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			var buf bytes.Buffer
			logrus.SetOutput(&buf)
			logrus.SetLevel(logrus.DebugLevel)
			defer func() {
				logrus.SetOutput(io.Discard)
			}()

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := LogMiddleware(testHandler)
			req := httptest.NewRequest("GET", path, nil)
			recorder := httptest.NewRecorder()

			middleware.ServeHTTP(recorder, req)

			logOutput := buf.String()
			if !strings.Contains(logOutput, path) {
				t.Errorf("Expected log to contain path %s", path)
			}
		})
	}
}

func TestLogMiddleware_DifferentStatusCodes(t *testing.T) {
	statusCodes := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusNotFound,
		http.StatusInternalServerError,
	}

	for _, statusCode := range statusCodes {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			var buf bytes.Buffer
			logrus.SetOutput(&buf)
			logrus.SetLevel(logrus.DebugLevel)
			defer func() {
				logrus.SetOutput(io.Discard)
			}()

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
			})

			middleware := LogMiddleware(testHandler)
			req := httptest.NewRequest("GET", "/test", nil)
			recorder := httptest.NewRecorder()

			middleware.ServeHTTP(recorder, req)

			if recorder.Code != statusCode {
				t.Errorf("Expected status %d, got %d", statusCode, recorder.Code)
			}

			logOutput := buf.String()
			if !strings.Contains(logOutput, "statusCode") {
				t.Error("Expected log to contain statusCode field")
			}
		})
	}
}

func TestLogMiddleware_WithExistingContext(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.DebugLevel)
	defer func() {
		logrus.SetOutput(io.Discard)
	}()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request has the context with request ID
		if requestID := r.Context().Value(requestIDKey); requestID == nil {
			t.Error("Expected request context to contain request ID")
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := LogMiddleware(testHandler)

	// Create request with existing context
	ctx := context.WithValue(context.Background(), "existing", "value")
	req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()

	middleware.ServeHTTP(recorder, req)

	// Verify original context values are preserved
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestLogMiddleware_TimingMeasurement(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.DebugLevel)
	defer func() {
		logrus.SetOutput(io.Discard)
	}()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	middleware := LogMiddleware(testHandler)
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	middleware.ServeHTTP(recorder, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "duration") {
		t.Error("Expected log to contain duration field")
	}
}

func TestLogMiddleware_HandlerPanic(t *testing.T) {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.DebugLevel)
	defer func() {
		logrus.SetOutput(io.Discard)
	}()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	middleware := LogMiddleware(testHandler)
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	// Capture panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected handler to panic")
		}
	}()

	middleware.ServeHTTP(recorder, req)
}

func TestLogMiddleware_NoLogsWhenLevelAboveDebug(t *testing.T) {
	var buf bytes.Buffer
	originalLevel := logrus.GetLevel()
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.InfoLevel) // Above debug level
	defer func() {
		logrus.SetOutput(io.Discard)
		logrus.SetLevel(originalLevel)
	}()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := LogMiddleware(testHandler)
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	middleware.ServeHTTP(recorder, req)

	logOutput := buf.String()
	if strings.Contains(logOutput, "HTTP request received") || strings.Contains(logOutput, "HTTP response sent") {
		t.Error("Expected no debug logs when log level is above debug")
	}
}

// Benchmark tests
func BenchmarkLogMiddleware(b *testing.B) {
	logrus.SetOutput(io.Discard) // Discard logs for benchmarking
	defer func() {
		logrus.SetOutput(io.Discard)
	}()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := LogMiddleware(testHandler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		recorder := httptest.NewRecorder()
		middleware.ServeHTTP(recorder, req)
	}
}

func BenchmarkRequestIDHook_Fire(b *testing.B) {
	hook := &requestIDHook{requestIDKey: requestIDKey}
	requestID := "test-request-id-123"
	ctx := context.WithValue(context.Background(), requestIDKey, requestID)

	entry := &logrus.Entry{
		Context: ctx,
		Data:    logrus.Fields{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry.Data = logrus.Fields{} // Reset data
		hook.Fire(entry)
	}
}
