// Command custom shows how to plug any logger into the middleware by
// implementing logmiddleware.Logger. This one writes access-log style lines
// with the standard library's log package.
//
//	go run ./examples/custom
//	curl -i localhost:8080/hello
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	logmiddleware "github.com/tpyle/log-middleware/v3"
)

// accessLogger implements logmiddleware.Logger.
type accessLogger struct {
	out *log.Logger
}

func (a *accessLogger) Start(ctx context.Context, req logmiddleware.RequestInfo) (context.Context, logmiddleware.RequestLogger) {
	// A real adapter would also attach a request-scoped logger to ctx here.
	return ctx, &accessRequest{out: a.out, req: req}
}

type accessRequest struct {
	out *log.Logger
	req logmiddleware.RequestInfo
}

func (a *accessRequest) Finish(resp logmiddleware.ResponseInfo) {
	a.out.Printf("%s %s %s -> %d in %s", a.req.RequestID, a.req.Method, a.req.Path, resp.StatusCode, resp.Duration)
}

func main() {
	logger := &accessLogger{out: log.New(os.Stdout, "access: ", log.LstdFlags)}
	mw := logmiddleware.New(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "Hello!")
	})

	log.Fatal(http.ListenAndServe(":8080", mw.Handler(mux)))
}
