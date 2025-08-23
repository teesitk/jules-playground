package middleware

import (
	"context"
	"encoding/json"
	"errors" // Added for errors.Is
	"fmt"
	"log"
	"net/http"
	"strings"
	// "time" // Not directly used here, but often useful in middleware

	"github.com/golang-jwt/jwt/v5"
)

// ContextKey is a type used for context keys to avoid collisions.
type ContextKey string

const (
	UserIDCtxKey    ContextKey = "userID"
	UserEmailCtxKey ContextKey = "userEmail"
	UserRolesCtxKey ContextKey = "userRoles"
)

// ErrorResponse defines a generic JSON error response for middleware.
type ErrorResponse struct {
	Error string `json:"error"`
}

// JWTMiddleware creates a new HTTP middleware for JWT authentication.
// It takes the JWT secret key as a parameter.
func JWTMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Println("JWTMiddleware: Authorization header missing")
				writeJSONError(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				log.Printf("JWTMiddleware: Authorization header format must be Bearer {token}, got: %s", authHeader)
				writeJSONError(w, "Authorization header format must be Bearer {token}", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			claims := &jwt.MapClaims{} // Using MapClaims for flexibility

			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(jwtSecret), nil
			})

			if err != nil {
				log.Printf("JWTMiddleware: Error parsing or validating token: %v", err)
				if errors.Is(err, jwt.ErrTokenExpired) {
					writeJSONError(w, "Token has expired", http.StatusUnauthorized)
				} else if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
					writeJSONError(w, "Invalid token signature", http.StatusUnauthorized)
				} else if errors.Is(err, jwt.ErrTokenNotValidYet) {
					writeJSONError(w, "Token not yet valid", http.StatusUnauthorized)
				} else {
					writeJSONError(w, "Invalid token", http.StatusUnauthorized)
				}
				return
			}

			if !token.Valid { // Should be caught by ParseWithClaims errors, but as a safeguard
				log.Println("JWTMiddleware: Token is invalid (marked as not valid by library)")
				writeJSONError(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Extract claims
			userID, ok := (*claims)["sub"].(string)
			if !ok || userID == "" {
				log.Println("JWTMiddleware: UserID (sub) claim missing or invalid in token")
				writeJSONError(w, "Invalid token: UserID (sub claim) missing or malformed", http.StatusUnauthorized)
				return
			}

			userEmail, _ := (*claims)["email"].(string) // Email is optional in context

			var userRoles []string
			rolesClaimInterface, rolesClaimExists := (*claims)["roles"]
			if rolesClaimExists {
				if rolesClaim, ok := rolesClaimInterface.([]interface{}); ok {
					for _, roleInterface := range rolesClaim {
						if rStr, ok := roleInterface.(string); ok {
							userRoles = append(userRoles, rStr)
						}
					}
				} else {
					log.Printf("JWTMiddleware: Roles claim for user %s is not an array of interfaces as expected.", userID)
				}
			}

			if len(userRoles) == 0 {
				log.Printf("JWTMiddleware: No roles found in token for user %s or roles claim malformed. Proceeding without specific roles in context.", userID)
			}

			// Add user info to context
			ctx := context.WithValue(r.Context(), UserIDCtxKey, userID)
			if userEmail != "" { // Only add if present
				ctx = context.WithValue(ctx, UserEmailCtxKey, userEmail)
			}
			if len(userRoles) > 0 { // Only add if present
				ctx = context.WithValue(ctx, UserRolesCtxKey, userRoles)
			}

			log.Printf("JWTMiddleware: Authorized. UserID: %s, Email: %s, Roles: %v", userID, userEmail, userRoles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeJSONError is a helper to standardize JSON error responses from the middleware.
func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// Example of how a handler would retrieve values from context:
// func MyProtectedHandler(w http.ResponseWriter, r *http.Request) {
//     userID, ok := r.Context().Value(UserIDCtxKey).(string)
//     if !ok {
//         http.Error(w, "Failed to get userID from context", http.StatusInternalServerError)
//         return
//     }
//
//     userEmail, _ := r.Context().Value(UserEmailCtxKey).(string)
//     userRoles, _ := r.Context().Value(UserRolesCtxKey).([]string)
//
//     fmt.Fprintf(w, "Hello, User %s! Your email is %s and roles are %v", userID, userEmail, userRoles)
// }
