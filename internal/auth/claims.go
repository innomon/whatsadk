package auth

import "github.com/golang-jwt/jwt/v5"

type Claims struct {
	UserID  string `json:"user_id"`
	Channel string `json:"channel"`
	jwt.RegisteredClaims
}
