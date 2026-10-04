// Command slog runs an HTTP server that logs requests with the standard
// library's log/slog. It shows both ways of getting request fields into logs:
// the request-scoped logger from SlogFromContext, and a ContextHandler that
// adds them to any slog call made with the request context.
//
//	go run ./examples/slog
//	curl -i localhost:8080/hello?name=gopher
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	logmiddleware "github.com/tpyle/log-middleware/v3"
)

func main() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(logmiddleware.NewContextHandler(handler))
	slog.SetDefault(logger)
	mw := logmiddleware.NewSlog(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		// The context logger already carries requestId, method and path.
		logmiddleware.SlogFromContext(r.Context()).Info("greeting", "name", name)
		recordGreeting(r.Context(), name)
		_, _ = fmt.Fprintf(w, "Hello, %s! (request %s)\n", name, logmiddleware.GetRequestIdFromRequest(r))
	})

	logger.Info("listening", "addr", ":8080")
	if err := http.ListenAndServe(":8080", mw.Handler(mux)); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// recordGreeting stands in for code that does not import the middleware. The
// ContextHandler adds requestId, method and path because it logs with the
// request context.
func recordGreeting(ctx context.Context, name string) {
	slog.InfoContext(ctx, "greeting recorded", "name", name)
}
