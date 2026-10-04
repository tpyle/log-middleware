package logmiddleware

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

var testInfo = RequestInfo{RequestID: "req-1", Method: "GET", Path: "/p"}

func reqCtx() context.Context {
	return ContextWithRequestInfo(context.Background(), testInfo)
}

func newContextJSON(buf *bytes.Buffer) *slog.Logger {
	return slog.New(NewContextHandler(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
}

func TestContextHandler_AddsFields(t *testing.T) {
	tests := []struct {
		name string
		log  func(l *slog.Logger)
		want map[string]any // nil means the request fields must be absent
	}{
		{
			name: "InfoContext with request ctx",
			log:  func(l *slog.Logger) { l.InfoContext(reqCtx(), "m", "k", "v") },
			want: map[string]any{FieldRequestID: "req-1", FieldMethod: "GET", FieldPath: "/p", "k": "v"},
		},
		{
			name: "LogAttrs with request ctx",
			log:  func(l *slog.Logger) { l.LogAttrs(reqCtx(), slog.LevelWarn, "m") },
			want: map[string]any{FieldRequestID: "req-1"},
		},
		{
			name: "no context method",
			log:  func(l *slog.Logger) { l.Info("m") },
		},
		{
			name: "context without request",
			log:  func(l *slog.Logger) { l.InfoContext(context.Background(), "m") },
		},
		{
			name: "unrelated With attrs keep injection",
			log:  func(l *slog.Logger) { l.With("service", "api").InfoContext(reqCtx(), "m") },
			want: map[string]any{FieldRequestID: "req-1", "service": "api"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.log(newContextJSON(&buf))
			lines := decodeLines(t, &buf)
			if len(lines) != 1 {
				t.Fatalf("got %d lines: %s", len(lines), buf.String())
			}
			if tt.want == nil {
				if _, ok := lines[0][FieldRequestID]; ok {
					t.Errorf("unexpected request fields: %v", lines[0])
				}
				return
			}
			for k, v := range tt.want {
				if lines[0][k] != v {
					t.Errorf("%s = %v, want %v", k, lines[0][k], v)
				}
			}
		})
	}
}

func TestContextHandler_NoDuplicateWhenBound(t *testing.T) {
	tests := []struct {
		name string
		l    func(base *slog.Logger) *slog.Logger
		want int // occurrences of "requestId" in output
	}{
		{"bound with With", func(b *slog.Logger) *slog.Logger { return b.With(FieldRequestID, "bound") }, 1},
		{"bound then grouped", func(b *slog.Logger) *slog.Logger { return b.With(FieldRequestID, "bound").WithGroup("g") }, 1},
		{"bound inside group does not count", func(b *slog.Logger) *slog.Logger {
			return b.WithGroup("g").With(FieldRequestID, "nested")
		}, 2},
		{"empty group is ignored", func(b *slog.Logger) *slog.Logger { return b.WithGroup("").With(FieldRequestID, "bound") }, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.l(newContextJSON(&buf)).InfoContext(reqCtx(), "m")
			if got := strings.Count(buf.String(), `"requestId"`); got != tt.want {
				t.Errorf("requestId appears %d times, want %d: %s", got, tt.want, buf.String())
			}
		})
	}
}

func TestContextHandler_GroupPlacement(t *testing.T) {
	var buf bytes.Buffer
	newContextJSON(&buf).WithGroup("g").InfoContext(reqCtx(), "m")
	if !strings.Contains(buf.String(), `"g":{"requestId":"req-1"`) {
		t.Errorf("fields not placed in open group: %s", buf.String())
	}
}

func TestContextHandler_WithMiddlewareLogger(t *testing.T) {
	// The middleware's own logger binds requestId via With; combining it with a
	// ContextHandler must not duplicate fields in handler or response logs.
	var buf bytes.Buffer
	h := NewSlog(newContextJSON(&buf), fixedID("req-7")).Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		SlogFromContext(r.Context()).InfoContext(r.Context(), "via SlogFromContext")
		slog.New(NewContextHandler(slog.NewJSONHandler(&buf, nil))).InfoContext(r.Context(), "via plain logger")
	}))
	serve(t, h, "GET", "/x")

	lines := decodeLines(t, &buf)
	if len(lines) != 3 {
		t.Fatalf("got %d lines: %s", len(lines), buf.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if n := strings.Count(line, `"requestId":"req-7"`); n != 1 {
			t.Errorf("requestId appears %d times in %s", n, line)
		}
	}
}

func TestContextHandler_DefaultLogger(t *testing.T) {
	orig := slog.Default()
	t.Cleanup(func() { slog.SetDefault(orig) })

	var buf bytes.Buffer
	slog.SetDefault(newContextJSON(&buf))
	slog.InfoContext(reqCtx(), "global")
	if !strings.Contains(buf.String(), `"requestId":"req-1"`) {
		t.Errorf("default logger did not get request fields: %s", buf.String())
	}
}

func TestContextHandler_DoesNotMutateCallerRecord(t *testing.T) {
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "m", 0)
	r.AddAttrs(slog.String("a", "1"))
	h := NewContextHandler(slog.DiscardHandler)
	if err := h.Handle(reqCtx(), r); err != nil {
		t.Fatal(err)
	}
	if r.NumAttrs() != 1 {
		t.Errorf("caller record has %d attrs, want 1", r.NumAttrs())
	}
}

func TestNewContextHandler(t *testing.T) {
	t.Run("nil panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected panic")
			}
		}()
		NewContextHandler(nil)
	})
	t.Run("no double wrap", func(t *testing.T) {
		ch := NewContextHandler(slog.DiscardHandler)
		if NewContextHandler(ch) != ch {
			t.Error("wrapping a ContextHandler should return it")
		}
	})
	t.Run("unwrap", func(t *testing.T) {
		if NewContextHandler(slog.DiscardHandler).Unwrap() != slog.DiscardHandler {
			t.Error("Unwrap returned wrong handler")
		}
	})
	t.Run("WithGroup empty returns receiver", func(t *testing.T) {
		ch := NewContextHandler(slog.DiscardHandler)
		if ch.WithGroup("") != slog.Handler(ch) {
			t.Error("expected receiver")
		}
	})
	t.Run("WithAttrs empty returns receiver", func(t *testing.T) {
		ch := NewContextHandler(slog.DiscardHandler)
		if ch.WithAttrs(nil) != slog.Handler(ch) {
			t.Error("expected receiver")
		}
	})
}

func TestContextHandler_DelegatesEnabledAndErrors(t *testing.T) {
	inner := &stubHandler{enabled: false, err: errors.New("boom")}
	h := NewContextHandler(inner)
	if h.Enabled(context.Background(), slog.LevelError) {
		t.Error("Enabled should delegate")
	}
	if err := h.Handle(reqCtx(), slog.Record{}); err == nil || err.Error() != "boom" {
		t.Errorf("Handle error = %v, want boom", err)
	}
}

type stubHandler struct {
	enabled bool
	err     error
}

func (s *stubHandler) Enabled(context.Context, slog.Level) bool  { return s.enabled }
func (s *stubHandler) Handle(context.Context, slog.Record) error { return s.err }
func (s *stubHandler) WithAttrs([]slog.Attr) slog.Handler        { return s }
func (s *stubHandler) WithGroup(string) slog.Handler             { return s }
