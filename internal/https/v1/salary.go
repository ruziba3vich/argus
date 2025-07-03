package v1

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/ruziba3vich/argus/api/middleware"
	"github.com/ruziba3vich/argus/internal/entity"
	handlers "github.com/ruziba3vich/argus/internal/https"
	"github.com/ruziba3vich/argus/internal/pkg/config"
	"github.com/ruziba3vich/argus/internal/pkg/helper"
	"github.com/ruziba3vich/argus/internal/service"
	logger "github.com/ruziba3vich/prodonik_lgger"
)

// HandlerOption would be extended to include the new dependency.
// This is shown here for context but would be defined once in your project.
/*
type HandlerOption struct {
	Attendance service.AttendanceRepoInterface
	Bonuses    service.BonusesRepoInterface
	File       service.FileRepoInterface
	Salary     service.SalaryRepoInterface // Added dependency
	Logger     logger.Logger
	Config     *config.Config
	Enforcer   *casbin.CachedEnforcer
}
*/

type salaryRoutes struct {
	handlers.BaseHandler // Inherit common methods like handleResponse
	salaryUC             service.SalaryRepoInterface
	log                  *logger.Logger
	cfg                  *config.Config
	enforcer             *casbin.CachedEnforcer
}

// NewSalaryRoutes sets up the routes for salary management.
func NewSalaryRoutes(apiV1Group *gin.RouterGroup, option *handlers.HandlerOption) {
	r := &salaryRoutes{
		salaryUC: option.Salary, // Assuming option.Salary exists
		log:      option.Logger,
		cfg:      option.Config,
		enforcer: option.Enforcer,
	}

	// Define authorization policies for salary endpoints
	policies := [][]string{
		{"admin", "/v1/salaries/", "GET"},
		{"admin", "/v1/salaries/", "POST"},
		{"admin", "/v1/salaries/", "PUT"},
		{"admin", "/v1/salaries/", "DELETE"},
		// Users might see their own salaries via a different, more secure route like /users/me/salaries
	}

	for _, policy := range policies {
		_, err := option.Enforcer.AddPolicy(policy)
		if err != nil {
			option.Logger.Error("error during salary enforcer add policies", map[string]any{"error": err.Error()})
		}
	}

	salaryGroup := apiV1Group.Group("/salaries")
	{
		// salaryGroup.Use(middleware.Authorizer(option.Enforcer, option.Logger))

		salaryGroup.POST("", r.createSalary)
		salaryGroup.GET("/:id", r.getSalaryByID)
		salaryGroup.GET("", r.getAllSalaries)
		salaryGroup.PUT("/:id", r.updateSalary)
		salaryGroup.DELETE("/:id", r.deleteSalary)
	}
}

// handleResponse is a generic response handler.
// In a real project, this would be a method of a common BaseHandler.
func (r *salaryRoutes) handleResponse(c *gin.Context, status Status, customMessage any, data any) {
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

// @Router /salaries [post]
// @Summary Create a new salary record
// @Description Creates a new salary record for a user
// @Tags SALARIES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param salary body entity.CreateSalaryRequest true "Salary details"
// @Success 201 {object} Response{data=entity.Salary} "Salary created successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid input or missing data"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *salaryRoutes) createSalary(c *gin.Context) {
	var req entity.CreateSalaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	if req.UserID == 0 || req.AdminID == 0 || req.Amount <= 0 || req.PayDate.IsZero() || req.Currency == "" || req.Status == "" {
		r.handleResponse(c, BadRequest, "Missing or invalid required fields: user_id, admin_id, amount, pay_date, currency, status", nil)
		return
	}

	createdSalary, err := r.salaryUC.Create(c, &req)
	if err != nil {
		r.log.Error("Error while creating salary", map[string]any{"error": err.Error()})
		r.handleResponse(c, InternalServerError, "Error while creating salary", err.Error())
		return
	}

	r.handleResponse(c, Created, "Salary created successfully", createdSalary)
}

// @Router /salaries/{id} [get]
// @Summary Get a salary record by ID
// @Description Retrieves a single salary record by its unique ID
// @Tags SALARIES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Salary ID"
// @Success 200 {object} Response{data=entity.Salary} "Salary details"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - Salary not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *salaryRoutes) getSalaryByID(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid salary ID format", nil)
		return
	}

	salary, err := r.salaryUC.Get(c, map[string]string{"id": idStr})
	if err != nil {
		r.log.Error("Error while getting salary by ID", map[string]any{"error": err.Error(), "salary_id": id})

		if strings.Contains(err.Error(), "no rows in result set") {
			r.handleResponse(c, NotFound, fmt.Sprintf("Salary with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error retrieving salary", err.Error())
		return
	}

	r.handleResponse(c, OK, nil, salary)
}

// @Router /salaries [get]
// @Summary Get all salaries
// @Description Retrieves a list of salaries with optional filtering, pagination, and search
// @Tags SALARIES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user_id query int false "Filter by User ID"
// @Param admin_id query int false "Filter by Admin ID"
// @Param status query string false "Filter by Status (e.g., 'paid', 'pending')"
// @Param pay_date query string false "Filter by Pay Date (YYYY-MM-DD)"
// @Param search query string false "Search by currency or status"
// @Param page query int false "Page number for pagination" default(1)
// @Param limit query int false "Number of records per page" default(10)
// @Success 200 {object} Response{data=entity.GetAllSalariesResponse} "Successfully retrieved salaries"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid query parameters"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *salaryRoutes) getAllSalaries(c *gin.Context) {
	page, limit := helper.GetPaginationParams(c)
	search := c.Query("search")
	filter := make(map[string]string)

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if _, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			filter["user_id"] = userIDStr
		} else {
			r.handleResponse(c, BadRequest, "Invalid user_id format", nil)
			return
		}
	}

	if adminIDStr := c.Query("admin_id"); adminIDStr != "" {
		if _, err := strconv.ParseInt(adminIDStr, 10, 64); err == nil {
			filter["admin_id"] = adminIDStr
		} else {
			r.handleResponse(c, BadRequest, "Invalid admin_id format", nil)
			return
		}
	}

	if status := c.Query("status"); status != "" {
		filter["status"] = status
	}

	if dateStr := c.Query("pay_date"); dateStr != "" {
		if _, err := time.Parse("2006-01-02", dateStr); err == nil {
			filter["pay_date"] = dateStr
		} else {
			r.handleResponse(c, BadRequest, "Invalid pay_date format. Use YYYY-MM-DD", nil)
			return
		}
	}

	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	salaries, err := r.salaryUC.List(c, uint64(limit), uint64(offset), filter, search)
	if err != nil {
		r.log.Error("Error while getting all salaries", map[string]any{"error": err.Error()})
		r.handleResponse(c, InternalServerError, "Error retrieving salaries", err.Error())
		return
	}

	r.handleResponse(c, OK, nil, salaries)
}

// @Router /salaries/{id} [put]
// @Summary Update a salary record
// @Description Updates an existing salary record by its ID.
// @Tags SALARIES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Salary ID to update"
// @Param salary body entity.UpdateSalaryRequest true "Updated salary details"
// @Success 200 {object} Response{data=string} "Salary updated successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID or request body"
// @Failure 404 {object} Response{data=string} "Not Found - Salary not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *salaryRoutes) updateSalary(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid salary ID format", nil)
		return
	}

	var req entity.UpdateSalaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	req.ID = id // Set the ID from the URL path

	err = r.salaryUC.Update(c, &req)
	if err != nil {
		r.log.Error("Error while updating salary", map[string]any{"error": err.Error(), "salary_id": id})

		if strings.Contains(err.Error(), "no salary found with ID") {
			r.handleResponse(c, NotFound, fmt.Sprintf("Salary with ID %d not found", id), nil)
			return
		}
		if strings.Contains(err.Error(), "no fields to update") {
			r.handleResponse(c, BadRequest, "No fields provided to update", err.Error())
			return
		}
		r.handleResponse(c, InternalServerError, "Error while updating salary", err.Error())
		return
	}

	r.handleResponse(c, OK, "Salary updated successfully", nil)
}

// @Router /salaries/{id} [delete]
// @Summary Delete a salary record
// @Description Deletes a salary record by its unique ID
// @Tags SALARIES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Salary ID to delete"
// @Success 200 {object} Response{data=string} "Salary deleted successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - Salary not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *salaryRoutes) deleteSalary(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid salary ID format", nil)
		return
	}

	err = r.salaryUC.Delete(c, id)
	if err != nil {
		r.log.Error("Error while deleting salary", map[string]any{"error": err.Error(), "salary_id": id})

		if strings.Contains(err.Error(), "no salary found with ID") {
			r.handleResponse(c, NotFound, fmt.Sprintf("Salary with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error while deleting salary", err.Error())
		return
	}

	r.handleResponse(c, OK, "Salary deleted successfully", nil)
}
