# Go Log Middleware

A lightweight HTTP middleware for Go applications using [logrus](https://github.com/sirupsen/logrus) that provides automatic request logging with unique request IDs. This library wraps HTTP handlers to automatically log incoming requests and outgoing responses with structured logging and request correlation.

## Features

- **Automatic Request Logging**: Logs all HTTP requests and responses
- **Unique Request IDs**: Generates UUID for each request for correlation
- **Request ID Header**: Adds `X-Request-Id` header to all responses
- **Structured Logging**: Uses logrus for consistent, structured log output
- **Request Context**: Injects request ID into context for downstream use
- **Performance Timing**: Measures and logs request duration
- **Zero Configuration**: Works out of the box with sensible defaults
- **Full Test Coverage**: Comprehensive unit tests with 100% coverage

## Installation

```bash
go get github.com/tpyle/log-middleware
```

## Quick Start

### Basic Usage

```go
package main

import (
    "log"
    "net/http"

    "github.com/tpyle/log-middleware"
    "github.com/sirupsen/logrus"
)

func main() {
    // Set logrus to debug level to see request logs
    logrus.SetLevel(logrus.DebugLevel)

    // Your application handler
    helloHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Access the request ID from context if needed
        if requestID := r.Context().Value("requestId"); requestID != nil {
            logrus.WithField("requestId", requestID).Info("Processing hello request")
        }
        w.Write([]byte("Hello World!"))
    })

    // Wrap your handler with the log middleware
    http.Handle("/", logmiddleware.LogMiddleware(helloHandler))
    http.Handle("/api/health", logmiddleware.LogMiddleware(http.HandlerFunc(healthHandler)))

    logrus.Info("Server starting on :8080")
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
    "github.com/gorilla/mux"
    "github.com/tpyle/log-middleware"
    "github.com/sirupsen/logrus"
)

func main() {
    logrus.SetLevel(logrus.DebugLevel)

    r := mux.NewRouter()
    r.Use(logmiddleware.LogMiddleware) // Apply middleware to all routes
    r.HandleFunc("/users/{id}", getUserHandler).Methods("GET")
    r.HandleFunc("/users", createUserHandler).Methods("POST")

    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

#### With Chi Router

```go
package main

import (
    "github.com/go-chi/chi/v5"
    "github.com/tpyle/log-middleware"
    "github.com/sirupsen/logrus"
)

func main() {
    logrus.SetLevel(logrus.DebugLevel)

    r := chi.NewRouter()
    r.Use(func(next http.Handler) http.Handler {
        return logmiddleware.LogMiddleware(next)
    })

    r.Get("/users/{id}", getUserHandler)
    r.Post("/users", createUserHandler)

    log.Fatal(http.ListenAndServe(":8080", r))
}
```

## Log Output

When you make requests to your application, you'll see structured logs like this:

```
DEBU[2025-06-29T10:30:45Z] HTTP request received  method=GET path=/users/123 requestId=550e8400-e29b-41d4-a716-446655440000
INFO[2025-06-29T10:30:45Z] Processing user request requestId=550e8400-e29b-41d4-a716-446655440000 userId=123
DEBU[2025-06-29T10:30:45Z] HTTP response sent     method=GET path=/users/123 requestId=550e8400-e29b-41d4-a716-446655440000 statusCode=200 duration=2.547ms
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
    requestID := r.Context().Value("requestId").(string)

    // Use in your own logging
    logrus.WithField("requestId", requestID).Info("Processing request")

    // Pass to downstream services, database queries, etc.
    user, err := userService.GetUser(r.Context(), userID)
    if err != nil {
        logrus.WithField("requestId", requestID).Error("Failed to get user")
        http.Error(w, "Internal Server Error", 500)
        return
    }

    // The requestId is automatically included in response headers
    json.NewEncoder(w).Encode(user)
}
```

## Configuration

### Log Level

The middleware logs at the `Debug` level by default. To see the request/response logs, ensure your logrus level is set appropriately:

```go
// See all request/response logs
logrus.SetLevel(logrus.DebugLevel)

// Only see your application logs, not request/response logs
logrus.SetLevel(logrus.InfoLevel)
```

### Custom Logrus Configuration

You can configure logrus as needed for your application:

```go
package main

import (
    "os"
    "github.com/sirupsen/logrus"
    "github.com/tpyle/log-middleware"
)

func main() {
    // JSON formatting
    logrus.SetFormatter(&logrus.JSONFormatter{})

    // Output to file
    file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err == nil {
        logrus.SetOutput(file)
    }

    // Set level
    logrus.SetLevel(logrus.DebugLevel)

    // Your handlers with middleware
    http.Handle("/", logmiddleware.LogMiddleware(yourHandler))
}
```

## Advanced Usage

### Extracting Request ID in Middleware Chain

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := r.Context().Value("requestId").(string)

        // Log authentication attempts with request ID
        logrus.WithField("requestId", requestID).Info("Authenticating request")

        // Your auth logic here...

        next.ServeHTTP(w, r)
    })
}

// Chain middlewares
func main() {
    handler := logmiddleware.LogMiddleware(
        authMiddleware(yourAppHandler),
    )
    http.Handle("/", handler)
}
```

### Using with Database Operations

```go
func getUserHandler(w http.ResponseWriter, r *http.Request) {
    requestID := r.Context().Value("requestId").(string)
    userID := r.URL.Query().Get("id")

    // Include request ID in database queries for tracing
    ctx := context.WithValue(r.Context(), "requestId", requestID)

    user, err := db.GetUserWithContext(ctx, userID)
    if err != nil {
        logrus.WithFields(logrus.Fields{
            "requestId": requestID,
            "userId": userID,
            "error": err,
        }).Error("Failed to fetch user")

        http.Error(w, "User not found", 404)
        return
    }

    logrus.WithFields(logrus.Fields{
        "requestId": requestID,
        "userId": userID,
    }).Info("Successfully fetched user")

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

The middleware has minimal performance overhead:

- **Request ID generation**: ~7333 ns/op
- **Hook execution**: ~147 ns/op
- **Memory allocation**: Minimal heap allocations per request

Benchmarks can be run with:
```bash
go test -bench=.
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
