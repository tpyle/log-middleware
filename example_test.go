package logmiddleware_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"

	logmiddleware "github.com/tpyle/log-middleware/v3"
)

// exampleSlog returns a text logger without timestamps so output is stable.
func exampleSlog() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey || a.Key == logmiddleware.FieldDuration {
				return slog.Attr{}
			}
			return a
		},
	}))
}

func ExampleNewSlog() {
	mw := logmiddleware.NewSlog(exampleSlog(),
		logmiddleware.WithRequestIDGenerator(func() string { return "req-1" }))

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logmiddleware.SlogFromContext(r.Context()).Info("hello")
		w.WriteHeader(http.StatusCreated)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/greet", nil))
	// Output:
	// level=INFO msg=hello requestId=req-1 method=POST path=/greet
	// level=DEBUG msg="HTTP response sent" requestId=req-1 method=POST path=/greet statusCode=201
}

// printLogger is a minimal custom Logger that prints to stdout.
type printLogger struct{}

func (printLogger) Start(ctx context.Context, req logmiddleware.RequestInfo) (context.Context, logmiddleware.RequestLogger) {
	return ctx, printRequest{req}
}

type printRequest struct{ req logmiddleware.RequestInfo }

func (p printRequest) Finish(resp logmiddleware.ResponseInfo) {
	fmt.Println(p.req.Method, p.req.Path, resp.StatusCode)
}

func ExampleNew() {
	mw := logmiddleware.New(printLogger{})
	handler := mw.Handler(http.NotFoundHandler())
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/missing", nil))
	// Output:
	// GET /missing 404
}

// loadUser stands in for code elsewhere in an application that logs with the
// plain slog API and knows nothing about the middleware.
func loadUser(ctx context.Context, id string) {
	slog.InfoContext(ctx, "loading user", "userId", id)
}

func ExampleNewContextHandler() {
	orig := slog.Default()
	defer slog.SetDefault(orig)

	// Install once at startup.
	slog.SetDefault(slog.New(logmiddleware.NewContextHandler(exampleSlog().Handler())))

	mw := logmiddleware.NewSlog(nil, // nil uses slog.Default()
		logmiddleware.WithRequestIDGenerator(func() string { return "req-1" }))

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loadUser(r.Context(), "42")
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/users/42", nil))
	// Output:
	// level=INFO msg="loading user" userId=42 requestId=req-1 method=GET path=/users/42
	// level=DEBUG msg="HTTP response sent" requestId=req-1 method=GET path=/users/42 statusCode=200
}
