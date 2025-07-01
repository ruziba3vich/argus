package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/ruziba3vich/argus/internal/pkg/config"
	logger "github.com/ruziba3vich/prodonik_lgger"
)

var (
	ErrValidationErrorMalformed  = errors.New("Token is malformed")
	ErrTokenExpiredOrNotValidYet = errors.New("Token is either expired or not active yet")
)

type JWTHandler struct {
	Sub        string
	Exp        string
	Iat        string
	Role       string
	SigningKey string
	Log        *logger.Logger
	Token      string
	Timeout    int
}

func GenerateJwtToken(jwtsecret string, claims *jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Sign and get the complete encoded token as a string using the secret
	return token.SignedString([]byte(jwtsecret))
}

func GenerateToken(cfg *config.Config, sub, token_type string, optionalFields ...map[string]interface{}) (string, string, error) {
	accessClaims := jwt.MapClaims{
		"sub":  sub,
		"type": token_type,
		"exp":  time.Now().Add(cfg.Token.AccessTokenExpirationTime).Unix(),
		"iat":  time.Now().Unix(),
	}

	for _, fields := range optionalFields {
		for key, value := range fields {
			accessClaims[key] = value
		}
	}

	// generate access token
	access_token, err := GenerateJwtToken(cfg.Token.SigningKey, &accessClaims)
	if err != nil {
		return "", "", err
	}

	// generate refresh token
	refresh_token, err := GenerateJwtToken(cfg.Token.SigningKey, &jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(cfg.Token.RefreshTokenExpirationTime).Unix(),
		"sub": sub,
	})
	if err != nil {
		return "", "", err
	}
	return access_token, refresh_token, err
}

func ParseJwtToken(tokenStr, jwtsecret string) (map[string]interface{}, error) {
	var claims map[string]interface{}
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtsecret), nil
	})

	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				return claims, ErrValidationErrorMalformed
			}
			if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
				return claims, ErrTokenExpiredOrNotValidYet
			}
		}
		return claims, fmt.Errorf("Couldn't handle this token: %w", err)
	}
	// get claims
	if mapClaims, ok := token.Claims.(jwt.MapClaims); ok {
		claims = mapClaims
	}
	return claims, nil
}

func ExtractClaims(signingKey, tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(signingKey), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !(ok && token.Valid) {
		return nil, err
	}
	return claims, nil
}
