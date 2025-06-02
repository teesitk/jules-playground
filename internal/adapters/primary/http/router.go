package http

import (
	"entertainment-ecommerce-platform/internal/core/ports"
	"entertainment-ecommerce-platform/internal/core/services"
	"net/http"
	"github.com/gorilla/mux"
	"log"
	"encoding/json"
)

type RouterConfig struct {
	UserService     *services.UserService
	ProductService  *services.ProductService
	VideoService    *services.VideoService // Added VideoService
	TokenValidator  ports.TokenValidator
}

func NewRouter(cfg *RouterConfig) *mux.Router {
	// Added VideoService to the nil check
	if cfg.UserService == nil || cfg.ProductService == nil || cfg.VideoService == nil || cfg.TokenValidator == nil {
		panic("RouterConfig missing required dependencies (UserService, ProductService, VideoService, or TokenValidator)")
	}

	router := mux.NewRouter()
	router.Use(loggingMiddleware)

	userHandler := NewUserHandler(cfg.UserService)
	productHandler := NewProductHandler(cfg.ProductService)
	videoHandler := NewVideoHandler(cfg.VideoService) // Instantiate VideoHandler

	authMw := AuthMiddleware(cfg.TokenValidator)

	// --- Public routes ---
	router.HandleFunc("/health", HealthCheckHandler).Methods(http.MethodGet)

	authRouter := router.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", userHandler.Register).Methods(http.MethodPost)
	authRouter.HandleFunc("/login", userHandler.Login).Methods(http.MethodPost)

	// Product routes (public)
	router.HandleFunc("/products", productHandler.ListProducts).Methods(http.MethodGet)
	router.HandleFunc("/products/{id:[0-9]+}", productHandler.GetProduct).Methods(http.MethodGet)

	// Video routes (public parts)
	router.HandleFunc("/videos/{id:[0-9]+}", videoHandler.GetVideo).Methods(http.MethodGet) // Publicly view a video's details
	router.HandleFunc("/users/{userID:[0-9]+}/videos", videoHandler.ListUserVideos).Methods(http.MethodGet) // Publicly list videos by user
	router.HandleFunc("/products/{productID:[0-9]+}/videos", videoHandler.ListProductVideos).Methods(http.MethodGet) // Publicly list videos for a product


	// --- Protected routes (/api) ---
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(authMw)

	// Protected Product routes
	apiRouter.HandleFunc("/products", productHandler.CreateProduct).Methods(http.MethodPost)
	// Add other protected product routes here (e.g., PUT /api/products/{id}, DELETE /api/products/{id})

	// Protected Video routes
	apiRouter.HandleFunc("/videos/upload-request", videoHandler.RequestVideoUpload).Methods(http.MethodPost)
	apiRouter.HandleFunc("/videos/{id:[0-9]+}/confirm-upload", videoHandler.ConfirmVideoUpload).Methods(http.MethodPost)
	apiRouter.HandleFunc("/videos/{id:[0-9]+}", videoHandler.DeleteVideo).Methods(http.MethodDelete)


	log.Println("Router initialized with user, product, and video routes.")
	return router
}

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s from %s", r.Method, r.RequestURI, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
