package auth

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt" // For password hashing
	"github.com/golang-jwt/jwt/v5" // For JWT generation
	"github.com/google/uuid" // For generating user IDs
)

// AuthUseCase defines the interface for authentication and user management.
type AuthUseCase interface {
	Register(email, password string, roles []string) (*User, string, error) // Returns user, token, error
	Login(email, password string) (*User, string, error)                   // Returns user, token, error
}

// authUseCase implements the AuthUseCase interface.
type authUseCase struct {
	userRepo       UserRepository
	jwtSecret      []byte
	jwtExpiryHours int
}

// NewAuthUseCase creates a new instance of AuthUseCase.
func NewAuthUseCase(userRepo UserRepository, jwtSecret string, jwtExpiryHours int) AuthUseCase {
	return &authUseCase{
		userRepo:       userRepo,
		jwtSecret:      []byte(jwtSecret),
		jwtExpiryHours: jwtExpiryHours,
	}
}

// Register creates a new user, hashes their password, and returns the user and a JWT.
func (uc *authUseCase) Register(email, password string, roles []string) (*User, string, error) {
	if email == "" || password == "" {
		return nil, "", errors.New("email and password are required")
	}

	// Check if user already exists
	if _, err := uc.userRepo.FindByEmail(email); err == nil {
		return nil, "", errors.New("user with this email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}

	newUser := &User{
		ID:        uuid.NewString(),
		Email:     email,
		Password:  string(hashedPassword),
		Roles:     roles,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if len(newUser.Roles) == 0 { // Default role
		newUser.Roles = []string{"user"}
	}


	err = uc.userRepo.Create(newUser)
	if err != nil {
		return nil, "", err
	}

	// Generate JWT token
	tokenString, err := uc.generateJWT(newUser)
	if err != nil {
		return nil, "", err
	}

	// Don't return password hash
	newUser.Password = ""
	return newUser, tokenString, nil
}

// Login verifies user credentials and returns the user and a JWT.
func (uc *authUseCase) Login(email, password string) (*User, string, error) {
	if email == "" || password == "" {
		return nil, "", errors.New("email and password are required")
	}

	user, err := uc.userRepo.FindByEmail(email)
	if err != nil {
		// It's good practice not to reveal if the email exists or not for security reasons
		return nil, "", errors.New("invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		// Passwords don't match
		return nil, "", errors.New("invalid email or password")
	}

	// Generate JWT token
	tokenString, err := uc.generateJWT(user)
	if err != nil {
		return nil, "", err
	}

	// Don't return password hash
	user.Password = ""
	return user, tokenString, nil
}

// generateJWT creates a new JWT for a given user.
func (uc *authUseCase) generateJWT(user *User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"roles": user.Roles,
		"exp":   time.Now().Add(time.Hour * time.Duration(uc.jwtExpiryHours)).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(uc.jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
