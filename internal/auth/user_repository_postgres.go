package auth

import (
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq" // PostgreSQL driver
)

// PostgresUserRepository is an implementation of UserRepository for PostgreSQL.
type PostgresUserRepository struct {
	DB *sql.DB
}

// NewPostgresUserRepository creates a new instance of PostgresUserRepository.
func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{DB: db}
}

// Create inserts a new user into the database.
func (r *PostgresUserRepository) Create(user *User) error {
	query := `
		INSERT INTO users (id, email, password, roles, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.DB.Exec(query, user.ID, user.Email, user.Password, pq.Array(user.Roles), user.CreatedAt, user.UpdatedAt)
	if err != nil {
		// Check for unique constraint violation (e.g., email already exists)
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return errors.New("user with this email already exists")
		}
		return err
	}
	return nil
}

// FindByEmail retrieves a user from the database by their email.
func (r *PostgresUserRepository) FindByEmail(email string) (*User, error) {
	query := `
		SELECT id, email, password, roles, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	user := &User{}
	var roles pq.StringArray
	err := r.DB.QueryRow(query, email).Scan(&user.ID, &user.Email, &user.Password, &roles, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	user.Roles = []string(roles)
	return user, nil
}

// FindByID retrieves a user from the database by their ID.
func (r *PostgresUserRepository) FindByID(id string) (*User, error) {
	query := `
		SELECT id, email, password, roles, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	user := &User{}
	var roles pq.StringArray
	err := r.DB.QueryRow(query, id).Scan(&user.ID, &user.Email, &user.Password, &roles, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	user.Roles = []string(roles)
	return user, nil
}
