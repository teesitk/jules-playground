package http

import (
	"entertainment-ecommerce-platform/internal/core/ports"
	"entertainment-ecommerce-platform/internal/core/services"
	"net/http" // Keep this for http constants like http.MethodGet
	"github.com/gorilla/mux"
	"log"
	"encoding/json" // Added for HealthCheckHandler
)

// RouterConfig holds all dependencies needed to set up the router.
type RouterConfig struct {
	UserService     *services.UserService
	ProductService  *services.ProductService // Added ProductService
	TokenValidator  ports.TokenValidator
}

// NewRouter creates and configures a new Gorilla Mux router.
func NewRouter(cfg *RouterConfig) *mux.Router {
	// Added ProductService to the nil check
	if cfg.UserService == nil || cfg.ProductService == nil || cfg.TokenValidator == nil {
		panic("RouterConfig missing required dependencies (UserService, ProductService, or TokenValidator)")
	}

	router := mux.NewRouter()
	router.Use(loggingMiddleware)

	userHandler := NewUserHandler(cfg.UserService)
	productHandler := NewProductHandler(cfg.ProductService) // Instantiate ProductHandler

	authMw := AuthMiddleware(cfg.TokenValidator)

	// Public routes
	router.HandleFunc("/health", HealthCheckHandler).Methods(http.MethodGet)

	authRouter := router.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", userHandler.Register).Methods(http.MethodPost)
	authRouter.HandleFunc("/login", userHandler.Login).Methods(http.MethodPost)

	// Product routes
	// Publicly accessible product listing and detail view
	router.HandleFunc("/products", productHandler.ListProducts).Methods(http.MethodGet)
	router.HandleFunc("/products/{id:[0-9]+}", productHandler.GetProduct).Methods(http.MethodGet)

	// Protected product creation route
	// Using a subrouter for /api prefix for all authenticated actions might be cleaner
	// For now, directly protecting the POST /products endpoint
	apiRouter := router.PathPrefix("/api").Subrouter() // Create a subrouter for /api
	apiRouter.Use(authMw) // Apply AuthMiddleware to all /api routes

	// Example: POST /api/products for creating a product
	apiRouter.HandleFunc("/products", productHandler.CreateProduct).Methods(http.MethodPost)

	// You could also have specific protected routes for products if needed:
	// protectedProductRouter := router.PathPrefix("/products").Subrouter()
	// protectedProductRouter.Use(authMw)
	// protectedProductRouter.HandleFunc("", productHandler.CreateProduct).Methods(http.MethodPost)
	// Note: The above line would conflict with GET /products if not careful with prefixing or separate routers.
	// Using /api/products for POST is clearer.

	log.Println("Router initialized with user and product routes.")
	return router
}

// HealthCheckHandler remains the same
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// loggingMiddleware remains the same
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s from %s", r.Method, r.RequestURI, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
