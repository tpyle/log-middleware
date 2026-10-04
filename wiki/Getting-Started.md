# Getting Started

## Install

```bash
go get github.com/tpyle/log-middleware/v3
```

v3 requires Go 1.26 or newer.

## Minimal server (slog)

```go
package main

import (
    "log/slog"
    "net/http"
    "os"

    logmiddleware "github.com/tpyle/log-middleware/v3"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
    mw := logmiddleware.NewSlog(logger)

    mux := http.NewServeMux()
    mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
        logmiddleware.SlogFromContext(r.Context()).Info("handling request")
        w.Write([]byte("ok"))
    })

    http.ListenAndServe(":8080", mw.Handler(mux))
}
```

Each request then produces output like this:

```json
{"level":"INFO","msg":"handling request","requestId":"5616…","method":"GET","path":"/"}
{"level":"DEBUG","msg":"HTTP response sent","requestId":"5616…","method":"GET","path":"/","statusCode":200,"duration":68979}
```

The response log is written at **debug** level. If your logger's level is
Info or higher, you will only see your own log lines. Those lines still carry
the request fields.

## Using it with routers

`Handler` has the standard `func(http.Handler) http.Handler` shape, so it can be
passed straight to routers that accept middleware:

```go
// chi
r := chi.NewRouter()
r.Use(mw.Handler)

// gorilla/mux
r := mux.NewRouter()
r.Use(mw.Handler)
```

## Reading the request ID

```go
id := logmiddleware.GetRequestIdFromRequest(r)
// or, deeper in the call stack:
id := logmiddleware.GetRequestIdFromContext(ctx)
```

Both return `""` when the request did not go through the middleware.
