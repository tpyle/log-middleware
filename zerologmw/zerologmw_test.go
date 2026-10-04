package zerologmw

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	logmiddleware "github.com/tpyle/log-middleware/v3"
)

func opts() []logmiddleware.Option {
	t := time.Unix(0, 0)
	return []logmiddleware.Option{
		logmiddleware.WithRequestIDGenerator(func() string { return "req-1" }),
		logmiddleware.WithClock(func() time.Time { t = t.Add(3 * time.Millisecond); return t }),
	}
}

func decode(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("bad JSON %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func TestResponseLog(t *testing.T) {
	var buf bytes.Buffer
	l := zerolog.New(&buf).Level(zerolog.DebugLevel)
	h := New(&l, opts()...).Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x?y=1", nil))

	lines := decode(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d lines: %s", len(lines), buf.String())
	}
	want := map[string]any{
		"level":                       "debug",
		"message":                     logmiddleware.ResponseMessage,
		logmiddleware.FieldRequestID:  "req-1",
		logmiddleware.FieldMethod:     "GET",
		logmiddleware.FieldPath:       "/x",
		logmiddleware.FieldStatusCode: float64(404),
		logmiddleware.FieldDuration:   float64(3), // zerolog default unit is ms
	}
	for k, v := range want {
		if lines[0][k] != v {
			t.Errorf("%s = %v, want %v", k, lines[0][k], v)
		}
	}
}

func TestContextLoggerHasRequestFields(t *testing.T) {
	var buf bytes.Buffer
	l := zerolog.New(&buf).Level(zerolog.InfoLevel)
	h := New(&l, opts()...).Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		zerolog.Ctx(r.Context()).Info().Str("user", "bob").Msg("in handler")
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/users", nil))

	lines := decode(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (debug filtered): %s", len(lines), buf.String())
	}
	m := lines[0]
	if m["message"] != "in handler" || m["user"] != "bob" || m[logmiddleware.FieldRequestID] != "req-1" ||
		m[logmiddleware.FieldMethod] != "POST" || m[logmiddleware.FieldPath] != "/users" {
		t.Errorf("unexpected line: %v", m)
	}
}

func TestNilUsesGlobalLogger(t *testing.T) {
	orig := log.Logger
	t.Cleanup(func() { log.Logger = orig })

	mw := New(nil, opts()...)
	var buf bytes.Buffer
	log.Logger = zerolog.New(&buf).Level(zerolog.DebugLevel)
	mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if !strings.Contains(buf.String(), `"requestId":"req-1"`) {
		t.Errorf("global logger not used: %q", buf.String())
	}
}

func TestRequestIDAvailable(t *testing.T) {
	l := zerolog.Nop()
	var got string
	New(&l, opts()...).Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = logmiddleware.GetRequestIdFromRequest(r)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if got != "req-1" {
		t.Errorf("request ID = %q", got)
	}
}

func BenchmarkZerolog(b *testing.B) {
	l := zerolog.Nop()
	h := New(&l).Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	for b.Loop() {
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func TestContextHook(t *testing.T) {
	reqCtx := logmiddleware.ContextWithRequestInfo(context.Background(),
		logmiddleware.RequestInfo{RequestID: "req-h", Method: "PUT", Path: "/h"})
	tests := []struct {
		name string
		log  func(l zerolog.Logger)
		want bool
	}{
		{"event Ctx", func(l zerolog.Logger) { l.Info().Ctx(reqCtx).Msg("m") }, true},
		{"logger Ctx", func(l zerolog.Logger) { cl := l.With().Ctx(reqCtx).Logger(); cl.Warn().Msg("m") }, true},
		{"no context", func(l zerolog.Logger) { l.Info().Msg("m") }, false},
		{"context without request", func(l zerolog.Logger) { l.Info().Ctx(context.Background()).Msg("m") }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.log(zerolog.New(&buf).Hook(ContextHook{}))
			lines := decode(t, &buf)
			if len(lines) != 1 {
				t.Fatalf("got %d lines: %s", len(lines), buf.String())
			}
			m := lines[0]
			if !tt.want {
				if _, ok := m[logmiddleware.FieldRequestID]; ok {
					t.Errorf("unexpected request fields: %v", m)
				}
				return
			}
			if m[logmiddleware.FieldRequestID] != "req-h" || m[logmiddleware.FieldMethod] != "PUT" ||
				m[logmiddleware.FieldPath] != "/h" {
				t.Errorf("missing request fields: %v", m)
			}
		})
	}
}

func TestContextHook_WithMiddleware(t *testing.T) {
	var buf bytes.Buffer
	base := zerolog.New(&buf).Level(zerolog.DebugLevel)
	shared := base.Hook(ContextHook{}) // e.g. a logger held by a dependency
	New(&base, opts()...).Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		shared.Info().Ctx(r.Context()).Msg("from dependency")
		zerolog.Ctx(r.Context()).Info().Msg("from zerolog.Ctx")
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/m", nil))

	out := strings.TrimSpace(buf.String())
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines: %s", len(lines), out)
	}
	for _, line := range lines {
		if strings.Count(line, `"requestId":"req-1"`) != 1 {
			t.Errorf("want exactly one requestId in %s", line)
		}
	}
}
