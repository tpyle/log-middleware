package logmiddleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"uuid"
)

// fakeLogger records every call so tests can assert on what the middleware
// passed to its Logger.
type fakeLogger struct {
	mu       sync.Mutex
	starts   []RequestInfo
	finishes []ResponseInfo
}

type fakeCtxKey struct{}

func (f *fakeLogger) Start(ctx context.Context, req RequestInfo) (context.Context, RequestLogger) {
	f.mu.Lock()
	f.starts = append(f.starts, req)
	f.mu.Unlock()
	return context.WithValue(ctx, fakeCtxKey{}, "from-logger"), &fakeRequestLogger{parent: f}
}

type fakeRequestLogger struct{ parent *fakeLogger }

func (f *fakeRequestLogger) Finish(resp ResponseInfo) {
	f.parent.mu.Lock()
	f.parent.finishes = append(f.parent.finishes, resp)
	f.parent.mu.Unlock()
}

func fixedID(id string) Option {
	return WithRequestIDGenerator(func() string { return id })
}

// steppingClock returns a clock that advances by step on every call.
func steppingClock(step time.Duration) func() time.Time {
	t := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return func() time.Time {
		cur := t
		t = t.Add(step)
		return cur
	}
}

func serve(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestNew_NilLoggerPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil Logger")
		}
	}()
	New(nil)
}

func TestNew_NilOptionsKeepDefaults(t *testing.T) {
	lm := New(&fakeLogger{}, WithRequestIDGenerator(nil), WithClock(nil))
	if lm.newRequestID == nil || lm.now == nil {
		t.Fatal("nil options should not clear defaults")
	}
	id := lm.newRequestID()
	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("default generator should produce a UUID: %v", err)
	}
	if len(id) != 36 || id[14] != '4' {
		t.Errorf("default generator should produce a version 4 UUID, got %q", id)
	}
}

func TestHandler_PassesRequestInfo(t *testing.T) {
	tests := []struct {
		method, target, wantPath string
	}{
		{"GET", "/", "/"},
		{"POST", "/api/users", "/api/users"},
		{"DELETE", "/api/users/123?force=true", "/api/users/123"},
		{"PATCH", "/a%20b", "/a b"},
		{"OPTIONS", "/" + strings.Repeat("x", 1000), "/" + strings.Repeat("x", 1000)},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.target[:min(len(tt.target), 20)], func(t *testing.T) {
			fl := &fakeLogger{}
			h := New(fl, fixedID("id-1")).Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			serve(t, h, tt.method, tt.target)

			want := RequestInfo{RequestID: "id-1", Method: tt.method, Path: tt.wantPath}
			if len(fl.starts) != 1 || fl.starts[0] != want {
				t.Errorf("starts = %+v, want [%+v]", fl.starts, want)
			}
		})
	}
}

func TestHandler_StatusCodes(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    int
	}{
		{"nothing written", func(http.ResponseWriter, *http.Request) {}, http.StatusOK},
		{"body only", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hi")) }, http.StatusOK},
		{"explicit 201", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) }, http.StatusCreated},
		{"explicit 404", func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) }, http.StatusNotFound},
		{"explicit 500", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }, 500},
		{"informational then final", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusEarlyHints)
			w.WriteHeader(http.StatusAccepted)
		}, http.StatusAccepted},
		{"second WriteHeader ignored", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
			w.WriteHeader(http.StatusOK)
		}, http.StatusTeapot},
		{"write then WriteHeader", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("x"))
			w.WriteHeader(http.StatusBadRequest)
		}, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fl := &fakeLogger{}
			serve(t, New(fl).Handler(tt.handler), "GET", "/")
			if len(fl.finishes) != 1 {
				t.Fatalf("Finish called %d times, want 1", len(fl.finishes))
			}
			if got := fl.finishes[0].StatusCode; got != tt.want {
				t.Errorf("StatusCode = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHandler_Duration(t *testing.T) {
	fl := &fakeLogger{}
	h := New(fl, WithClock(steppingClock(42*time.Millisecond))).
		Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	serve(t, h, "GET", "/")
	if got := fl.finishes[0].Duration; got != 42*time.Millisecond {
		t.Errorf("Duration = %v, want 42ms", got)
	}
}

func TestHandler_ContextAndHeader(t *testing.T) {
	fl := &fakeLogger{}
	var gotID string
	var gotLoggerValue any
	h := New(fl, fixedID("abc")).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = GetRequestIdFromRequest(r)
		gotLoggerValue = r.Context().Value(fakeCtxKey{})
	}))
	rec := serve(t, h, "GET", "/")

	if gotID != "abc" {
		t.Errorf("request ID in context = %q, want abc", gotID)
	}
	if gotLoggerValue != "from-logger" {
		t.Error("context returned by Logger.Start was not passed to the handler")
	}
	if got := rec.Header().Get(RequestIDHeader); got != "abc" {
		t.Errorf("%s header = %q, want abc", RequestIDHeader, got)
	}
}

func TestHandler_OverridesIncomingRequestID(t *testing.T) {
	var gotID string
	h := New(&fakeLogger{}, fixedID("new")).Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotID = GetRequestIdFromRequest(r)
	}))
	ctx := ContextWithRequestInfo(context.Background(), RequestInfo{RequestID: "old"})
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	if gotID != "new" {
		t.Errorf("request ID = %q, want new", gotID)
	}
}

func TestHandler_PreservesParentContext(t *testing.T) {
	type key struct{}
	var got any
	h := New(&fakeLogger{}).Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Context().Value(key{})
	}))
	ctx := context.WithValue(context.Background(), key{}, "kept")
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	if got != "kept" {
		t.Errorf("parent context value = %v, want kept", got)
	}
}

func TestHandler_PanicSkipsFinish(t *testing.T) {
	fl := &fakeLogger{}
	h := New(fl).Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	func() {
		defer func() {
			if recover() == nil {
				t.Error("expected panic to propagate")
			}
		}()
		serve(t, h, "GET", "/")
	}()
	if len(fl.starts) != 1 || len(fl.finishes) != 0 {
		t.Errorf("starts=%d finishes=%d, want 1 and 0", len(fl.starts), len(fl.finishes))
	}
}

func TestHandler_ConcurrentUniqueIDs(t *testing.T) {
	const n = 100
	h := New(&fakeLogger{}).Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	ids := make(chan string, n)
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
			ids <- rec.Header().Get(RequestIDHeader)
		})
	}
	wg.Wait()
	close(ids)

	seen := map[string]bool{}
	for id := range ids {
		if id == "" || seen[id] {
			t.Fatalf("empty or duplicate request ID %q", id)
		}
		seen[id] = true
	}
}

func TestGetRequestIdFromContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"present", ContextWithRequestInfo(context.Background(), RequestInfo{RequestID: "id"}), "id"},
		{"absent", context.Background(), ""},
		{"nil", nil, ""},
		{"wrong type", context.WithValue(context.Background(), requestInfoKey, "id"), ""},
		{"string key not used", context.WithValue(context.Background(), "requestId", "id"), ""}, //nolint:staticcheck // intentionally untyped key
		{"empty", ContextWithRequestInfo(context.Background(), RequestInfo{}), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetRequestIdFromContext(tt.ctx); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequestInfoFromContext(t *testing.T) {
	want := RequestInfo{RequestID: "id", Method: "GET", Path: "/p"}
	tests := []struct {
		name   string
		ctx    context.Context
		want   RequestInfo
		wantOK bool
	}{
		{"present", ContextWithRequestInfo(context.Background(), want), want, true},
		{"absent", context.Background(), RequestInfo{}, false},
		{"nil", nil, RequestInfo{}, false},
		{"wrong type", context.WithValue(context.Background(), requestInfoKey, &want), RequestInfo{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := RequestInfoFromContext(tt.ctx)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("got (%+v, %v), want (%+v, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestHandler_StoresRequestInfoInContext(t *testing.T) {
	var got RequestInfo
	var ok bool
	h := New(&fakeLogger{}, fixedID("id-9")).Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, ok = RequestInfoFromContext(r.Context())
	}))
	serve(t, h, "POST", "/things?q=1")
	want := RequestInfo{RequestID: "id-9", Method: "POST", Path: "/things"}
	if !ok || got != want {
		t.Errorf("got (%+v, %v), want %+v", got, ok, want)
	}
}

func TestGetRequestIdFromRequest(t *testing.T) {
	if got := GetRequestIdFromRequest(nil); got != "" {
		t.Errorf("nil request: got %q", got)
	}
	r := httptest.NewRequest("GET", "/", nil)
	if got := GetRequestIdFromRequest(r); got != "" {
		t.Errorf("plain request: got %q", got)
	}
	r = r.WithContext(ContextWithRequestInfo(r.Context(), RequestInfo{RequestID: "x"}))
	if got := GetRequestIdFromRequest(r); got != "x" {
		t.Errorf("got %q, want x", got)
	}
}

// noFlushWriter hides the Flush method of the writer it wraps, to model
// writers that cannot flush.
type noFlushWriter struct{ http.ResponseWriter }

func TestResponseWriter_Flush(t *testing.T) {
	t.Run("delegates", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &responseWriter{ResponseWriter: rec}
		w.Flush()
		if !rec.Flushed {
			t.Error("underlying recorder not flushed")
		}
		if w.status() != http.StatusOK {
			t.Errorf("status after flush = %d, want 200", w.status())
		}
	})
	t.Run("unsupported is no-op", func(t *testing.T) {
		w := &responseWriter{ResponseWriter: noFlushWriter{httptest.NewRecorder()}}
		w.Flush()
		if w.statusCode != 0 {
			t.Errorf("statusCode = %d, want 0", w.statusCode)
		}
	})
	t.Run("handler sees Flusher", func(t *testing.T) {
		h := New(&fakeLogger{}).Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if _, ok := w.(http.Flusher); !ok {
				t.Error("wrapped writer is not an http.Flusher")
			}
		}))
		serve(t, h, "GET", "/")
	})
}

func TestResponseWriter_UnwrapWorksWithResponseController(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &responseWriter{ResponseWriter: noFlushWriter{rec}}
	if w.Unwrap() != (noFlushWriter{rec}) {
		t.Fatal("Unwrap returned wrong writer")
	}
	// SetWriteDeadline is unsupported by the recorder; ResponseController must
	// report that rather than fail to find the method on the wrapper.
	err := http.NewResponseController(w).SetWriteDeadline(time.Now())
	if err == nil || !strings.Contains(fmt.Sprint(err), "not supported") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResponseWriter_WriteBody(t *testing.T) {
	for _, body := range [][]byte{nil, {}, []byte("hello"), make([]byte, 1<<20)} {
		rec := httptest.NewRecorder()
		w := &responseWriter{ResponseWriter: rec}
		n, err := w.Write(body)
		if err != nil || n != len(body) || rec.Body.Len() != len(body) {
			t.Errorf("len %d: n=%d err=%v body=%d", len(body), n, err, rec.Body.Len())
		}
	}
}

func BenchmarkHandler(b *testing.B) {
	h := New(&nopLogger{}).Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	for b.Loop() {
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
}

type nopLogger struct{}

func (nopLogger) Start(ctx context.Context, _ RequestInfo) (context.Context, RequestLogger) {
	return ctx, nopLogger{}
}
func (nopLogger) Finish(ResponseInfo) {}
