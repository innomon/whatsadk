package auth

import (
	"crypto/rsa"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type VerificationClaims struct {
	Mobile      string `json:"mobile"`
	AppName     string `json:"app_name"`
	CallbackURL string `json:"callback_url"`
	ChallengeID string `json:"challenge_id"`
	jwt.RegisteredClaims
}

func IsVerificationToken(raw string) *VerificationClaims {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "eyJ") {
		return nil
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil
	}

	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	claims := &VerificationClaims{}
	_, _, err := parser.ParseUnverified(raw, claims)
	if err != nil {
		return nil
	}

	if claims.Mobile == "" || claims.AppName == "" || claims.CallbackURL == "" {
		return nil
	}

	return claims
}

func VerifyVerificationToken(raw string, appKey *rsa.PublicKey) (*VerificationClaims, error) {
	claims := &VerificationClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return appKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	if claims.Mobile == "" || claims.AppName == "" || claims.CallbackURL == "" || claims.ChallengeID == "" {
		return nil, fmt.Errorf("missing required claims")
	}

	return claims, nil
}
