package logrusmw

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	logmiddleware "github.com/tpyle/log-middleware/v3"
)

func opts() []logmiddleware.Option {
	t := time.Unix(0, 0)
	return []logmiddleware.Option{
		logmiddleware.WithRequestIDGenerator(func() string { return "req-1" }),
		logmiddleware.WithClock(func() time.Time { t = t.Add(7 * time.Millisecond); return t }),
	}
}

func newTestLogger(level logrus.Level) (*logrus.Logger, *test.Hook) {
	l, hook := test.NewNullLogger()
	l.SetLevel(level)
	return l, hook
}

func TestResponseLog(t *testing.T) {
	l, hook := newTestLogger(logrus.DebugLevel)
	h := New(l, opts()...).Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("DELETE", "/items/1", nil))

	if len(hook.Entries) != 1 {
		t.Fatalf("got %d entries", len(hook.Entries))
	}
	e := hook.LastEntry()
	if e.Level != logrus.DebugLevel || e.Message != logmiddleware.ResponseMessage {
		t.Errorf("level=%v message=%q", e.Level, e.Message)
	}
	want := logrus.Fields{
		logmiddleware.FieldRequestID:  "req-1",
		logmiddleware.FieldMethod:     "DELETE",
		logmiddleware.FieldPath:       "/items/1",
		logmiddleware.FieldStatusCode: http.StatusAccepted,
		logmiddleware.FieldDuration:   7 * time.Millisecond,
	}
	for k, v := range want {
		if e.Data[k] != v {
			t.Errorf("%s = %v, want %v", k, e.Data[k], v)
		}
	}
	if logmiddleware.GetRequestIdFromContext(e.Context) != "req-1" {
		t.Error("entry context does not carry the request ID")
	}
}

func TestFromContextInHandler(t *testing.T) {
	l, hook := newTestLogger(logrus.InfoLevel)
	h := New(l, opts()...).Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		FromContext(r.Context()).WithField("user", "bob").Info("in handler")
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/users", nil))

	if len(hook.Entries) != 1 {
		t.Fatalf("got %d entries, want 1 (debug filtered)", len(hook.Entries))
	}
	e := hook.LastEntry()
	if e.Message != "in handler" || e.Data["user"] != "bob" || e.Data[logmiddleware.FieldRequestID] != "req-1" ||
		e.Data[logmiddleware.FieldMethod] != "GET" || e.Data[logmiddleware.FieldPath] != "/users" {
		t.Errorf("unexpected entry: %q %v", e.Message, e.Data)
	}
}

func TestEntryWithPresetFields(t *testing.T) {
	l, hook := newTestLogger(logrus.DebugLevel)
	New(l.WithField("service", "api"), opts()...).
		Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if hook.LastEntry().Data["service"] != "api" {
		t.Error("preset entry fields were dropped")
	}
}

func TestNilUsesStandardLogger(t *testing.T) {
	std := logrus.StandardLogger()
	origOut, origLevel := std.Out, std.Level
	t.Cleanup(func() { std.SetOutput(origOut); std.SetLevel(origLevel) })

	var buf bytes.Buffer
	std.SetOutput(&buf)
	std.SetLevel(logrus.DebugLevel)
	New(nil, opts()...).Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if !bytes.Contains(buf.Bytes(), []byte("requestId=req-1")) {
		t.Errorf("standard logger not used: %q", buf.String())
	}
}

func TestFromContext(t *testing.T) {
	l, _ := newTestLogger(logrus.InfoLevel)
	stored := l.WithField("k", "v")
	type key struct{}
	plainCtx := context.WithValue(context.Background(), key{}, 1)

	tests := []struct {
		name       string
		ctx        context.Context
		wantStored bool
	}{
		{"stored", ContextWithEntry(context.Background(), stored), true},
		{"absent", plainCtx, false},
		{"nil ctx", nil, false},
		{"nil entry stored", ContextWithEntry(context.Background(), nil), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromContext(tt.ctx)
			if got == nil {
				t.Fatal("FromContext returned nil")
			}
			if (got == stored) != tt.wantStored {
				t.Errorf("returned stored entry = %v, want %v", got == stored, tt.wantStored)
			}
			if !tt.wantStored && got.Logger != logrus.StandardLogger() {
				t.Error("fallback entry should use the standard logger")
			}
			if tt.ctx != nil && !tt.wantStored && got.Context != tt.ctx {
				t.Error("fallback entry should be bound to ctx")
			}
		})
	}
}

func BenchmarkLogrus(b *testing.B) {
	l, _ := newTestLogger(logrus.InfoLevel)
	h := New(l).Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	for b.Loop() {
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
}

var hookInfo = logmiddleware.RequestInfo{RequestID: "req-h", Method: "PUT", Path: "/h"}

func TestContextHook(t *testing.T) {
	reqCtx := logmiddleware.ContextWithRequestInfo(context.Background(), hookInfo)
	tests := []struct {
		name string
		log  func(l *logrus.Logger)
		want logrus.Fields // nil means request fields must be absent
	}{
		{
			name: "WithContext",
			log:  func(l *logrus.Logger) { l.WithContext(reqCtx).Info("m") },
			want: logrus.Fields{
				logmiddleware.FieldRequestID: "req-h",
				logmiddleware.FieldMethod:    "PUT",
				logmiddleware.FieldPath:      "/h",
			},
		},
		{
			name: "WithContext and fields",
			log:  func(l *logrus.Logger) { l.WithContext(reqCtx).WithField("k", "v").Warn("m") },
			want: logrus.Fields{logmiddleware.FieldRequestID: "req-h", "k": "v"},
		},
		{
			name: "existing fields not overwritten",
			log: func(l *logrus.Logger) {
				l.WithContext(reqCtx).WithField(logmiddleware.FieldRequestID, "mine").Info("m")
			},
			want: logrus.Fields{logmiddleware.FieldRequestID: "mine", logmiddleware.FieldMethod: "PUT"},
		},
		{
			name: "no context",
			log:  func(l *logrus.Logger) { l.Info("m") },
		},
		{
			name: "context without request",
			log:  func(l *logrus.Logger) { l.WithContext(context.Background()).Info("m") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, hook := newTestLogger(logrus.DebugLevel)
			l.AddHook(ContextHook{})
			tt.log(l)
			e := hook.LastEntry()
			if e == nil {
				t.Fatal("no entry logged")
			}
			if tt.want == nil {
				if _, ok := e.Data[logmiddleware.FieldRequestID]; ok {
					t.Errorf("unexpected request fields: %v", e.Data)
				}
				return
			}
			for k, v := range tt.want {
				if e.Data[k] != v {
					t.Errorf("%s = %v, want %v", k, e.Data[k], v)
				}
			}
		})
	}
}

func TestContextHook_NilData(t *testing.T) {
	e := &logrus.Entry{Context: logmiddleware.ContextWithRequestInfo(context.Background(), hookInfo)}
	if err := (ContextHook{}).Fire(e); err != nil {
		t.Fatal(err)
	}
	if e.Data[logmiddleware.FieldRequestID] != "req-h" {
		t.Errorf("Data = %v", e.Data)
	}
}

func TestContextHook_Levels(t *testing.T) {
	if len(ContextHook{}.Levels()) != len(logrus.AllLevels) {
		t.Error("hook should fire at all levels")
	}
}

func TestContextHook_WithMiddleware(t *testing.T) {
	l, hook := newTestLogger(logrus.DebugLevel)
	l.AddHook(ContextHook{})
	shared := l.WithField("component", "repo") // e.g. a logger held by a dependency
	New(l, opts()...).Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		shared.WithContext(r.Context()).Info("from dependency")
		FromContext(r.Context()).Info("from FromContext")
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/m", nil))

	if len(hook.Entries) != 3 {
		t.Fatalf("got %d entries", len(hook.Entries))
	}
	for _, e := range hook.Entries {
		if e.Data[logmiddleware.FieldRequestID] != "req-1" || e.Data[logmiddleware.FieldPath] != "/m" {
			t.Errorf("%q missing request fields: %v", e.Message, e.Data)
		}
	}
}
