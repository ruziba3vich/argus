package v1

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/ruziba3vich/argus/api/middleware"
	"github.com/ruziba3vich/argus/internal/entity"
	handlers "github.com/ruziba3vich/argus/internal/https"
	"github.com/ruziba3vich/argus/internal/pkg/config"
	"github.com/ruziba3vich/argus/internal/pkg/helper"
	"github.com/ruziba3vich/argus/internal/service"
	logger "github.com/ruziba3vich/prodonik_lgger"
	"golang.org/x/crypto/bcrypt"
)

// HandlerOption would be extended to include the new dependency.
// This is shown here for context but would be defined once in your project.
/*
type HandlerOption struct {
    // ... other services
	User       service.UserRepoInterface // Added dependency
	Logger     logger.Logger
	Config     *config.Config
	Enforcer   *casbin.CachedEnforcer
}
*/

// UserResponse is a sanitized version of entity.User, omitting sensitive fields.
type UserResponse struct {
	ID        int64           `json:"id"`
	FirstName string          `json:"first_name"`
	LastName  string          `json:"last_name"`
	Role      entity.UserRole `json:"role"`
	Email     string          `json:"email"`
	Phone     string          `json:"phone"`
	PhotoURL  *string         `json:"photo_url,omitempty"`
	Bio       *string         `json:"bio,omitempty"`
	CreatedAt any             `json:"created_at"`
	UpdatedAt any             `json:"updated_at"`
}

// UpdateUserPayload is used for binding the update request, including a plain-text password.
type UpdateUserPayload struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Role      *string `json:"role"`
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
	PhotoURL  *string `json:"photo_url,omitempty"`
	Bio       *string `json:"bio,omitempty"`
	Password  *string `json:"password,omitempty"` // For password changes
}

type userRoutes struct {
	handlers.BaseHandler
	userUC   service.UserRepoInterface
	log      logger.Logger
	cfg      *config.Config
	enforcer *casbin.CachedEnforcer
}

// NewUserRoutes sets up the routes for user management.
func NewUserRoutes(apiV1Group *gin.RouterGroup, option *handlers.HandlerOption) {
	r := &userRoutes{
		userUC:   option.User, // Assuming option.User exists
		log:      option.Logger,
		cfg:      option.Config,
		enforcer: option.Enforcer,
	}

	// Define authorization policies for user endpoints
	policies := [][]string{
		{"admin", "/v1/users/", "GET"},
		{"admin", "/v1/users/", "POST"},
		{"admin", "/v1/users/", "PUT"},
		{"admin", "/v1/users/", "DELETE"},
		// Users can get/update their own profiles via a different route like /users/me
	}

	for _, policy := range policies {
		_, err := option.Enforcer.AddPolicy(policy)
		if err != nil {
			option.Logger.Error("error during user enforcer add policies", map[string]any{"error": err.Error()})
		}
	}

	userGroup := apiV1Group.Group("/users")
	{
		// userGroup.Use(middleware.Authorizer(option.Enforcer, option.Logger))

		userGroup.POST("", r.createUser)
		userGroup.GET("/:id", r.getUserByID)
		userGroup.GET("", r.getAllUsers)
		userGroup.PUT("/:id", r.updateUser)
		userGroup.DELETE("/:id", r.deleteUser)
	}
}

// handleResponse is a generic response handler.
func (r *userRoutes) handleResponse(c *gin.Context, status Status, customMessage any, data any) {
	if status.Code >= 400 {
		if data == nil {
			data = customMessage
		}
		reqID, exists := c.Get(middleware.RequestIDHeader)
		if exists {
			data = fmt.Errorf("requestID : %s, %v", reqID, data)
		}
		r.log.Error(fmt.Sprintf("API Error: %s", data), map[string]any{"request_id": reqID, "status_code": status.Code})
		c.JSON(status.Code, Response{
			Status:        status.Status,
			Description:   status.Description,
			Data:          nil,
			CustomMessage: customMessage,
		})
		return
	}
	c.JSON(status.Code, Response{
		Status:        status.Status,
		Description:   status.Description,
		Data:          data,
		CustomMessage: status.CustomMessage,
	})
}

// @Router /users [post]
// @Summary Create a new user
// @Description Creates a new user with a hashed password
// @Tags USERS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user body entity.CreateUserRequest true "User creation details"
// @Success 201 {object} Response{data=v1.UserResponse} "User created successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid input or validation error"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *userRoutes) createUser(c *gin.Context) {
	var req entity.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	// Basic validation
	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" || req.Role == "" || req.Phone == "" {
		r.handleResponse(c, BadRequest, "Missing required fields: email, password, first_name, last_name, role, phone", nil)
		return
	}
	// Simple email validation
	if !regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`).MatchString(req.Email) {
		r.handleResponse(c, BadRequest, "Invalid email format", nil)
		return
	}

	createdUser, err := r.userUC.Create(c, &req)
	if err != nil {
		r.log.Error("Error while creating user", map[string]any{"error": err.Error()})
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			r.handleResponse(c, BadRequest, "User with this email or phone already exists", err.Error())
			return
		}
		r.handleResponse(c, InternalServerError, "Error while creating user", err.Error())
		return
	}

	// Return a sanitized response
	resp := UserResponse{
		ID:        createdUser.ID,
		FirstName: createdUser.FirstName,
		LastName:  createdUser.LastName,
		Role:      createdUser.Role,
		Email:     createdUser.Email,
		Phone:     createdUser.Phone,
		PhotoURL:  createdUser.PhotoURL,
		Bio:       createdUser.Bio,
		CreatedAt: createdUser.CreatedAt,
		UpdatedAt: createdUser.UpdatedAt,
	}

	r.handleResponse(c, Created, "User created successfully", resp)
}

// @Router /users/{id} [get]
// @Summary Get a user by ID
// @Description Retrieves a single user by their unique ID, without sensitive data
// @Tags USERS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} Response{data=v1.UserResponse} "User details"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - User not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *userRoutes) getUserByID(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid user ID format", nil)
		return
	}

	user, err := r.userUC.Get(c, map[string]string{"id": idStr})
	if err != nil {
		r.log.Error("Error while getting user by ID", map[string]any{"error": err.Error(), "user_id": id})
		if strings.Contains(err.Error(), "no rows in result set") {
			r.handleResponse(c, NotFound, fmt.Sprintf("User with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error retrieving user", err.Error())
		return
	}

	// Sanitize the response: DO NOT return hashed passwords or tokens
	resp := UserResponse{
		ID:        user.ID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Role:      user.Role,
		Email:     user.Email,
		Phone:     user.Phone,
		PhotoURL:  user.PhotoURL,
		Bio:       user.Bio,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	r.handleResponse(c, OK, nil, resp)
}

// @Router /users [get]
// @Summary Get all users
// @Description Retrieves a list of users with optional filtering and pagination
// @Tags USERS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param role query string false "Filter by Role (e.g., 'admin', 'user')"
// @Param search query string false "Search by name, email, or phone"
// @Param page query int false "Page number for pagination" default(1)
// @Param limit query int false "Number of records per page" default(10)
// @Success 200 {object} Response{data=entity.GetAllUsersResponse} "Successfully retrieved users"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid query parameters"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *userRoutes) getAllUsers(c *gin.Context) {
	page, limit := helper.GetPaginationParams(c)
	search := c.Query("search")
	filter := make(map[string]string)

	if role := c.Query("role"); role != "" {
		filter["role"] = role
	}

	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	// The service layer's List method already returns sanitized data (no passwords)
	users, err := r.userUC.List(c, uint64(limit), uint64(offset), filter, search)
	if err != nil {
		r.log.Error("Error while getting all users", map[string]any{"error": err.Error()})
		r.handleResponse(c, InternalServerError, "Error retrieving users", err.Error())
		return
	}

	r.handleResponse(c, OK, nil, users)
}

// @Router /users/{id} [put]
// @Summary Update a user
// @Description Updates an existing user's details by their ID. Can also update password.
// @Tags USERS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID to update"
// @Param user body v1.UpdateUserPayload true "Updated user details"
// @Success 200 {object} Response{data=string} "User updated successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID or request body"
// @Failure 404 {object} Response{data=string} "Not Found - User not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *userRoutes) updateUser(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid user ID format", nil)
		return
	}

	var payload UpdateUserPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	role := entity.GetUserRole(*payload.Role)

	updateReq := &entity.UpdateUserRequest{
		ID:        id,
		FirstName: payload.FirstName,
		LastName:  payload.LastName,
		Role:      &role,
		Email:     payload.Email,
		Phone:     payload.Phone,
		PhotoURL:  payload.PhotoURL,
		Bio:       payload.Bio,
	}

	// If a new password is provided, hash it before updating
	if payload.Password != nil && *payload.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*payload.Password), bcrypt.DefaultCost)
		if err != nil {
			r.log.Error("Error hashing new password during update", map[string]any{"error": err.Error(), "user_id": id})
			r.handleResponse(c, InternalServerError, "Could not process password update", err.Error())
			return
		}
		hashedPasswordStr := string(hashedPassword)
		updateReq.HashedPassword = &hashedPasswordStr
	}

	err = r.userUC.Update(c, updateReq)
	if err != nil {
		r.log.Error("Error while updating user", map[string]any{"error": err.Error(), "user_id": id})
		if strings.Contains(err.Error(), "no user found with ID") {
			r.handleResponse(c, NotFound, fmt.Sprintf("User with ID %d not found", id), nil)
			return
		}
		if strings.Contains(err.Error(), "no fields to update") {
			r.handleResponse(c, BadRequest, "No fields provided to update", err.Error())
			return
		}
		r.handleResponse(c, InternalServerError, "Error while updating user", err.Error())
		return
	}

	r.handleResponse(c, OK, "User updated successfully", nil)
}

// @Router /users/{id} [delete]
// @Summary Delete a user
// @Description Deletes a user by their unique ID
// @Tags USERS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID to delete"
// @Success 200 {object} Response{data=string} "User deleted successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - User not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *userRoutes) deleteUser(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid user ID format", nil)
		return
	}

	err = r.userUC.Delete(c, id)
	if err != nil {
		r.log.Error("Error while deleting user", map[string]any{"error": err.Error(), "user_id": id})
		if strings.Contains(err.Error(), "no user found with ID") {
			r.handleResponse(c, NotFound, fmt.Sprintf("User with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error while deleting user", err.Error())
		return
	}

	r.handleResponse(c, OK, "User deleted successfully", nil)
}
