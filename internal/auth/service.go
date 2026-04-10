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

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func CreateUser(email, username, password string) (*User, error) {
	if email == "" || username == "" || password == "" {
		return nil, errors.New("email, username, and password are required")
	}

	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	result, err := database.DB.Exec(
		`INSERT INTO users (email, username, password_hash) VALUES (?, ?, ?)`,
		email, username, passwordHash,
	)
	if err != nil {
		if err.Error() == "UNIQUE constraint failed: users.email" {
			return nil, ErrUserExists
		}
		return nil, err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return GetUserByID(int(userID))
}

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

	now := time.Now()
	database.DB.Exec(`UPDATE users SET last_login = ? WHERE id = ?`, now, user.ID)
	user.LastLogin = &now

	return user, nil
}

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

//stub for future payment integration
func UpdateSubscriptionStatus(userID int, status, tier string) error {
	_, err := database.DB.Exec(
		`UPDATE users SET subscription_status = ?, subscription_tier = ? WHERE id = ?`,
		status, tier, userID,
	)
	return err
}
