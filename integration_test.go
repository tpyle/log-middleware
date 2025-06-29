package logmiddleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// Test the init function behavior
func TestInit(t *testing.T) {
	// Save original hooks
	originalHooks := logrus.StandardLogger().Hooks

	// Clear hooks to test init
	logrus.StandardLogger().Hooks = make(logrus.LevelHooks)

	// Re-run init (simulate package initialization)
	logrus.AddHook(&requestIDHook{requestIDKey: requestIDKey})

	// Verify hook was added
	if len(logrus.StandardLogger().Hooks) == 0 {
		t.Error("Expected hook to be added to logrus")
	}

	// Check if our hook exists in all levels
	for _, level := range logrus.AllLevels {
		found := false
		for _, hook := range logrus.StandardLogger().Hooks[level] {
			if _, ok := hook.(*requestIDHook); ok {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected requestIDHook to be registered for level %v", level)
		}
	}

	// Restore original hooks
	logrus.StandardLogger().Hooks = originalHooks
}

// Test request ID key type safety
func TestRequestIDKeyType(t *testing.T) {
	// Test that different key types don't interfere
	ctx := context.Background()
	ctx = context.WithValue(ctx, "requestId", "string-key-value")         // string key
	ctx = context.WithValue(ctx, requestIDKey, "typed-key-value")         // typed key
	ctx = context.WithValue(ctx, logRequestIdKey("other"), "other-value") // different typed key

	hook := &requestIDHook{requestIDKey: requestIDKey}
	entry := &logrus.Entry{
		Context: ctx,
		Data:    logrus.Fields{},
	}

	err := hook.Fire(entry)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should only get the typed key value
	if entry.Data["requestId"] != "typed-key-value" {
		t.Errorf("Expected 'typed-key-value', got %v", entry.Data["requestId"])
	}
}

// Test concurrent access to middleware
func TestLogMiddleware_Concurrent(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Context().Value(requestIDKey).(string)
		w.Header().Set("Test-Request-ID", requestID)
		w.WriteHeader(http.StatusOK)
	})

	middleware := LogMiddleware(testHandler)

	// Run multiple requests concurrently
	const numRequests = 100
	results := make(chan string, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			recorder := httptest.NewRecorder()
			middleware.ServeHTTP(recorder, req)
			results <- recorder.Header().Get("Test-Request-ID")
		}()
	}

	// Collect results and verify uniqueness
	requestIDs := make(map[string]bool)
	for i := 0; i < numRequests; i++ {
		requestID := <-results
		if requestID == "" {
			t.Error("Expected non-empty request ID")
		}
		if requestIDs[requestID] {
			t.Errorf("Duplicate request ID found: %s", requestID)
		}
		requestIDs[requestID] = true
	}

	if len(requestIDs) != numRequests {
		t.Errorf("Expected %d unique request IDs, got %d", numRequests, len(requestIDs))
	}
}

// Test middleware with malformed requests
func TestLogMiddleware_MalformedRequests(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := LogMiddleware(testHandler)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"Root path", "GET", "/"},
		{"Path with query", "GET", "/test?param=value"},
		{"Path with fragment", "GET", "/test#fragment"},
		{"Very long path", "GET", "/test/" + strings.Repeat("a", 1000)},
		{"Path with spaces", "GET", "/test%20path"}, // URL encoded spaces
		{"Path with safe special chars", "GET", "/test-_~"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			recorder := httptest.NewRecorder()

			// Should not panic
			middleware.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", recorder.Code)
			}

			requestID := recorder.Header().Get("X-Request-Id")
			if requestID == "" {
				t.Error("Expected X-Request-Id header to be set")
			}
		})
	}
}

// Test responseWriterWrapper edge cases
func TestResponseWriterWrapper_EdgeCases(t *testing.T) {
	t.Run("Multiple WriteHeader calls", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		wrapper := &responseWriterWrapper{
			ResponseWriter: recorder,
			requestId:      "test-id",
		}

		// First call
		wrapper.WriteHeader(http.StatusOK)
		if wrapper.statusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", wrapper.statusCode)
		}

		// Second call (should not change the wrapper's status, but Go's http.ResponseWriter behavior may vary)
		wrapper.WriteHeader(http.StatusInternalServerError)
		// The wrapper should still track the first status code
		if wrapper.statusCode != http.StatusOK {
			t.Errorf("Expected wrapper status to remain 200, got %d", wrapper.statusCode)
		}

		// The underlying recorder follows Go's standard behavior (first WriteHeader wins)
		if recorder.Code != http.StatusOK {
			t.Errorf("Expected recorder code 200, got %d", recorder.Code)
		}
	})

	t.Run("Write without explicit WriteHeader", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		wrapper := &responseWriterWrapper{
			ResponseWriter: recorder,
			requestId:      "test-id",
		}

		data := []byte("test data")
		wrapper.Write(data)

		// Should implicitly set status to 200
		if recorder.Code != http.StatusOK {
			t.Errorf("Expected implicit status 200, got %d", recorder.Code)
		}

		// Wrapper statusCode should still be 0 since WriteHeader wasn't called explicitly
		if wrapper.statusCode != 0 {
			t.Errorf("Expected wrapper statusCode 0, got %d", wrapper.statusCode)
		}
	})

	t.Run("Empty write", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		wrapper := &responseWriterWrapper{
			ResponseWriter: recorder,
			requestId:      "test-id",
		}

		n, err := wrapper.Write([]byte{})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if n != 0 {
			t.Errorf("Expected 0 bytes written, got %d", n)
		}
	})

	t.Run("Large write", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		wrapper := &responseWriterWrapper{
			ResponseWriter: recorder,
			requestId:      "test-id",
		}

		largeData := make([]byte, 1024*1024) // 1MB
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		n, err := wrapper.Write(largeData)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if n != len(largeData) {
			t.Errorf("Expected %d bytes written, got %d", len(largeData), n)
		}
	})
}

// Test context inheritance and isolation
func TestLogMiddleware_ContextIsolation(t *testing.T) {
	var capturedContexts []context.Context

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedContexts = append(capturedContexts, r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := LogMiddleware(testHandler)

	// Make multiple requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		recorder := httptest.NewRecorder()
		middleware.ServeHTTP(recorder, req)
	}

	// Verify each request has a unique context with unique request ID
	if len(capturedContexts) != 3 {
		t.Fatalf("Expected 3 contexts, got %d", len(capturedContexts))
	}

	requestIDs := make([]string, 3)
	for i, ctx := range capturedContexts {
		requestID, ok := ctx.Value(requestIDKey).(string)
		if !ok || requestID == "" {
			t.Errorf("Request %d: expected non-empty request ID", i)
		}
		requestIDs[i] = requestID
	}

	// Verify all request IDs are unique
	for i := 0; i < len(requestIDs); i++ {
		for j := i + 1; j < len(requestIDs); j++ {
			if requestIDs[i] == requestIDs[j] {
				t.Errorf("Duplicate request ID: %s", requestIDs[i])
			}
		}
	}
}

// Test hook with entry that has no Data field initialized
func TestRequestIDHook_Fire_NilData(t *testing.T) {
	hook := &requestIDHook{requestIDKey: requestIDKey}
	requestID := "test-request-id"
	ctx := context.WithValue(context.Background(), requestIDKey, requestID)

	entry := &logrus.Entry{
		Context: ctx,
		Data:    nil, // nil data
	}

	// Should not panic
	err := hook.Fire(entry)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Data should be initialized and contain requestId
	if entry.Data == nil {
		t.Error("Expected Data to be initialized")
	} else if entry.Data["requestId"] != requestID {
		t.Errorf("Expected requestId %s, got %v", requestID, entry.Data["requestId"])
	}
}
