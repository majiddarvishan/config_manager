# HTTP Server Documentation

## Overview

The `HttpServer` provides a flexible HTTP interface for configuration management. It supports three main usage patterns:

1. **Standalone** - Let HttpServer create and manage the server
2. **User-Provided** - Provide your own `http.Server` instance
3. **Handler-Only** - Get the handler and integrate it yourself

## Quick Start

### Simplest Usage (Default Server)

```go
manager, _ := config.NewManager(source)
httpServer, _ := config.NewHttpServer(manager)

// Starts on localhost:8080
httpServer.Start()
```

### With Configuration Options

```go
httpServer, _ := config.NewHttpServer(
    manager,
    config.WithAddress("0.0.0.0"),
    config.WithPort(9090),
    config.WithAPIKey("secret-key"),
)

httpServer.Start()
```

## Configuration Options

### WithAddress(address string)

Sets the server address.

```go
config.WithAddress("0.0.0.0")  // Listen on all interfaces
config.WithAddress("127.0.0.1") // Localhost only
```

### WithPort(port int)

Sets the server port (1-65535).

```go
config.WithPort(8080)
config.WithPort(9090)
```

### WithAPIKey(apiKey string)

Sets API key for authentication. Requests must include `X-API-Key` header.

```go
config.WithAPIKey("my-secret-key")
```

**Security Note**: Uses constant-time comparison to prevent timing attacks.

### WithServer(server *http.Server)

Provides your own `http.Server` instance. The handler will be set automatically.

```go
myServer := &http.Server{
    Addr:           ":8080",
    ReadTimeout:    30 * time.Second,
    WriteTimeout:   30 * time.Second,
    IdleTimeout:    120 * time.Second,
    MaxHeaderBytes: 1 << 20,
}

httpServer, _ := config.NewHttpServer(
    manager,
    config.WithServer(myServer),
)
```

## Usage Patterns

### Pattern 1: Standalone Server

Let HttpServer create and manage everything.

```go
httpServer, _ := config.NewHttpServer(
    manager,
    config.WithAddress("0.0.0.0"),
    config.WithPort(8080),
    config.WithAPIKey("secret"),
)

// Starts the server
if err := httpServer.Start(); err != nil {
    log.Fatal(err)
}
```

**When to use**: Simple deployments, microservices, config-only servers.

### Pattern 2: User-Provided Server

You create the server, HttpServer manages the handler.

```go
myServer := &http.Server{
    Addr:         ":8080",
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 30 * time.Second,
}

httpServer, _ := config.NewHttpServer(
    manager,
    config.WithServer(myServer),
)

// Uses your server
httpServer.Start()
```

**When to use**: Need custom server settings, custom error handling, production deployments.

**Benefits**:
- Full control over server configuration
- Custom timeouts and limits
- Custom TLS settings
- Custom error logging

### Pattern 3: Handler-Only Integration

Get the handler and integrate it yourself.

```go
httpServer, _ := config.NewHttpServer(manager)
handler := httpServer.GetHandler()

// Use with your own router
mux := http.NewServeMux()
mux.HandleFunc("/", homeHandler)
mux.Handle("/config/", http.StripPrefix("/config", handler))

http.ListenAndServe(":8080", mux)
```

**When to use**: Existing application, custom routing, middleware integration.

### Pattern 4: Route Integration

Add config routes to existing ServeMux.

```go
mux := http.NewServeMux()
mux.HandleFunc("/", homeHandler)
mux.HandleFunc("/api/users", usersHandler)

httpServer, _ := config.NewHttpServer(manager)
httpServer.SetupRoutes(mux) // Adds /config and /health

server := &http.Server{
    Addr:    ":8080",
    Handler: mux,
}
server.ListenAndServe()
```

**When to use**: Adding config management to existing service.

## API Methods

### NewHttpServer(manager, opts...)

Creates a new HTTP server.

```go
httpServer, err := config.NewHttpServer(
    manager,
    config.WithAddress("0.0.0.0"),
    config.WithPort(8080),
)
```

### NewHttpServerFromNode(manager, node)

Legacy constructor from config Node. **Deprecated** - use `NewHttpServer` with options.

```go
httpServer, err := config.NewHttpServerFromNode(manager, configNode)
```

### Start() error

Starts the HTTP server. Blocks until server stops.

```go
if err := httpServer.Start(); err != nil {
    log.Fatal(err)
}
```

### StartTLS(certFile, keyFile string) error

Starts HTTPS server with TLS.

```go
if err := httpServer.StartTLS("cert.pem", "key.pem"); err != nil {
    log.Fatal(err)
}
```

### Shutdown(ctx context.Context) error

Gracefully shuts down the server.

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := httpServer.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

### GetHandler() http.Handler

Returns the HTTP handler with CORS configured.

```go
handler := httpServer.GetHandler()
// Use handler in your own server/router
```

### SetupRoutes(mux *http.ServeMux)

Adds config routes to existing ServeMux.

```go
mux := http.NewServeMux()
httpServer.SetupRoutes(mux)
// Now mux has /config and /health endpoints
```

### GetServer() *http.Server

Returns underlying http.Server for further configuration.

```go
server := httpServer.GetServer()
if server != nil {
    server.MaxHeaderBytes = 2 << 20
}
```

## Endpoints

### GET /config

Returns current configuration state.

**Request:**
```bash
curl -H "X-API-Key: secret" http://localhost:8080/config
```

**Response:**
```json
{
  "success": true,
  "data": {
    "config": { ... },
    "schema": { ... },
    "modifiable_paths": {
      "insertable": ["/items"],
      "removable": ["/items"],
      "replaceable": ["/settings/timeout"]
    },
    "version": 42
  }
}
```

### POST /config

Modifies configuration.

**Operations:**
- `insert` - Insert into array
- `remove` - Remove from array
- `replace` - Replace value

**Insert Example:**
```bash
curl -X POST http://localhost:8080/config \
  -H "X-API-Key: secret" \
  -H "Content-Type: application/json" \
  -d '{
    "op": "insert",
    "path": "/items",
    "index": 0,
    "value": {"name": "new item"},
    "version": 42
  }'
```

**Remove Example:**
```bash
curl -X POST http://localhost:8080/config \
  -H "X-API-Key: secret" \
  -d '{
    "op": "remove",
    "path": "/items",
    "index": 0,
    "version": 42
  }'
```

**Replace Example:**
```bash
curl -X POST http://localhost:8080/config \
  -H "X-API-Key: secret" \
  -d '{
    "op": "replace",
    "path": "/settings/timeout",
    "value": 60,
    "version": 42
  }'
```

**Response (Success):**
```json
{
  "success": true,
  "data": {
    "config": { ... },
    "version": 43
  }
}
```

**Response (Conflict):**
```json
{
  "success": false,
  "error": {
    "code": 409,
    "message": "version mismatch: expected 42, current 43"
  }
}
```

### GET /health

Health check endpoint.

**Request:**
```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
  "status": "ok"
}
```

## Authentication

### API Key Authentication

Enabled when API key is configured:

```go
httpServer, _ := config.NewHttpServer(
    manager,
    config.WithAPIKey("your-secret-key"),
)
```

**Include in requests:**
```bash
curl -H "X-API-Key: your-secret-key" http://localhost:8080/config
```

**Security Features:**
- Uses SHA256 hash storage
- Constant-time comparison (prevents timing attacks)
- No auth required if no key configured

### No Authentication

If no API key is set, all requests are allowed:

```go
httpServer, _ := config.NewHttpServer(manager)
// No authentication required
```

## Advanced Examples

### Production Setup with Graceful Shutdown

```go
// Create production server
myServer := &http.Server{
    Addr:              ":8080",
    ReadTimeout:       15 * time.Second,
    WriteTimeout:      15 * time.Second,
    IdleTimeout:       60 * time.Second,
    ReadHeaderTimeout: 5 * time.Second,
    MaxHeaderBytes:    1 << 20,
}

httpServer, _ := config.NewHttpServer(
    manager,
    config.WithServer(myServer),
    config.WithAPIKey(os.Getenv("CONFIG_API_KEY")),
)

// Start in goroutine
go func() {
    if err := httpServer.Start(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("Server failed: %v", err)
    }
}()

// Wait for shutdown signal
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
<-sigChan

// Graceful shutdown
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := httpServer.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

### Multiple Servers (Public + Admin)

```go
// Public API - read-only, no auth
publicServer, _ := config.NewHttpServer(
    manager,
    config.WithAddress("0.0.0.0"),
    config.WithPort(8080),
)

// Admin API - full access, with auth
adminServer, _ := config.NewHttpServer(
    manager,
    config.WithAddress("127.0.0.1"),
    config.WithPort(9090),
    config.WithAPIKey("admin-secret"),
)

go publicServer.Start()
go adminServer.Start()
```

### Custom Middleware

```go
httpServer, _ := config.NewHttpServer(manager)
handler := httpServer.GetHandler()

// Add logging middleware
loggingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    log.Printf("Started %s %s", r.Method, r.URL.Path)

    handler.ServeHTTP(w, r)

    log.Printf("Completed in %v", time.Since(start))
})

http.ListenAndServe(":8080", loggingHandler)
```

### Integration with Gorilla Mux

```go
import "github.com/gorilla/mux"

router := mux.NewRouter()

// Your routes
router.HandleFunc("/api/users", usersHandler).Methods("GET")
router.HandleFunc("/api/posts", postsHandler).Methods("GET")

// Add config management
httpServer, _ := config.NewHttpServer(manager)
configHandler := httpServer.GetHandler()

router.PathPrefix("/admin/config").Handler(
    http.StripPrefix("/admin/config", configHandler),
)

http.ListenAndServe(":8080", router)
```

### Testing

```go
func TestConfigEndpoint(t *testing.T) {
    manager, _ := config.NewManager(source)
    httpServer, _ := config.NewHttpServer(manager)
    handler := httpServer.GetHandler()

    req := httptest.NewRequest("GET", "/config", nil)
    w := httptest.NewRecorder()

    handler.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", w.Code)
    }
}
```

## CORS Configuration

CORS is automatically configured with:
- **Allowed Origins**: `*` (all origins)
- **Allowed Methods**: `GET`, `POST`, `OPTIONS`
- **Allowed Headers**: `Content-Type`, `Authorization`, `X-API-Key`
- **Max Age**: 3600 seconds

To customize CORS, get the handler and wrap it:

```go
handler := httpServer.GetHandler()
customCORS := cors.New(cors.Options{
    AllowedOrigins: []string{"https://yourdomain.com"},
    AllowedMethods: []string{"GET", "POST"},
})
http.ListenAndServe(":8080", customCORS.Handler(handler))
```

## Security Best Practices

1. **Use API Keys in Production**
   ```go
   config.WithAPIKey(os.Getenv("CONFIG_API_KEY"))
   ```

2. **Use HTTPS**
   ```go
   httpServer.StartTLS("cert.pem", "key.pem")
   ```

3. **Bind to Localhost for Admin**
   ```go
   config.WithAddress("127.0.0.1")
   ```

4. **Set Appropriate Timeouts**
   ```go
   myServer := &http.Server{
       ReadTimeout:  15 * time.Second,
       WriteTimeout: 15 * time.Second,
       IdleTimeout:  60 * time.Second,
   }
   ```

5. **Limit Request Size** (automatic, 10MB max)

6. **Use Version-Based Locking**
   - Include `version` field in POST requests
   - Prevents concurrent modification conflicts

## Migration from Old Version

**Old Code:**
```go
hs := &http_server{address: addr, port: port, api_key: key, manager: m}
hs.Start()
```

**New Code (Option 1 - Standalone):**
```go
httpServer, _ := config.NewHttpServer(
    manager,
    config.WithAddress(addr),
    config.WithPort(port),
    config.WithAPIKey(key),
)
httpServer.Start()
```

**New Code (Option 2 - User-Provided Server):**
```go
myServer := &http.Server{Addr: fmt.Sprintf("%s:%d", addr, port)}
httpServer, _ := config.NewHttpServer(
    manager,
    config.WithServer(myServer),
    config.WithAPIKey(key),
)
httpServer.Start()
```

**New Code (Option 3 - Legacy Compatibility):**
```go
httpServer, _ := config.NewHttpServerFromNode(manager, configNode)
httpServer.Start()
```

## Troubleshooting

### Port Already in Use
```
Error: listen tcp :8080: bind: address already in use
```

**Solution**: Change port or stop conflicting service.

### API Key Rejected
```
{"success": false, "error": {"code": 401, "message": "unauthorized"}}
```

**Solution**: Include `X-API-Key` header with correct key.

### Version Conflict
```
{"success": false, "error": {"code": 409, "message": "version mismatch"}}
```

**Solution**: Fetch current config to get latest version, then retry.

### Server Won't Shutdown
**Solution**: Check context timeout is sufficient:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
```

## Performance Considerations

- **Request Limit**: 10MB max body size
- **Timeouts**: Default 15s read/write, 60s idle
- **Concurrent Requests**: Handled by Go's http.Server
- **Lock Duration**: Held during entire operation (insert/remove/replace)

## Summary

| Use Case | Pattern | When to Use |
|----------|---------|-------------|
| Simple deployment | Standalone with options | Microservices, config-only servers |
| Custom server needs | User-provided server | Production, custom settings |
| Existing application | Handler-only | Adding config to existing app |
| Multiple endpoints | Route integration | Unified API surface |

**Key Advantage**: Maximum flexibility - use the pattern that fits your architecture.