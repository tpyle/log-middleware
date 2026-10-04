// Command logrus runs an HTTP server that logs requests with logrus. It shows
// both ways of getting request fields into logs: the entry from
// logrusmw.FromContext, and a ContextHook that adds them to any entry logged
// with WithContext.
//
//	go run ./examples/logrus
//	curl -i localhost:8080/hello?name=gopher
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"

	logmiddleware "github.com/tpyle/log-middleware/v3"
	"github.com/tpyle/log-middleware/v3/logrusmw"
)

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.AddHook(logrusmw.ContextHook{})
	mw := logrusmw.New(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		logrusmw.FromContext(r.Context()).WithField("name", name).Info("greeting")
		recordGreeting(logger, r.Context(), name)
		_, _ = fmt.Fprintf(w, "Hello, %s! (request %s)\n", name, logmiddleware.GetRequestIdFromRequest(r))
	})

	logger.WithField("addr", ":8080").Info("listening")
	if err := http.ListenAndServe(":8080", mw.Handler(mux)); err != nil {
		logger.WithError(err).Fatal("server stopped")
	}
}

// recordGreeting stands in for code that does not import the middleware. The
// ContextHook adds request fields because it logs with the request context.
func recordGreeting(logger *logrus.Logger, ctx context.Context, name string) {
	logger.WithContext(ctx).WithField("name", name).Info("greeting recorded")
}
