package http

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/iancoleman/orderedmap"
	"github.com/rs/cors"
)

const (
	maxBodySize     = 10 * 1024 * 1024 // 10MB max request body
	defaultAddress  = "localhost"
	defaultPort     = 8080
	shutdownTimeout = 30 * time.Second
	readTimeout     = 15 * time.Second
	writeTimeout    = 15 * time.Second
	idleTimeout     = 60 * time.Second
)

type HttpServer struct {
	address   string
	port      int
	apiKey    string
	apiKeyHash [32]byte
	manager   *Manager
	server    *http.Server
	userProvided bool // Track if server was user-provided
}

// HttpServerOption is a function that configures HttpServer
type HttpServerOption func(*HttpServer)

// WithAddress sets the server address
func WithAddress(address string) HttpServerOption {
	return func(hs *HttpServer) {
		hs.address = address
	}
}

// WithPort sets the server port
func WithPort(port int) HttpServerOption {
	return func(hs *HttpServer) {
		if port > 0 && port <= 65535 {
			hs.port = port
		}
	}
}

// WithAPIKey sets the API key for authentication
func WithAPIKey(apiKey string) HttpServerOption {
	return func(hs *HttpServer) {
		if apiKey != "" {
			hs.apiKey = apiKey
			hs.apiKeyHash = sha256.Sum256([]byte(apiKey))
		}
	}
}

// WithServer sets a user-provided http.Server
// The server's Handler will be set by the HttpServer
func WithServer(server *http.Server) HttpServerOption {
	return func(hs *HttpServer) {
		if server != nil {
			hs.server = server
			hs.userProvided = true
			// Extract address and port from server if available
			if server.Addr != "" {
				// User's server address takes precedence
				hs.address = ""
				hs.port = 0
			}
		}
	}
}

// NewHttpServer creates a new HTTP server for the config manager
// If no server is provided via options, a default server will be created
func NewHttpServer(m *Manager, opts ...HttpServerOption) (*HttpServer, error) {
	if m == nil {
		return nil, fmt.Errorf("manager cannot be nil")
	}

	hs := &HttpServer{
		manager: m,
		address: defaultAddress,
		port:    defaultPort,
	}

	// Apply options
	for _, opt := range opts {
		opt(hs)
	}

	return hs, nil
}

// NewHttpServerFromNode creates HTTP server from config node (legacy compatibility)
func NewHttpServerFromNode(m *Manager, conf *Node) (*HttpServer, error) {
	if m == nil {
		return nil, fmt.Errorf("manager cannot be nil")
	}

	hs := &HttpServer{
		manager: m,
		address: defaultAddress,
		port:    defaultPort,
	}

	if conf != nil {
		if addrNode, err := conf.At("address"); err == nil {
			if addr, err := addrNode.GetString(); err == nil && addr != "" {
				hs.address = addr
			}
		}

		if portNode, err := conf.At("port"); err == nil {
			if port, err := portNode.GetInt(); err == nil && port > 0 && port <= 65535 {
				hs.port = port
			}
		}

		if keyNode, err := conf.At("api_key"); err == nil {
			if key, err := keyNode.GetString(); err == nil && key != "" {
				hs.apiKey = key
				hs.apiKeyHash = sha256.Sum256([]byte(key))
			}
		}
	}

	return hs, nil
}

// GetHandler returns the http.Handler for the config endpoints
// Use this to integrate with your own server/router
func (hs *HttpServer) GetHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/config", hs.handleConfig)
	mux.HandleFunc("/health", hs.handleHealth)

	// Apply CORS
	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-API-Key"},
		AllowCredentials: false,
		MaxAge:           3600,
	}).Handler(mux)

	return handler
}

// SetupRoutes configures routes on an existing http.ServeMux
// Use this to add config endpoints to your existing mux
func (hs *HttpServer) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/config", hs.handleConfig)
	mux.HandleFunc("/health", hs.handleHealth)
}

// Start starts the HTTP server
// If a user-provided server was given, it will use that server
// Otherwise, it creates a default server
func (hs *HttpServer) Start() error {
	handler := hs.GetHandler()

	if hs.server == nil {
		// Create default server
		addr := fmt.Sprintf("%s:%d", hs.address, hs.port)
		hs.server = &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		}
		log.Printf("Starting HTTP server on %s", addr)
	} else {
		// Use user-provided server, just set the handler
		if hs.userProvided {
			log.Printf("Using user-provided HTTP server at %s", hs.server.Addr)
		}
		hs.server.Handler = handler
	}

	if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// StartTLS starts the HTTP server with TLS
func (hs *HttpServer) StartTLS(certFile, keyFile string) error {
	handler := hs.GetHandler()

	if hs.server == nil {
		addr := fmt.Sprintf("%s:%d", hs.address, hs.port)
		hs.server = &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		}
		log.Printf("Starting HTTPS server on %s", addr)
	} else {
		hs.server.Handler = handler
		log.Printf("Using user-provided HTTPS server at %s", hs.server.Addr)
	}

	if err := hs.server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (hs *HttpServer) Shutdown(ctx context.Context) error {
	if hs.server == nil {
		return nil
	}

	log.Println("Shutting down HTTP server...")
	return hs.server.Shutdown(ctx)
}

// GetServer returns the underlying http.Server
// Useful if you need to configure it further after creation
func (hs *HttpServer) GetServer() *http.Server {
	return hs.server
}

func (hs *HttpServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		hs.onGet(w, r)
	case http.MethodPost:
		hs.onPost(w, r)
	case http.MethodOptions:
		hs.onOptions(w)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (hs *HttpServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

////////////////////////////////////////////////////////////////////////////////
// GET
////////////////////////////////////////////////////////////////////////////////

func (hs *HttpServer) onGet(w http.ResponseWriter, r *http.Request) {
	if !hs.checkAccess(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	data, err := hs.buildConfigState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to build config: %s", err))
		return
	}

	writeSuccess(w, data)
}

////////////////////////////////////////////////////////////////////////////////
// POST
////////////////////////////////////////////////////////////////////////////////

func (hs *HttpServer) onPost(w http.ResponseWriter, r *http.Request) {
	if !hs.checkAccess(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("could not read body: %s", err))
		return
	}

	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "request body is empty")
		return
	}

	bodyJSON := orderedmap.New()
	if err := json.Unmarshal(body, &bodyJSON); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %s", err))
		return
	}

	op, err := getString(bodyJSON, "op")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	path, err := getString(bodyJSON, "path")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate path format
	if path == "" || path[0] != '/' {
		writeError(w, http.StatusBadRequest, "path must start with '/'")
		return
	}

	value, hasValue := bodyJSON.Get("value")

	// Version-based optimistic locking (better than hash)
	var expectedVersion int64
	if versionVal, ok := bodyJSON.Get("version"); ok {
		if versionFloat, ok := versionVal.(float64); ok {
			expectedVersion = int64(versionFloat)
		} else {
			writeError(w, http.StatusBadRequest, "version must be a number")
			return
		}

		currentVersion := hs.manager.Version()
		if currentVersion != expectedVersion {
			writeError(w, http.StatusConflict,
				fmt.Sprintf("version mismatch: expected %d, current %d", expectedVersion, currentVersion))
			return
		}
	}

	// Execute operation
	switch op {
	case "insert":
		if !hasValue {
			writeError(w, http.StatusBadRequest, "value is required for insert")
			return
		}

		index, err := getIndex(bodyJSON)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := hs.manager.insert(path, index, value); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

	case "remove":
		index, err := getIndex(bodyJSON)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := hs.manager.remove(path, index); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

	case "replace":
		if !hasValue {
			writeError(w, http.StatusBadRequest, "value is required for replace")
			return
		}

		if err := hs.manager.replace(path, value); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported operation: %s", op))
		return
	}

	// Build updated config for response
	data, err := hs.buildConfigState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to build config: %s", err))
		return
	}

	writeSuccess(w, data)
}

////////////////////////////////////////////////////////////////////////////////
// OPTIONS
////////////////////////////////////////////////////////////////////////////////

func (hs *HttpServer) onOptions(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, X-API-Key")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.WriteHeader(http.StatusOK)
}

////////////////////////////////////////////////////////////////////////////////
// BUILD CONFIG STATE
////////////////////////////////////////////////////////////////////////////////

func (hs *HttpServer) buildConfigState() (*orderedmap.OrderedMap, error) {
	confJSON := orderedmap.New()
	schemaJSON := orderedmap.New()

	configStr := hs.manager.Source().getConfig()
	if configStr == nil {
		return nil, fmt.Errorf("config is nil")
	}

	if err := json.Unmarshal([]byte(*configStr), &confJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	schemaStr := hs.manager.Source().getSchema()
	if schemaStr != nil {
		if err := json.Unmarshal([]byte(*schemaStr), &schemaJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
		}
	}

	paths := orderedmap.New()
	paths.Set("insertable", hs.manager.getInsertablePaths())
	paths.Set("removable", hs.manager.getRemovablePaths())
	paths.Set("replaceable", hs.manager.getReplaceablePaths())

	out := orderedmap.New()
	out.Set("modifiable_paths", paths)
	out.Set("config", confJSON)
	out.Set("schema", schemaJSON)
	out.Set("version", hs.manager.Version())

	return out, nil
}

////////////////////////////////////////////////////////////////////////////////
// HELPERS
////////////////////////////////////////////////////////////////////////////////

func HashSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	errObj := orderedmap.New()
	errObj.Set("message", msg)
	errObj.Set("code", code)

	resp := orderedmap.New()
	resp.Set("success", false)
	resp.Set("error", errObj)

	out, _ := json.MarshalIndent(resp, "", "  ")
	w.Write(out)
}

func writeSuccess(w http.ResponseWriter, data *orderedmap.OrderedMap) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := orderedmap.New()
	resp.Set("success", true)
	resp.Set("data", data)

	out, _ := json.MarshalIndent(resp, "", "  ")
	w.Write(out)
}

func getString(m *orderedmap.OrderedMap, key string) (string, error) {
	v, ok := m.Get(key)
	if !ok {
		return "", fmt.Errorf("'%s' is missing", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("'%s' must be a string", key)
	}
	if s == "" {
		return "", fmt.Errorf("'%s' cannot be empty", key)
	}
	return s, nil
}

func getIndex(m *orderedmap.OrderedMap) (int, error) {
	val, ok := m.Get("index")
	if !ok {
		return 0, fmt.Errorf("'index' is missing")
	}
	f, ok := val.(float64)
	if !ok {
		return 0, fmt.Errorf("'index' must be a number")
	}
	if f < 0 {
		return 0, fmt.Errorf("'index' must be non-negative")
	}
	return int(f), nil
}

func (hs *HttpServer) checkAccess(r *http.Request) bool {
	if hs.apiKey == "" {
		return true // No auth required if no key set
	}

	providedKey := r.Header.Get("X-API-Key")
	if providedKey == "" {
		return false
	}

	// Constant-time comparison to prevent timing attacks
	providedHash := sha256.Sum256([]byte(providedKey))
	return subtle.ConstantTimeCompare(hs.apiKeyHash[:], providedHash[:]) == 1
}