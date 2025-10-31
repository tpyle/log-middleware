# Go Log Middleware v2

A lightweight HTTP middleware for Go applications using [zerolog](https://github.com/rs/zerolog) that provides automatic request logging with unique request IDs. This library wraps HTTP handlers to automatically log incoming requests and outgoing responses with structured logging and request correlation.

## ⚠️ Version 2.0 Breaking Changes

Version 2.0 introduces significant breaking changes:
- **Switched from logrus to zerolog** for better performance and structured logging
- **New API**: Middleware is now instantiated with `NewLogMiddleware(logger).Handler(next)` instead of a simple function wrapper
- **Updated import path**: Now `github.com/tpyle/log-middleware/v2`
- **Logger injection**: You must provide your own zerolog logger instance

See the [Migration Guide](#migration-from-v1) below for upgrading from v1.

## Features

- **Automatic Request Logging**: Logs all HTTP requests and responses
- **Unique Request IDs**: Generates UUID for each request for correlation
- **Request ID Header**: Adds `X-Request-Id` header to all responses
- **Structured Logging**: Uses zerolog for high-performance, structured log output
- **Request Context**: Injects request ID into context for downstream use
- **Performance Timing**: Measures and logs request duration
- **Zero Configuration**: Works out of the box with sensible defaults
- **Full Test Coverage**: Comprehensive unit tests with 100% coverage

## Installation

```bash
go get github.com/tpyle/log-middleware/v2
```

For v1 (using logrus):
```bash
go get github.com/tpyle/log-middleware@v1
```

## Quick Start

### Basic Usage

```go
package main

import (
    "log"
    "net/http"
    "os"

    logmiddleware "github.com/tpyle/log-middleware/v2"
    "github.com/rs/zerolog"
)

func main() {
    // Create a zerolog logger
    logger := zerolog.New(os.Stdout).
        Level(zerolog.DebugLevel).
        With().
        Timestamp().
        Logger()

    // Create the middleware instance
    logMiddleware := logmiddleware.NewLogMiddleware(&logger)

    // Your application handler
    helloHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Access the request ID from context using the helper function
        requestID := logmiddleware.GetRequestIdFromRequest(r)

        // Get the logger from context (includes request ID automatically)
        ctxLogger := zerolog.Ctx(r.Context())
        ctxLogger.Info().Msg("Processing hello request")

        w.Write([]byte("Hello World!"))
    })

    // Wrap your handlers with the log middleware
    http.Handle("/", logMiddleware.Handler(helloHandler))
    http.Handle("/api/health", logMiddleware.Handler(http.HandlerFunc(healthHandler)))

    logger.Info().Msg("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}
```

### Using with HTTP Router Libraries

#### With Gorilla Mux

```go
package main

import (
    "os"
    "log"
    "net/http"

    "github.com/gorilla/mux"
    logmiddleware "github.com/tpyle/log-middleware/v2"
    "github.com/rs/zerolog"
)

func main() {
    // Create logger
    logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).With().Timestamp().Logger()
    logMiddleware := logmiddleware.NewLogMiddleware(&logger)

    r := mux.NewRouter()
    r.Use(func(next http.Handler) http.Handler {
        return logMiddleware.Handler(next)
    })
    r.HandleFunc("/users/{id}", getUserHandler).Methods("GET")
    r.HandleFunc("/users", createUserHandler).Methods("POST")

    log.Fatal(http.ListenAndServe(":8080", r))
}
```

#### With Chi Router

```go
package main

import (
    "os"
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"
    logmiddleware "github.com/tpyle/log-middleware/v2"
    "github.com/rs/zerolog"
)

func main() {
    // Create logger
    logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).With().Timestamp().Logger()
    logMiddleware := logmiddleware.NewLogMiddleware(&logger)

    r := chi.NewRouter()
    r.Use(func(next http.Handler) http.Handler {
        return logMiddleware.Handler(next)
    })

    r.Get("/users/{id}", getUserHandler)
    r.Post("/users", createUserHandler)

    log.Fatal(http.ListenAndServe(":8080", r))
}
```

## Log Output

When you make requests to your application, you'll see structured JSON logs like this:

```json
{"level":"debug","requestId":"550e8400-e29b-41d4-a716-446655440000","method":"GET","path":"/users/123","statusCode":200,"duration":2.547,"time":"2025-10-31T10:30:45Z","message":"HTTP response sent"}
{"level":"info","requestId":"550e8400-e29b-41d4-a716-446655440000","userId":"123","time":"2025-10-31T10:30:45Z","message":"Processing user request"}
```

Or with console formatting:
```
10:30:45 DBG HTTP response sent requestId=550e8400-e29b-41d4-a716-446655440000 method=GET path=/users/123 statusCode=200 duration=2.547ms
10:30:45 INF Processing user request requestId=550e8400-e29b-41d4-a716-446655440000 userId=123
```

## Middleware Behavior

### Automatic Features

- **Request ID Generation**: Each request gets a unique UUID
- **Request ID Injection**: Available in request context as `"requestId"`
- **Response Header**: `X-Request-Id` header added to all responses
- **Request Logging**: Logs method, path, and request ID when request starts
- **Response Logging**: Logs method, path, status code, duration, and request ID when request completes
- **Context Preservation**: Original request context is preserved and extended

### Log Fields

**Request Log Fields:**
- `method`: HTTP method (GET, POST, etc.)
- `path`: Request URL path
- `requestId`: Unique request identifier

**Response Log Fields:**
- `method`: HTTP method
- `path`: Request URL path
- `statusCode`: HTTP response status code
- `duration`: Request processing time
- `requestId`: Unique request identifier

## Accessing Request ID

The request ID is available in the request context and can be accessed in your handlers:

```go
func myHandler(w http.ResponseWriter, r *http.Request) {
    // Get request ID using helper function
    requestID := logmiddleware.GetRequestIdFromRequest(r)

    // Get the logger from context (already includes requestId field)
    logger := zerolog.Ctx(r.Context())

    // Use in your own logging - requestId is automatically included
    logger.Info().Msg("Processing request")

    // Pass context to downstream services, database queries, etc.
    user, err := userService.GetUser(r.Context(), userID)
    if err != nil {
        logger.Error().Err(err).Msg("Failed to get user")
        http.Error(w, "Internal Server Error", 500)
        return
    }

    // Log success with additional fields
    logger.Info().Str("userId", user.ID).Msg("Successfully retrieved user")

    // The requestId is automatically included in response headers
    json.NewEncoder(w).Encode(user)
}
```

## Configuration

### Log Level

The middleware logs at the `Debug` level by default. To see the request/response logs, ensure your zerolog level is set appropriately:

```go
// See all request/response logs
logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel)

// Only see your application logs, not request/response logs
logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel)
```

### Custom Zerolog Configuration

You can configure zerolog as needed for your application:

```go
package main

import (
    "os"
    logmiddleware "github.com/tpyle/log-middleware/v2"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

func main() {
    // JSON formatting (default)
    logger := zerolog.New(os.Stdout).
        Level(zerolog.DebugLevel).
        With().
        Timestamp().
        Logger()

    // Console formatting for development
    logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
        Level(zerolog.DebugLevel).
        With().
        Timestamp().
        Logger()

    // Output to file
    file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err == nil {
        logger = zerolog.New(file).Level(zerolog.DebugLevel).With().Timestamp().Logger()
    }

    // Create middleware with your logger
    logMiddleware := logmiddleware.NewLogMiddleware(&logger)

    // Your handlers with middleware
    http.Handle("/", logMiddleware.Handler(yourHandler))
}
```

## Advanced Usage

### Extracting Request ID in Middleware Chain

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Get logger from context (includes requestId automatically)
        logger := zerolog.Ctx(r.Context())

        // Log authentication attempts with request ID
        logger.Info().Msg("Authenticating request")

        // Your auth logic here...

        next.ServeHTTP(w, r)
    })
}

// Chain middlewares
func main() {
    logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).With().Timestamp().Logger()
    logMiddleware := logmiddleware.NewLogMiddleware(&logger)

    handler := logMiddleware.Handler(
        authMiddleware(yourAppHandler),
    )
    http.Handle("/", handler)
}
```

### Using with Database Operations

```go
func getUserHandler(w http.ResponseWriter, r *http.Request) {
    requestID := logmiddleware.GetRequestIdFromRequest(r)
    userID := r.URL.Query().Get("id")

    // Get logger from context (already includes requestId)
    logger := zerolog.Ctx(r.Context())

    // Pass context to database queries for tracing
    user, err := db.GetUserWithContext(r.Context(), userID)
    if err != nil {
        logger.Error().
            Err(err).
            Str("userId", userID).
            Msg("Failed to fetch user")

        http.Error(w, "User not found", 404)
        return
    }

    logger.Info().
        Str("userId", userID).
        Msg("Successfully fetched user")

    json.NewEncoder(w).Encode(user)
}
```

## Testing

Run the test suite:

```bash
go test -v
```

Run with coverage:

```bash
go test -cover
```

Generate coverage report:

```bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

The test suite includes:
- Unit tests for all middleware components
- Integration tests for real HTTP scenarios
- Edge case testing (panics, malformed requests, etc.)
- Concurrency testing for thread safety
- Performance benchmarks

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Ensure tests pass (`go test -v`)
6. Commit your changes (`git commit -m 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

## Performance

The middleware has minimal performance overhead with zerolog's high-performance logging:

- **Complete middleware overhead**: ~3274 ns/op (v2.0 with zerolog)
- **Memory allocation**: Minimal heap allocations per request
- **Zero-allocation logging**: Zerolog's design minimizes garbage collection pressure

Performance improvements in v2.0:
- ~55% faster than v1.0 with logrus
- Lower memory footprint
- Better CPU cache efficiency

Benchmarks can be run with:
```bash
go test -bench=.
```

## Migration from v1

### Import Path Changes

**v1:**
```go
import "github.com/tpyle/log-middleware"
```

**v2:**
```go
import logmiddleware "github.com/tpyle/log-middleware/v2"
```

### API Changes

**v1 - Function-based:**
```go
import "github.com/sirupsen/logrus"

// Setup logrus globally
logrus.SetLevel(logrus.DebugLevel)

// Use middleware directly as function
http.Handle("/", logmiddleware.LogMiddleware(handler))
```

**v2 - Instance-based with dependency injection:**
```go
import "github.com/rs/zerolog"

// Create logger instance
logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).With().Timestamp().Logger()

// Create middleware instance
logMiddleware := logmiddleware.NewLogMiddleware(&logger)

// Use middleware Handler method
http.Handle("/", logMiddleware.Handler(handler))
```

### Logging API Changes

**v1 - Logrus fields:**
```go
func handler(w http.ResponseWriter, r *http.Request) {
    requestID := r.Context().Value("requestId").(string)
    logrus.WithField("requestId", requestID).Info("Processing request")
}
```

**v2 - Zerolog context:**
```go
func handler(w http.ResponseWriter, r *http.Request) {
    // RequestID automatically included via context logger
    logger := zerolog.Ctx(r.Context())
    logger.Info().Msg("Processing request")

    // Or use helper function to get requestID
    requestID := logmiddleware.GetRequestIdFromRequest(r)
}
```

### New Helper Functions in v2

```go
// Get request ID from request
requestID := logmiddleware.GetRequestIdFromRequest(r)

// Get request ID from context
requestID := logmiddleware.GetRequestIdFromContext(ctx)
```

### Benefits of Migration

- **Performance**: ~55% faster logging with zerolog
- **Memory efficiency**: Lower allocations and GC pressure
- **Better structure**: Dependency injection allows better testing
- **Type safety**: Structured logging with compile-time field validation
- **Context integration**: Logger automatically injected into request context

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
