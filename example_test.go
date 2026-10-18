package logmiddleware_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

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

func ExampleWithRequestIDFromRequest() {
	// Prefix a random UUID with the HTTP method. Any function of the request
	// works; see RequestIDFromHeader for reusing an incoming header.
	byMethod := logmiddleware.RequestIDGeneratorFunc(func(r *http.Request) string {
		return strings.ToLower(r.Method) + "-" + logmiddleware.UUIDRequestID(r)
	})

	mw := logmiddleware.NewSlog(slog.New(slog.DiscardHandler), logmiddleware.WithRequestIDFromRequest(byMethod))
	handler := mw.Handler(http.NotFoundHandler())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	fmt.Println(strings.HasPrefix(rec.Header().Get(logmiddleware.RequestIDHeader), "get-"))
	// Output:
	// true
}

func ExampleRequestIDFromHeader() {
	// Reuse a valid X-Request-Id from an upstream proxy; otherwise use the
	// fallback (here a fixed value; use UUIDRequestID in real code).
	gen := logmiddleware.RequestIDFromHeader(logmiddleware.RequestIDHeader,
		logmiddleware.RequestIDGeneratorFunc(func(*http.Request) string { return "generated" }))

	mw := logmiddleware.NewSlog(exampleSlog(), logmiddleware.WithRequestIDFromRequest(gen))
	handler := mw.Handler(http.NotFoundHandler())

	for _, incoming := range []string{"from-proxy", "bad id\nlevel=ERROR"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(logmiddleware.RequestIDHeader, incoming)
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	// Output:
	// level=DEBUG msg="HTTP response sent" requestId=from-proxy method=GET path=/ statusCode=404
	// level=DEBUG msg="HTTP response sent" requestId=generated method=GET path=/ statusCode=404
}

func ExampleValidRequestID() {
	for _, id := range []string{"0f8fad5b-d9cb-469f-a165-70867728950e", "a b", ""} {
		fmt.Printf("%q %v\n", id, logmiddleware.ValidRequestID(id))
	}
	// Output:
	// "0f8fad5b-d9cb-469f-a165-70867728950e" true
	// "a b" false
	// "" false
}
