package auth

import (
	"time"
)

type User struct {
	ID                 int       `json:"id"`
	Email              string    `json:"email"`
	Username           string    `json:"username"`
	PasswordHash       string    `json:"-"` //json:"-" hides hash from api responses
	SubscriptionStatus string    `json:"subscriptionStatus"`
	SubscriptionTier   string    `json:"subscriptionTier"`
	CreatedAt          time.Time `json:"createdAt"`
	LastLogin          *time.Time `json:"lastLogin,omitempty"`
	IsActive           bool      `json:"isActive"`
}

type SignupRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
