package auth

import (
	"database/sql"
	"errors"
	"time"

	"github.com/kingl0w/PropLeads/internal/database"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists      = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound    = errors.New("user not found")
	ErrInactiveAccount = errors.New("account is inactive")
)

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword compares a password with a hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// CreateUser creates a new user account
func CreateUser(email, username, password string) (*User, error) {
	// Validate input
	if email == "" || username == "" || password == "" {
		return nil, errors.New("email, username, and password are required")
	}

	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	// Hash password
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Insert user
	result, err := database.DB.Exec(
		`INSERT INTO users (email, username, password_hash) VALUES (?, ?, ?)`,
		email, username, passwordHash,
	)
	if err != nil {
		// Check for unique constraint violation
		if err.Error() == "UNIQUE constraint failed: users.email" {
			return nil, ErrUserExists
		}
		return nil, err
	}

	// Get the created user
	userID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return GetUserByID(int(userID))
}

// AuthenticateUser verifies credentials and returns the user
func AuthenticateUser(email, password string) (*User, error) {
	user, err := GetUserByEmail(email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrInactiveAccount
	}

	if !CheckPassword(password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	// Update last login
	now := time.Now()
	database.DB.Exec(`UPDATE users SET last_login = ? WHERE id = ?`, now, user.ID)
	user.LastLogin = &now

	return user, nil
}

// GetUserByID retrieves a user by ID
func GetUserByID(id int) (*User, error) {
	var user User
	err := database.DB.QueryRow(
		`SELECT id, email, username, password_hash, subscription_status,
		        subscription_tier, created_at, last_login, is_active
		 FROM users WHERE id = ?`,
		id,
	).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash,
		&user.SubscriptionStatus, &user.SubscriptionTier,
		&user.CreatedAt, &user.LastLogin, &user.IsActive,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by email
func GetUserByEmail(email string) (*User, error) {
	var user User
	err := database.DB.QueryRow(
		`SELECT id, email, username, password_hash, subscription_status,
		        subscription_tier, created_at, last_login, is_active
		 FROM users WHERE email = ?`,
		email,
	).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash,
		&user.SubscriptionStatus, &user.SubscriptionTier,
		&user.CreatedAt, &user.LastLogin, &user.IsActive,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdateSubscriptionStatus updates a user's subscription (for future payment integration)
func UpdateSubscriptionStatus(userID int, status, tier string) error {
	_, err := database.DB.Exec(
		`UPDATE users SET subscription_status = ?, subscription_tier = ? WHERE id = ?`,
		status, tier, userID,
	)
	return err
}
