package main

import (
	"entertainment-ecommerce-platform/internal/adapters/primary/http"
	pg_adapter "entertainment-ecommerce-platform/internal/adapters/secondary/postgres"
	bcrypt_adapter "entertainment-ecommerce-platform/internal/adapters/secondary/bcrypt"
	jwt_adapter "entertainment-ecommerce-platform/internal/adapters/secondary/jwt"
	"entertainment-ecommerce-platform/internal/config"
	core_services "entertainment-ecommerce-platform/internal/core/services"
	"fmt"
	"log"
	stdhttp "net/http"
	"os"
	"time"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	if jwtSecret == "" {
		log.Println("Warning: JWT_SECRET_KEY not set, using default insecure key.")
		jwtSecret = "default-super-secret-key-for-dev-only"
	}
	jwtDurationStr := os.Getenv("JWT_TOKEN_DURATION_MINUTES")
	jwtDurationMinutes, err := time.ParseDuration(jwtDurationStr + "m")
	if err != nil || jwtDurationMinutes <= 0 {
		log.Printf("Warning: Invalid or missing JWT_TOKEN_DURATION_MINUTES, defaulting to 60 minutes. Error: %v", err)
		jwtDurationMinutes = 60 * time.Minute
	}

	dbCfg := pg_adapter.DBConfig{DSN: cfg.DatabaseURL}
	db, err := pg_adapter.ConnectDB(dbCfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Successfully connected to the database.")

	// --- Instantiate Secondary Adapters (Driven Adapters) ---
	userRepository := pg_adapter.NewPostgreSQLUserRepository(db)
	productRepository := pg_adapter.NewPostgreSQLProductRepository(db) // Instantiate ProductRepository

	passwordHasher := bcrypt_adapter.NewBcryptHasher(0)
	tokenGenerator, tokenValidator, err := jwt_adapter.NewJWTManager(jwtSecret, jwtDurationMinutes)
	if err != nil {
		log.Fatalf("Failed to create JWT Manager: %v", err)
	}

	// --- Instantiate Core Services (Application Logic) ---
	userService := core_services.NewUserService(userRepository, passwordHasher, tokenGenerator)
	productService := core_services.NewProductService(productRepository) // Instantiate ProductService

	// --- Setup Router (Primary Adapter - HTTP) ---
	routerCfg := &http.RouterConfig{
		UserService:    userService,
		ProductService: productService, // Pass ProductService
		TokenValidator: tokenValidator,
	}
	router := http.NewRouter(routerCfg)

	// --- Start HTTP Server ---
	serverAddr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting server on %s", serverAddr)

	httpServer := &stdhttp.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := httpServer.ListenAndServe(); err != nil && err != stdhttp.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v\n", serverAddr, err) // Corrected log format in template
	}

	log.Println("Server stopped.")
}
