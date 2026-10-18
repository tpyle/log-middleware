// Command requestid shows how to control request IDs. It reuses a valid
// X-Request-Id header set by an upstream proxy or caller, and otherwise
// generates a random UUID. Invalid incoming IDs (for example, ones containing
// spaces or newlines) are replaced rather than logged.
//
//	go run ./examples/requestid
//	curl -i localhost:8080/hello
//	curl -i -H 'X-Request-Id: trace-abc123' localhost:8080/hello
//	curl -i -H 'X-Request-Id: not valid' localhost:8080/hello
package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	logmiddleware "github.com/tpyle/log-middleware/v3"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ids := logmiddleware.RequestIDFromHeader(logmiddleware.RequestIDHeader,
		logmiddleware.RequestIDGeneratorFunc(logmiddleware.UUIDRequestID))
	mw := logmiddleware.NewSlog(logger, logmiddleware.WithRequestIDFromRequest(ids))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		logmiddleware.SlogFromContext(r.Context()).Info("hello")
		_, _ = w.Write([]byte("Hello!\n"))
	})

	log.Fatal(http.ListenAndServe(":8080", mw.Handler(mux)))
}
