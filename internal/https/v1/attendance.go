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

type Response struct {
	Status        string      `json:"status"`
	Description   string      `json:"description"`
	Data          interface{} `json:"data"`
	CustomMessage interface{} `json:"custom_message,omitempty"`
}

type Status struct {
	Code          int
	Status        string
	Description   string
	CustomMessage interface{}
}

var (
	OK                  = Status{Code: 200, Status: "OK", Description: "Request successful"}
	Created             = Status{Code: 201, Status: "Created", Description: "Resource created successfully"}
	BadRequest          = Status{Code: 400, Status: "Bad Request", Description: "Invalid request data"}
	NotFound            = Status{Code: 404, Status: "Not Found", Description: "Resource not found"}
	InternalServerError = Status{Code: 500, Status: "Internal Server Error", Description: "An unexpected error occurred"}
)

var ErrorNotFound = fmt.Errorf("record not found") // Simple error simulation
// --- END SIMULATED IMPORTS ---

type attendanceRoutes struct {
	handlers.BaseHandler // Inherit common methods like handleResponse
	attendanceUC         service.AttendanceRepoInterface
	log                  *logger.Logger
	cfg                  *config.Config
	enforcer             *casbin.CachedEnforcer
}

func NewAttendanceRoutes(apiV1Group *gin.RouterGroup, option *handlers.HandlerOption) {
	r := &attendanceRoutes{
		attendanceUC: option.Attendance,
		log:          option.Logger,
		cfg:          option.Config,
		enforcer:     option.Enforcer,
	}

	policies := [][]string{
		{"admin", "/v1/attendance/", "GET"},
		{"admin", "/v1/attendance/", "POST"},
		{"admin", "/v1/attendance/", "PUT"},
		{"admin", "/v1/attendance/", "DELETE"},
		{"user", "/v1/attendance/", "GET"},
	}

	for _, policy := range policies {
		_, err := option.Enforcer.AddPolicy(policy)
		if err != nil {
			option.Logger.Error("error during attendance enforcer add policies", map[string]any{"error": err.Error()})
		}
	}

	attendanceGroup := apiV1Group.Group("/attendance")
	{
		// attendanceGroup.Use(middleware.Authorizer(option.Enforcer, option.Logger)) // Uncomment if you have an Authorizer middleware

		attendanceGroup.POST("", r.createAttendance)
		attendanceGroup.GET("/:id", r.getAttendanceByID)
		attendanceGroup.GET("/", r.getAllAttendances)
		attendanceGroup.PUT("/:id", r.updateAttendance)
		attendanceGroup.DELETE("/:id", r.deleteAttendance)
	}
}

// handleResponse is copied from your cardRoutes for self-containment.
// In a real project, this would likely be a method of a common BaseHandler.
func (r *attendanceRoutes) handleResponse(c *gin.Context, status Status, customMessage any, data any) {
	if status.Code >= 400 {
		if data == nil {
			data = customMessage
		}

		reqID, exists := c.Get(middleware.RequestIDHeader) // Use your middleware.RequestIDKey
		if exists {
			data = fmt.Errorf("requestID : %s, %v", reqID, data)
		}

		// Log errors for server-side
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

// @Router /attendance [post]
// @Summary Create a new attendance record
// @Description Creates a new attendance record for a user
// @Tags ATTENDANCE
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param attendance body entity.CreateAttendanceRequest true "Attendance record details"
// @Success 201 {object} Response{data=entity.Attendance} "Attendance record created successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid input or missing data"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *attendanceRoutes) createAttendance(c *gin.Context) {
	var req entity.CreateAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	// Basic validation, more specific validation should be in a separate validator or usecase layer
	if req.UserID == 0 || req.Date.IsZero() || req.InTime.IsZero() || req.Status == "" {
		r.handleResponse(c, BadRequest, "Missing required fields: user_id, date, intime, status", nil)
		return
	}

	createdAttendance, err := r.attendanceUC.Create(c, &req)
	if err != nil {
		r.log.Error("Error while creating attendance", map[string]any{"error": err.Error()})
		r.handleResponse(c, InternalServerError, "Error while creating attendance record", err.Error())
		return
	}
	r.handleResponse(c, Created, "Attendance record created successfully", createdAttendance)
}

// @Router /attendance/{id} [get]
// @Summary Get an attendance record by ID
// @Description Retrieves a single attendance record by its unique ID
// @Tags ATTENDANCE
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Attendance ID"
// @Success 200 {object} Response{data=entity.Attendance} "Attendance record details"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - Attendance record not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *attendanceRoutes) getAttendanceByID(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid attendance ID format", nil)
		return
	}

	attendance, err := r.attendanceUC.Get(c, map[string]string{"id": idStr})
	if err != nil {
		r.log.Error("Error while getting attendance by ID",
			map[string]any{"error": err.Error(), "attendance_id": id})

		// Check for "no rows in result set" error, which usually means Not Found
		if strings.Contains(err.Error(), "no rows in result set") || err == ErrorNotFound { // assuming ErrorNotFound if applicable
			r.handleResponse(c, NotFound, fmt.Sprintf("Attendance record with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error retrieving attendance record", err.Error())
		return
	}

	r.handleResponse(c, OK, nil, attendance)
}

// @Router /attendance [get]
// @Summary Get all attendance records
// @Description Retrieves a list of attendance records with optional filtering, pagination, and search
// @Tags ATTENDANCE
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user_id query int false "Filter by User ID"
// @Param date query string false "Filter by Date (YYYY-MM-DD)"
// @Param status query string false "Filter by Status (e.g., Present, Absent)"
// @Param search query string false "Search by Status or other relevant text fields"
// @Param page query int false "Page number for pagination" default(1)
// @Param limit query int false "Number of records per page" default(10)
// @Success 200 {object} Response{data=entity.GetAllAttendancesResponse} "Successfully retrieved attendance records"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid query parameters"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *attendanceRoutes) getAllAttendances(c *gin.Context) {
	page, limit := helper.GetPaginationParams(c) // Your helper for pagination

	filter := make(map[string]string)
	search := c.Query("search")

	// Apply filters from query parameters
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if _, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
			filter["user_id"] = userIDStr
		} else {
			r.handleResponse(c, BadRequest, "Invalid user_id format", nil)
			return
		}
	}
	if dateStr := c.Query("date"); dateStr != "" {
		// Validate date format if necessary, e.g., YYYY-MM-DD
		if _, err := time.Parse("2006-01-02", dateStr); err == nil {
			filter["date"] = dateStr
		} else {
			r.handleResponse(c, BadRequest, "Invalid date format. Use YYYY-MM-DD", nil)
			return
		}
	}
	if status := c.Query("status"); status != "" {
		filter["status"] = status
	}

	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0 // Ensure offset is not negative
	}

	attendances, err := r.attendanceUC.List(c, uint64(limit), uint64(offset), filter, search)
	if err != nil {
		r.log.Error("Error while getting all attendance records", map[string]any{"error": err.Error()})
		r.handleResponse(c, InternalServerError, "Error retrieving attendance records", err.Error())
		return
	}

	r.handleResponse(c, OK, nil, attendances)
}

// @Router /attendance/{id} [put]
// @Summary Update an attendance record
// @Description Updates an existing attendance record by its ID.
// @Tags ATTENDANCE
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Attendance ID to update"
// @Param attendance body entity.UpdateAttendanceRequest true "Updated attendance record details"
// @Success 200 {object} Response{data=string} "Attendance record updated successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID or request body"
// @Failure 404 {object} Response{data=string} "Not Found - Attendance record not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *attendanceRoutes) updateAttendance(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid attendance ID format", nil)
		return
	}

	var req entity.UpdateAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	req.ID = id // Set the ID from the URL path

	err = r.attendanceUC.Update(c, &req)
	if err != nil {
		r.log.Error("Error while updating attendance",
			map[string]any{"error": err.Error(), "attendance_id": id})

		if strings.Contains(err.Error(), "no attendance record found") || err == ErrorNotFound { // Check for specific error message or common not found error
			r.handleResponse(c, NotFound, fmt.Sprintf("Attendance record with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error while updating attendance record", err.Error())
		return
	}

	r.handleResponse(c, OK, "Attendance record updated successfully", nil)
}

// @Router /attendance/{id} [delete]
// @Summary Delete an attendance record
// @Description Deletes an attendance record by its unique ID
// @Tags ATTENDANCE
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Attendance ID to delete"
// @Success 200 {object} Response{data=string} "Attendance record deleted successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - Attendance record not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *attendanceRoutes) deleteAttendance(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid attendance ID format", nil)
		return
	}

	err = r.attendanceUC.Delete(c, id)
	if err != nil {
		r.log.Error("Error while deleting attendance",
			map[string]any{"error": err.Error(), "attendance_id": id})

		if strings.Contains(err.Error(), "no attendance record found") || err == ErrorNotFound { // Check for specific error message or common not found error
			r.handleResponse(c, NotFound, fmt.Sprintf("Attendance record with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error while deleting attendance record", err.Error())
		return
	}

	r.handleResponse(c, OK, "Attendance record deleted successfully", nil)
}
