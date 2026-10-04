// Command zerolog runs an HTTP server that logs requests with zerolog. It shows
// both ways of getting request fields into logs: the request-scoped logger from
// zerolog.Ctx, and a ContextHook on a shared logger used with .Ctx(ctx).
//
//	go run ./examples/zerolog
//	curl -i localhost:8080/hello?name=gopher
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/rs/zerolog"

	logmiddleware "github.com/tpyle/log-middleware/v3"
	"github.com/tpyle/log-middleware/v3/zerologmw"
)

func main() {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).
		Level(zerolog.DebugLevel).
		With().Timestamp().Logger()
	mw := zerologmw.New(&logger)
	store := &greetingStore{log: logger.Hook(zerologmw.ContextHook{})}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		// zerolog's own context API returns the request-scoped logger.
		zerolog.Ctx(r.Context()).Info().Str("name", name).Msg("greeting")
		store.record(r.Context(), name)
		_, _ = fmt.Fprintf(w, "Hello, %s! (request %s)\n", name, logmiddleware.GetRequestIdFromRequest(r))
	})

	logger.Info().Str("addr", ":8080").Msg("listening")
	if err := http.ListenAndServe(":8080", mw.Handler(mux)); err != nil {
		logger.Fatal().Err(err).Msg("server stopped")
	}
}

// greetingStore stands in for a dependency that holds its own logger. The
// ContextHook adds request fields because it passes the request context.
type greetingStore struct {
	log zerolog.Logger
}

func (s *greetingStore) record(ctx context.Context, name string) {
	s.log.Info().Ctx(ctx).Str("name", name).Msg("greeting recorded")
}
