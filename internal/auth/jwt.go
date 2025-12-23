package auth

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(getJWTSecret())

// getJWTSecret returns the JWT secret from env or generates one
func getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// In production, this should be set via environment variable
		// For now, use a default (CHANGE THIS IN PRODUCTION!)
		return "propleads-super-secret-key-change-in-production-2024"
	}
	return secret
}

type Claims struct {
	UserID               int    `json:"userId"`
	Email                string `json:"email"`
	Username             string `json:"username"`
	SubscriptionStatus   string `json:"subscriptionStatus"`
	SubscriptionTier     string `json:"subscriptionTier"`
	jwt.RegisteredClaims
}

// GenerateToken creates a new JWT token for a user
func GenerateToken(user *User) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour) // Token valid for 24 hours

	claims := &Claims{
		UserID:             user.ID,
		Email:              user.Email,
		Username:           user.Username,
		SubscriptionStatus: user.SubscriptionStatus,
		SubscriptionTier:   user.SubscriptionTier,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "propleads",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// AuthMiddleware is a middleware that validates JWT tokens
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Bearer token format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// Validate token
		_, err := ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Optional: Check subscription status for premium features
		// if claims.SubscriptionStatus != "active" {
		//     http.Error(w, "Active subscription required", http.StatusForbidden)
		//     return
		// }

		// Token is valid, proceed to next handler
		// You can attach claims to request context if needed
		next.ServeHTTP(w, r)
	}
}
