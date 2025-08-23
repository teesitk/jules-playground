package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq" // PostgreSQL driver, underscore for side effects (registration)

	"entertainment_ecommerce/internal/auth"
	"entertainment_ecommerce/internal/video"
	"entertainment_ecommerce/config" // Import the config package
	"entertainment_ecommerce/pkg/middleware" // Import the middleware package
)

func main() {
	log.Println("Starting server...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
	// Optionally print config for debugging (be mindful of secrets in prod logs)
	// cfg.Print()


	// Initialize Database Connection
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)

	var db *sql.DB
	// Retry connecting to DB a few times, useful for docker-compose startup order
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			log.Printf("Error connecting to database (attempt %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(5 * time.Second)
			continue
		}
		err = db.Ping()
		if err != nil {
			log.Printf("Error pinging database (attempt %d/%d): %v", i+1, maxRetries, err)
			db.Close() // Close the connection before retrying
			time.Sleep(5 * time.Second)
			continue
		}
		break // Success
	}
	if err != nil {
		log.Fatalf("Failed to connect to database after %d attempts: %v", maxRetries, err)
	}
	log.Println("Successfully connected to the database!")
	// defer db.Close() // This defer might be too early if main exits due to other reasons before server start

	// Initialize Repositories
	userRepo := auth.NewPostgresUserRepository(db)

	mockFileStorage := video.NewMockFileStorage()
	videoRepo := video.NewPostgresVideoRepository(db, mockFileStorage)

	mockRedisClient := video.NewMockRedisClient()

	// Initialize UseCases
	authUseCase := auth.NewAuthUseCase(userRepo, cfg.JWTSecret, cfg.JWTExpiryHours)
	videoUseCase := video.NewVideoUseCase(videoRepo, mockRedisClient, cfg.VideoProcessingQueue)

	// Initialize Handlers
	authHandler := auth.NewAuthHandler(authUseCase)
	videoHandler := video.NewVideoHandler(videoUseCase)

	// Initialize Router
	r := mux.NewRouter()

	// Middleware for logging requests
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("%s %s %s", r.Method, r.RequestURI, r.Proto)
			next.ServeHTTP(w, r)
		})
	})


	apiV1 := r.PathPrefix("/api/v1").Subrouter()

	// Auth routes
	authAPIRoutes := apiV1.PathPrefix("/auth").Subrouter()
	authAPIRoutes.HandleFunc("/register", authHandler.Register).Methods(http.MethodPost)
	authAPIRoutes.HandleFunc("/login", authHandler.Login).Methods(http.MethodPost)

	// Video routes
	videoAPIRoutes := apiV1.PathPrefix("/videos").Subrouter()

	// Publicly view a video by ID - using query param `id` as per video_handler.go
	// To use path variable: videoAPIRoutes.HandleFunc("/{id}", videoHandler.GetVideoByIDHandler).Methods(http.MethodGet)
	// And in handler: videoID := mux.Vars(r)["id"]
	videoAPIRoutes.HandleFunc("", videoHandler.GetVideoByIDHandler).Methods(http.MethodGet) // Expects ?id=<videoID>

	// --- Protected Routes (Authentication Required) ---
	// Create a new subrouter for routes that require JWT authentication
	protectedRoutes := apiV1.PathPrefix("").Subrouter()
	protectedRoutes.Use(middleware.JWTMiddleware(cfg.JWTSecret))

	// Apply JWT middleware to video upload
	// Note: If /videos is used for both GET (public) and POST (protected),
	// we need to be careful with router setup.
	// One way is to have specific subrouters or define POST on the protected router.

	// Let's adjust videoAPIRoutes to be more specific for public GET
	// and create a new one for protected POST if needed, or apply middleware selectively.

	// Current setup:
	// videoAPIRoutes = /api/v1/videos
	//   -> GET "" (videoHandler.GetVideoByIDHandler) -- Public
	//   -> POST "" (videoHandler.UploadVideoHandler) -- Will be protected

	// To protect only POST on /api/v1/videos:
	// We can define the POST route on a router that has the middleware.
	// Or, more simply, create a new subrouter for video uploads that uses the middleware.

	// Let's make a dedicated subrouter for protected video actions
	protectedVideoAPI := protectedRoutes.PathPrefix("/videos").Subrouter()
	protectedVideoAPI.HandleFunc("", videoHandler.UploadVideoHandler).Methods(http.MethodPost)
	// Any other protected video routes would go on protectedVideoAPI


	// Start Server
	log.Printf("Server listening on %s", cfg.ServerAddress)

	srv := &http.Server{
		Handler:      r, // Use the main router r
		Addr:         cfg.ServerAddress,
		WriteTimeout: 30 * time.Second, // Increased for potential large uploads
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown setup
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()
	log.Printf("Server started on %s", serverAddr)

	// Wait for interrupt signal to gracefully shut down the server
	// quit := make(chan os.Signal, 1)
	// signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// <-quit
	// log.Println("Shutting down server...")

	// ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// defer cancel()

	// if err := srv.Shutdown(ctx); err != nil {
	// 	log.Fatalf("Server forced to shutdown: %v", err)
	// }
	// db.Close() // Close DB connection after server shutdown
	// log.Println("Server exiting")

	// For simpler non-graceful shutdown for now (to keep it concise)
	select {} // Block forever
}
