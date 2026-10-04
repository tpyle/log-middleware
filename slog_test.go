package logmiddleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newJSONSlog(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level}))
}

// decodeLines parses newline-delimited JSON log output.
func decodeLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
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

func TestSlog_ResponseLog(t *testing.T) {
	var buf bytes.Buffer
	lm := NewSlog(newJSONSlog(&buf, slog.LevelDebug), fixedID("req-1"), WithClock(steppingClock(5*time.Millisecond)))
	h := lm.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	serve(t, h, "PUT", "/brew?x=1")

	lines := decodeLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %s", len(lines), buf.String())
	}
	want := map[string]any{
		"level":         "DEBUG",
		"msg":           ResponseMessage,
		FieldRequestID:  "req-1",
		FieldMethod:     "PUT",
		FieldPath:       "/brew",
		FieldStatusCode: float64(http.StatusTeapot),
		FieldDuration:   float64(5 * time.Millisecond),
	}
	for k, v := range want {
		if lines[0][k] != v {
			t.Errorf("%s = %v, want %v", k, lines[0][k], v)
		}
	}
}

func TestSlog_HandlerLoggerHasRequestFields(t *testing.T) {
	var buf bytes.Buffer
	h := NewSlog(newJSONSlog(&buf, slog.LevelInfo), fixedID("req-2")).
		Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			SlogFromContext(r.Context()).Info("in handler", "user", "bob")
		}))
	serve(t, h, "GET", "/users")

	lines := decodeLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (debug response log should be filtered): %s", len(lines), buf.String())
	}
	l := lines[0]
	if l["msg"] != "in handler" || l["user"] != "bob" || l[FieldRequestID] != "req-2" ||
		l[FieldMethod] != "GET" || l[FieldPath] != "/users" {
		t.Errorf("unexpected log line: %v", l)
	}
}

func TestSlog_NilUsesCurrentDefault(t *testing.T) {
	orig := slog.Default()
	t.Cleanup(func() { slog.SetDefault(orig) })

	lm := NewSlog(nil, fixedID("req-3"))

	var buf bytes.Buffer
	slog.SetDefault(newJSONSlog(&buf, slog.LevelDebug))
	serve(t, lm.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})), "GET", "/")

	if !strings.Contains(buf.String(), `"requestId":"req-3"`) {
		t.Errorf("default logger set after construction was not used: %q", buf.String())
	}
}

func TestSlog_FinishUsesRequestContext(t *testing.T) {
	rec := &ctxRecordingHandler{}
	lm := NewSlog(slog.New(rec), fixedID("req-4"))
	serve(t, lm.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})), "GET", "/")
	if rec.lastCtx == nil || GetRequestIdFromContext(rec.lastCtx) != "req-4" {
		t.Error("slog handler did not receive the request context")
	}
}

type ctxRecordingHandler struct{ lastCtx context.Context }

func (h *ctxRecordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *ctxRecordingHandler) Handle(ctx context.Context, _ slog.Record) error {
	h.lastCtx = ctx
	return nil
}
func (h *ctxRecordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *ctxRecordingHandler) WithGroup(string) slog.Handler      { return h }

func TestSlogFromContext(t *testing.T) {
	custom := slog.New(slog.DiscardHandler)
	tests := []struct {
		name string
		ctx  context.Context
		want *slog.Logger
	}{
		{"stored", ContextWithSlog(context.Background(), custom), custom},
		{"absent", context.Background(), slog.Default()},
		{"nil ctx", nil, slog.Default()},
		{"nil logger stored", ContextWithSlog(context.Background(), nil), slog.Default()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SlogFromContext(tt.ctx); got != tt.want {
				t.Errorf("got %p, want %p", got, tt.want)
			}
		})
	}
}

func BenchmarkSlog(b *testing.B) {
	h := NewSlog(slog.New(slog.DiscardHandler)).Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	for b.Loop() {
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
}
