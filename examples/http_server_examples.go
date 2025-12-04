package examples

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"your-module/config"
)

func main() {
	// Setup manager
	configJSON := `{"users": [], "settings": {"port": 8080}}`
	schemaJSON := `{"type": "object"}`
	source, _ := config.NewStrSource(configJSON, schemaJSON)
	manager, _ := config.NewManager(source)

	// Run different examples
	// Uncomment the one you want to try

	// example1_DefaultServer(manager)
	// example2_WithOptions(manager)
	// example3_UserProvidedServer(manager)
	// example4_IntegrateWithExistingRouter(manager)
	// example5_GetHandlerOnly(manager)
	// example6_MultipleEndpoints(manager)
	example7_LegacyCompatibility(manager)
}

// Example 1: Use default server (simplest)
func example1_DefaultServer(manager *config.Manager) {
	httpServer, err := config.NewHttpServer(manager)
	if err != nil {
		log.Fatal(err)
	}

	// Starts on localhost:8080 by default
	log.Println("Starting default server on localhost:8080")
	if err := httpServer.Start(); err != nil {
		log.Fatal(err)
	}
}

// Example 2: Configure with options
func example2_WithOptions(manager *config.Manager) {
	httpServer, err := config.NewHttpServer(
		manager,
		config.WithAddress("0.0.0.0"),
		config.WithPort(9090),
		config.WithAPIKey("secret-key-123"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Starting configured server on 0.0.0.0:9090")
	if err := httpServer.Start(); err != nil {
		log.Fatal(err)
	}
}

// Example 3: Provide your own http.Server
func example3_UserProvidedServer(manager *config.Manager) {
	// Create your own server with custom settings
	myServer := &http.Server{
		Addr:           ":8888",
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Pass your server to HttpServer
	httpServer, err := config.NewHttpServer(
		manager,
		config.WithServer(myServer),
		config.WithAPIKey("my-secret-key"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// The handler will be set on your server
	log.Println("Starting user-provided server on :8888")
	if err := httpServer.Start(); err != nil {
		log.Fatal(err)
	}
}

// Example 4: Integrate with existing router/mux
func example4_IntegrateWithExistingRouter(manager *config.Manager) {
	// Your existing application's mux
	mux := http.NewServeMux()

	// Your existing routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome to my app"))
	})
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("User API"))
	})

	// Add config management routes
	httpServer, err := config.NewHttpServer(manager)
	if err != nil {
		log.Fatal(err)
	}
	httpServer.SetupRoutes(mux)

	// Start your server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("Starting integrated server with config routes on :8080")
	log.Println("  - /          -> Your app")
	log.Println("  - /api/users -> Your API")
	log.Println("  - /config    -> Config management")
	log.Println("  - /health    -> Health check")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// Example 5: Get handler and use it in your own way
func example5_GetHandlerOnly(manager *config.Manager) {
	httpServer, err := config.NewHttpServer(
		manager,
		config.WithAPIKey("secret"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Get the handler without starting the server
	handler := httpServer.GetHandler()

	// Use it however you want
	// Option A: With your own middleware
	withLogging := loggingMiddleware(handler)

	// Option B: Mount at a sub-path
	mux := http.NewServeMux()
	mux.Handle("/admin/config/", http.StripPrefix("/admin", handler))

	// Start your custom server
	server := &http.Server{
		Addr:    ":8080",
		Handler: withLogging,
	}

	log.Println("Starting server with custom handler on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// Example 6: Multiple HttpServers on different ports
func example6_MultipleEndpoints(manager *config.Manager) {
	// Public API (read-only, no auth)
	publicServer, _ := config.NewHttpServer(
		manager,
		config.WithAddress("0.0.0.0"),
		config.WithPort(8080),
	)

	// Admin API (full access, with auth)
	adminServer, _ := config.NewHttpServer(
		manager,
		config.WithAddress("127.0.0.1"),
		config.WithPort(9090),
		config.WithAPIKey("admin-secret"),
	)

	// Start both servers
	go func() {
		log.Println("Starting public server on :8080")
		if err := publicServer.Start(); err != nil {
			log.Printf("Public server error: %v", err)
		}
	}()

	go func() {
		log.Println("Starting admin server on :9090")
		if err := adminServer.Start(); err != nil {
			log.Printf("Admin server error: %v", err)
		}
	}()

	// Graceful shutdown example
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Wait for interrupt signal
	// In real code, use signal.Notify here
	time.Sleep(60 * time.Second)

	log.Println("Shutting down servers...")
	publicServer.Shutdown(ctx)
	adminServer.Shutdown(ctx)
}

// Example 7: Legacy compatibility (from config Node)
func example7_LegacyCompatibility(manager *config.Manager) {
	// Old way: configure from Node
	configNode := &config.Node{}
	// Assume configNode has address, port, api_key fields

	httpServer, err := config.NewHttpServerFromNode(manager, configNode)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Starting legacy-configured server")
	if err := httpServer.Start(); err != nil {
		log.Fatal(err)
	}
}

// Example 8: Advanced - Custom server with TLS
func example8_TLS(manager *config.Manager) {
	httpServer, err := config.NewHttpServer(
		manager,
		config.WithAddress("0.0.0.0"),
		config.WithPort(8443),
		config.WithAPIKey("secure-key"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Starting HTTPS server on :8443")
	if err := httpServer.StartTLS("cert.pem", "key.pem"); err != nil {
		log.Fatal(err)
	}
}

// Example 9: Modify server after creation
func example9_ModifyServer(manager *config.Manager) {
	httpServer, err := config.NewHttpServer(manager)
	if err != nil {
		log.Fatal(err)
	}

	// Get the underlying server and modify it
	server := httpServer.GetServer()
	if server != nil {
		server.ReadTimeout = 60 * time.Second
		server.WriteTimeout = 60 * time.Second
		server.MaxHeaderBytes = 2 << 20
	}

	log.Println("Starting modified server")
	if err := httpServer.Start(); err != nil {
		log.Fatal(err)
	}
}

// Example 10: Complete production setup
func example10_Production(manager *config.Manager) {
	// Production-grade server configuration
	myServer := &http.Server{
		Addr:              ":8080",
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	httpServer, err := config.NewHttpServer(
		manager,
		config.WithServer(myServer),
		config.WithAPIKey("production-secret-key"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Start server in goroutine
	go func() {
		log.Println("Production server starting on :8080")
		if err := httpServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Setup graceful shutdown
	// In production, use signal.Notify to catch SIGTERM/SIGINT
	shutdownChan := make(chan struct{})
	go func() {
		// Simulate shutdown signal after 5 minutes
		time.Sleep(5 * time.Minute)
		close(shutdownChan)
	}()

	<-shutdownChan
	log.Println("Shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Server stopped gracefully")
}

// Helper: Logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("Started %s %s", r.Method, r.URL.Path)

		next.ServeHTTP(w, r)

		log.Printf("Completed %s %s in %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// Example 11: Using with gorilla/mux or other routers
func example11_GorillaRouter(manager *config.Manager) {
	// If using gorilla/mux or similar
	// import "github.com/gorilla/mux"
	/*
		router := mux.NewRouter()

		// Your routes
		router.HandleFunc("/api/v1/users", usersHandler).Methods("GET")
		router.HandleFunc("/api/v1/posts", postsHandler).Methods("GET")

		// Add config routes
		httpServer, _ := config.NewHttpServer(manager)
		configHandler := httpServer.GetHandler()

		// Mount config handler at /admin/config
		router.PathPrefix("/admin/config").Handler(
			http.StripPrefix("/admin/config", configHandler),
		)

		// Start server
		http.ListenAndServe(":8080", router)
	*/
}

// Example 12: Testing setup
func example12_Testing(manager *config.Manager) {
	httpServer, err := config.NewHttpServer(
		manager,
		config.WithAddress("127.0.0.1"),
		config.WithPort(0), // Random port for testing
	)
	if err != nil {
		log.Fatal(err)
	}

	// For testing, you can get the handler directly
	handler := httpServer.GetHandler()

	// Use httptest
	// req := httptest.NewRequest("GET", "/config", nil)
	// w := httptest.NewRecorder()
	// handler.ServeHTTP(w, req)

	fmt.Println("Handler ready for testing:", handler != nil)
}