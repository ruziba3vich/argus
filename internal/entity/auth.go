package entity

import "time"

// Authentication request and response structures
type (
	// RegisterRequest represents the user registration request payload
	RegisterRequest struct {
		FirstName   string `json:"first_name" binding:"required,min=2,max=30"`
		LastName    string `json:"last_name"`
		Email       string `json:"email" binding:"required,email"`
		PhoneNumber string `json:"phone_number"`
		Password    string `json:"password" binding:"required,min=6,max=16"`
	}

	// RegisterByPhoneRequest represents the user registration request payload by phone number
	RegisterByPhoneRequest struct {
		FirstName   string `json:"first_name" binding:"required,min=2,max=30"`
		LastName    string `json:"last_name"`
		PhoneNumber string `json:"phone_number"`
		Password    string `json:"password" binding:"required,min=6,max=16"`
	}

	ListUsers struct {
		Items []User `json:"itmes"`
		Total int    `json:"total"`
	}

	// LoginByPhoneRequest represents the login by phone request payload
	LoginByPhoneRequest struct {
		Email    string `json:"phone_number" binding:"required,phone_number"`
		Password string `json:"password" binding:"required,min=6,max=16"`
	}

	// LoginRequest represents the login request payload
	LoginRequest struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6,max=16"`
	}

	// VerifyEmail represents the email verification request
	VerifyEmail struct {
		Email string `json:"email" binding:"required,email"`
		Code  string `json:"code" binding:"required"`
	}

	// VerifyPhone represents the phone verification request
	VerifyPhone struct {
		Phone string `json:"phone" binding:"required,phone"`
		Code  string `json:"code" binding:"required"`
	}

	// VerifyRequest represents the request to verify a user with a code
	VerifyRequest struct {
		Email       string `json:"email" binding:"required,email"`
		Code        string `json:"code" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	// ForgotPasswordRequest represents the forgot password request
	ForgotPasswordRequest struct {
		Email string `json:"email" binding:"required,email"`
	}

	// UpdatePasswordRequest represents the update password request
	UpdatePasswordRequest struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	// AccessToken represents a token for API access
	AccessToken struct {
		AccessToken string `json:"access_token"`
	}

	// JWTClaims represents the JWT token claims
	JWTClaims struct {
		Sub  string `json:"sub"`
		Role string `json:"role"`
		Exp  int64  `json:"exp"`
	}

	// UpdatePassword represents a request to update a password
	UpdatePassword struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
)

// User represents a user in the system
type Clinet struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Surname    string    `json:"surname"`
	Username   string    `json:"username"`
	Password   string    `json:"password"`
	BirthDate  string    `json:"birth_date"`
	TgUserName string    `json:"tg_user_name"`
	Phone      string    `json:"phone"`
	Instagram  string    `json:"instagram"`
	ClientFrom string    `json:"client_from"`
	RoleID     string    `json:"role_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Role represents a user role in the system
