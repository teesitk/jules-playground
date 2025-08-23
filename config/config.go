package config

import (
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	// It's good practice to use a struct tag based library for env vars
	// For example: "github.com/kelseyhightower/envconfig"
	// If that's not allowed/available, we'll do it manually.
)

// Config holds all configuration for the application.
// Environment variables will be used to populate this struct.
type Config struct {
	DBHost               string
	DBPort               int
	DBUser               string
	DBPassword           string
	DBName               string
	JWTSecret            string
	JWTExpiryHours       int
	ServerAddress        string // e.g. ":8080"
	VideoProcessingQueue string
	LogLevel             string
	// S3 related config (if we were to implement real S3 storage)
	// S3Endpoint string
	// S3AccessKeyID string
	// S3SecretKey string
	// S3BucketName string
	// S3Region string
	// S3UseSSL bool
	// Redis related config (if we were to implement real Redis client)
	// RedisAddr string
	// RedisPassword string
	// RedisDB int
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{}
	var err, lastErr error // To store the last error from getEnvAs helpers

	cfg.DBHost = getEnv("DB_HOST", "localhost")
	cfg.DBPort, err = getEnvAsInt("DB_PORT", 5432)
	if err != nil {
		log.Printf("Warning: Invalid DB_PORT value '%s', using default 5432. Error: %v", os.Getenv("DB_PORT"), err)
		lastErr = err
	}
	cfg.DBUser = getEnv("DB_USER", "testuser")
	cfg.DBPassword = getEnv("DB_PASSWORD", "testpassword")
	cfg.DBName = getEnv("DB_NAME", "entertainmentecomm")

	cfg.JWTSecret = getEnv("JWT_SECRET", "a_very_secret_key_that_should_be_long_and_random_and_changed_in_prod")
	if cfg.JWTSecret == "a_very_secret_key_that_should_be_long_and_random_and_changed_in_prod" {
		// This is a default, potentially insecure secret. Log a prominent warning.
		log.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
		log.Println("WARNING: JWT_SECRET is using a default, insecure value.")
		log.Println("Please set a strong, unique JWT_SECRET environment variable for production deployments.")
		log.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	}
	cfg.JWTExpiryHours, err = getEnvAsInt("JWT_EXPIRY_HOURS", 24)
	if err != nil {
		log.Printf("Warning: Invalid JWT_EXPIRY_HOURS value '%s', using default 24. Error: %v", os.Getenv("JWT_EXPIRY_HOURS"), err)
		lastErr = err
	}

	serverPort := getEnv("SERVER_PORT", "8080")
	cfg.ServerAddress = ":" + serverPort

	cfg.VideoProcessingQueue = getEnv("VIDEO_PROCESSING_QUEUE", "video_processing_jobs")
	cfg.LogLevel = getEnv("LOG_LEVEL", "info")

	// cfg.S3UseSSL, err = getEnvAsBool("S3_USE_SSL", true)
	// if err != nil {
	//  log.Printf("Warning: Invalid S3_USE_SSL value '%s', using default true. Error: %v", os.Getenv("S3_USE_SSL"), err)
	//	lastErr = err
	// }


	// If any getEnvAs* helper returned an error (e.g. for parsing),
	// we might want to decide if it's critical. For now, we log warnings and use defaults.
	// The `lastErr` variable can be returned if strict parsing is required.
	// For this setup, we'll allow defaults to take precedence over parse errors.
	_ = lastErr // Suppress unused variable warning if not returning it.

	log.Printf("Configuration loaded: ServerAddress=%s, LogLevel=%s", cfg.ServerAddress, cfg.LogLevel)
	// cfg.Print() // Optionally print config for debugging (be mindful of secrets)
	return cfg, nil // Currently not returning parsing errors as critical
}

// getEnv retrieves an environment variable or returns a default value.
// It also logs when a default is being used.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	log.Printf("Environment variable '%s' not set, using default value: '%s'", key, fallback)
	return fallback
}

// getEnvAsInt retrieves an environment variable as an integer or returns a default value.
// Logs a message if the default is used or if parsing fails.
func getEnvAsInt(key string, fallback int) (int, error) {
	valueStr, exists := os.LookupEnv(key)
	if !exists {
		log.Printf("Environment variable '%s' not set, using default value: %d", key, fallback)
		return fallback, nil
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		// Do not return fallback directly on error, let caller decide.
		// Log that parsing failed. The Load() function will use the fallback.
		return fallback, errors.New("failed to parse " + key + "='" + valueStr + "' as integer")
	}
	return value, nil
}

// getEnvAsBool retrieves an environment variable as a boolean or returns a default value.
// Accepts "true", "1", "false", "0". Case-insensitive for "true"/"false".
// Logs a message if the default is used or if parsing fails.
func getEnvAsBool(key string, fallback bool) (bool, error) {
	valueStr, exists := os.LookupEnv(key)
	if !exists {
		log.Printf("Environment variable '%s' not set, using default value: %t", key, fallback)
		return fallback, nil
	}

	valueStrLower := strings.ToLower(valueStr)
	if valueStrLower == "true" || valueStr == "1" {
		return true, nil
	}
	if valueStrLower == "false" || valueStr == "0" {
		return false, nil
	}
	// Do not return fallback directly on error, let caller decide.
	// Log that parsing failed. The Load() function will use the fallback.
	return fallback, errors.New("invalid boolean value for " + key + ": '" + valueStr + "' (expected true/false/1/0)")
}

// Print logs the current configuration values.
// Sensitive information like passwords and secrets are redacted.
func (c *Config) Print() {
	log.Println("--- Application Configuration ---")
	log.Printf("  DBHost: %s", c.DBHost)
	log.Printf("  DBPort: %d", c.DBPort)
	log.Printf("  DBUser: %s", c.DBUser)
	log.Printf("  DBPassword: %s", "[REDACTED]")
	log.Printf("  DBName: %s", c.DBName)
	log.Printf("  JWTSecret: %s", "[REDACTED]")
	log.Printf("  JWTExpiryHours: %d H", c.JWTExpiryHours)
	log.Printf("  ServerAddress: %s", c.ServerAddress)
	log.Printf("  VideoProcessingQueue: %s", c.VideoProcessingQueue)
	log.Printf("  LogLevel: %s", c.LogLevel)
	// Add S3/Redis details here if/when they are fully configured
	log.Println("-------------------------------")
}
