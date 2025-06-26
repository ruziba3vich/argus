package helper

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/ruziba3vich/argus/internal/entity"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func ValidatePassword(password string) bool {
	// Check if password length is at least 6 characters
	if len(password) < 6 {
		return false
	}

	var hasLower, hasUpper bool
	for _, char := range password {
		// Check if password contains at least one lowercase letter
		if unicode.IsLower(char) {
			hasLower = true
		}
		// Check if password contains at least one uppercase letter
		if unicode.IsUpper(char) {
			hasUpper = true
		}
	}

	// Return true only if both lowercase and uppercase letters are present
	return hasLower && hasUpper
}

func ValidatePhoneNumber(phoneNumber string) bool {
	// Check if phone number is 13 characters long
	if len(phoneNumber) != 13 {
		return false
	}

	// Check if first 4 characters are "+998"
	prefix := "+998"
	if phoneNumber[:4] != prefix {
		return false
	}

	if _, err := strconv.Atoi(phoneNumber[4:]); err != nil {
		return false
	}

	// Return true if all conditions are met
	return true
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hashedBytes), nil
}

// CheckPassword compares a hashed password with a plain text password
func CheckPassword(plainPassword, hashedPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}

// GenerateRandomCode generates a random numeric code of specified length
func GenerateRandomCode(length int) (string, error) {
	const digits = "0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		result[i] = digits[num.Int64()]
	}
	return string(result), nil
}

// GenerateJWT generates a JWT token with given claims
func GenerateJWT(userID, role, signingKey string, timeout int) (string, string, error) {
	// Access token
	accessTokenClaims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(time.Hour * time.Duration(timeout)).Unix(),
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString([]byte(signingKey))
	if err != nil {
		return "", "", fmt.Errorf("error while generating access token: %w", err)
	}

	// Refresh token with longer expiry
	refreshTokenClaims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 days
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(signingKey))
	if err != nil {
		return "", "", fmt.Errorf("error while generating refresh token: %w", err)
	}

	return accessTokenString, refreshTokenString, nil
}

// ParseToken parses and validates a JWT token
func ParseToken(tokenString, signingKey string) (*entity.JWTClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(signingKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return &entity.JWTClaims{
		Sub:  claims["sub"].(string),
		Role: claims["role"].(string),
		Exp:  int64(claims["exp"].(float64)),
	}, nil
}

// PaginationParams holds the parameters for pagination

// GetPaginationParams extracts pagination parameters from the request.
func GetPaginationParams(c *gin.Context) (int, int) {
	// Default values
	defaultLimit := 10
	defaultPage := 1

	// Get 'limit' query parameter
	limit := defaultLimit
	limitParam := c.DefaultQuery("limit", strconv.Itoa(defaultLimit))
	if parsedLimit, err := strconv.Atoi(limitParam); err == nil && parsedLimit > 0 {
		limit = parsedLimit
	}

	// Get 'page' query parameter
	page := defaultPage
	pageParam := c.DefaultQuery("page", strconv.Itoa(defaultPage))
	if parsedPage, err := strconv.Atoi(pageParam); err == nil && parsedPage > 0 {
		page = parsedPage
	}

	return page, limit
}

func IsValidUUID(uuid string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(uuid)
}
