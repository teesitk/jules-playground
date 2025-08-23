package auth

import (
	"encoding/json"
	"net/http"
	"time"
)

// AuthHandler handles HTTP requests for authentication.
type AuthHandler struct {
	authUseCase AuthUseCase
}

// NewAuthHandler creates a new instance of AuthHandler.
func NewAuthHandler(authUseCase AuthUseCase) *AuthHandler {
	return &AuthHandler{authUseCase: authUseCase}
}

// RegisterRequest defines the expected JSON structure for registration requests.
type RegisterRequest struct {
	Email    string   `json:"email"`
	Password string   `json:"password"`
	Roles    []string `json:"roles"` // Optional, defaults to ["user"] in usecase
}

// LoginRequest defines the expected JSON structure for login requests.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse defines the JSON structure for successful authentication responses.
type AuthResponse struct {
	User  *User  `json:"user"`
	Token string `json:"token"`
}

// ErrorResponse defines a generic JSON error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Register handles user registration requests.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, ErrorResponse{Error: "Invalid request payload"}.String(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.Email == "" || req.Password == "" {
		http.Error(w, ErrorResponse{Error: "Email and password are required"}.String(), http.StatusBadRequest)
		return
	}

	user, token, err := h.authUseCase.Register(req.Email, req.Password, req.Roles)
	if err != nil {
		// Determine appropriate status code based on error type if possible
		// For now, using a generic bad request for simplicity, but could be more specific
		// e.g. http.StatusConflict for "user already exists"
		http.Error(w, ErrorResponse{Error: err.Error()}.String(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(AuthResponse{User: user, Token: token})
}

// Login handles user login requests.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, ErrorResponse{Error: "Invalid request payload"}.String(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.Email == "" || req.Password == "" {
		http.Error(w, ErrorResponse{Error: "Email and password are required"}.String(), http.StatusBadRequest)
		return
	}

	user, token, err := h.authUseCase.Login(req.Email, req.Password)
	if err != nil {
		// For login, it's common to use http.StatusUnauthorized for failed attempts
		http.Error(w, ErrorResponse{Error: err.Error()}.String(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AuthResponse{User: user, Token: token})
}

// String method for ErrorResponse to be used with http.Error
func (e ErrorResponse) String() string {
	// A bit of a shortcut, ideally marshal to JSON string
	return `{"error":"` + e.Error + `"}`
}

// Helper to set JWT as a cookie (optional, can also be returned in body as done above)
func setTokenCookie(w http.ResponseWriter, token string, expiration time.Time) {
	cookie := http.Cookie{
		Name:     "jwt_token",
		Value:    token,
		Expires:  expiration,
		HttpOnly: true, // Important for security
		Secure:   true, // Should be true in production (requires HTTPS)
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, &cookie)
}

// Example of how to use the setTokenCookie (not directly used in current Register/Login, but good to have)
// func (h *AuthHandler) LoginWithCookie(w http.ResponseWriter, r *http.Request) {
// ... login logic ...
//	if err == nil {
//		expiration := time.Now().Add(time.Hour * 24) // Example: 24 hour cookie
//		setTokenCookie(w, token, expiration)
//		w.Header().Set("Content-Type", "application/json")
//		w.WriteHeader(http.StatusOK)
//		json.NewEncoder(w).Encode(AuthResponse{User: user, Token: token}) // Can still return token in body
//		return
//	}
// ...
// }

// Placeholder for a protected route handler to demonstrate JWT usage later
// func (h *AuthHandler) ProtectedEndpoint(w http.ResponseWriter, r *http.Request) {
//	 userID := r.Context().Value("userID").(string) // Assuming userID is set by middleware
//	 userEmail := r.Context().Value("userEmail").(string)
//	 userRoles := r.Context().Value("userRoles").([]string)

//	 response := map[string]interface{}{
//		 "message": "This is a protected endpoint",
//		 "userID": userID,
//		 "email": userEmail,
//		 "roles": userRoles,
//	 }
//	 w.Header().Set("Content-Type", "application/json")
//	 json.NewEncoder(w).Encode(response)
// }
