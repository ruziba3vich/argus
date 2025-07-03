package v1

import (
	"fmt"
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
)

// HandlerOption would be extended to include the new dependency.
// This is shown here for context but would be defined once in your project.
/*
type HandlerOption struct {
	Attendance service.AttendanceRepoInterface
	Bonuses    service.BonusesRepoInterface
	File       service.FileRepoInterface // Added dependency
	Logger     logger.Logger
	Config     *config.Config
	Enforcer   *casbin.CachedEnforcer
}
*/

type fileRoutes struct {
	handlers.BaseHandler
	fileUC   service.FileRepoInterface
	log      *logger.Logger
	cfg      *config.Config
	enforcer *casbin.CachedEnforcer
}

// NewFileRoutes sets up the routes for file management.
func NewFileRoutes(apiV1Group *gin.RouterGroup, option *handlers.HandlerOption) {
	r := &fileRoutes{
		fileUC:   option.File, // Assuming option.File exists
		log:      option.Logger,
		cfg:      option.Config,
		enforcer: option.Enforcer,
	}

	// Define authorization policies for file endpoints
	policies := [][]string{
		{"admin", "/v1/files/", "GET"},
		{"admin", "/v1/files/", "POST"},
		{"admin", "/v1/files/", "PUT"},
		{"admin", "/v1/files/", "DELETE"},
		// You might add user-specific policies if they can access their own files
		// For example: {"user", "/v1/files/", "GET"}
	}

	for _, policy := range policies {
		_, err := option.Enforcer.AddPolicy(policy)
		if err != nil {
			option.Logger.Error("error during file enforcer add policies", map[string]any{"error": err.Error()})
		}
	}

	fileGroup := apiV1Group.Group("/files")
	{
		// fileGroup.Use(middleware.Authorizer(option.Enforcer, option.Logger))

		fileGroup.POST("", r.createFile)
		fileGroup.GET("/:id", r.getFileByID)
		fileGroup.GET("", r.getAllFiles)
		fileGroup.PUT("/:id", r.updateFile)
		fileGroup.DELETE("/:id", r.deleteFile)
	}
}

// handleResponse is a generic response handler.
// In a real project, this would be a method of a common BaseHandler.
func (r *fileRoutes) handleResponse(c *gin.Context, status Status, customMessage any, data any) {
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

// @Router /files [post]
// @Summary Create a new file record
// @Description Creates a new file record, optionally linked to a task
// @Tags FILES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param file body entity.CreateFileRequest true "File details"
// @Success 201 {object} Response{data=entity.File} "File created successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid input or missing data"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *fileRoutes) createFile(c *gin.Context) {
	var req entity.CreateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Name == "" {
		r.handleResponse(c, BadRequest, "Missing required field: name", nil)
		return
	}

	createdFile, err := r.fileUC.Create(c, &req)
	if err != nil {
		r.log.Error("Error while creating file", map[string]any{"error": err.Error()})
		r.handleResponse(c, InternalServerError, "Error while creating file", err.Error())
		return
	}

	r.handleResponse(c, Created, "File created successfully", createdFile)
}

// @Router /files/{id} [get]
// @Summary Get a file by ID
// @Description Retrieves a single file record by its unique ID
// @Tags FILES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "File ID"
// @Success 200 {object} Response{data=entity.File} "File details"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - File not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *fileRoutes) getFileByID(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid file ID format", nil)
		return
	}

	file, err := r.fileUC.Get(c, map[string]string{"id": idStr})
	if err != nil {
		r.log.Error("Error while getting file by ID", map[string]any{"error": err.Error(), "file_id": id})

		if strings.Contains(err.Error(), "no rows in result set") {
			r.handleResponse(c, NotFound, fmt.Sprintf("File with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error retrieving file", err.Error())
		return
	}

	r.handleResponse(c, OK, nil, file)
}

// @Router /files [get]
// @Summary Get all files
// @Description Retrieves a list of files with optional filtering, pagination, and search
// @Tags FILES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param task_id query int false "Filter by Task ID"
// @Param search query string false "Search by file name"
// @Param page query int false "Page number for pagination" default(1)
// @Param limit query int false "Number of records per page" default(10)
// @Success 200 {object} Response{data=entity.GetAllFilesResponse} "Successfully retrieved files"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid query parameters"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *fileRoutes) getAllFiles(c *gin.Context) {
	page, limit := helper.GetPaginationParams(c)
	search := c.Query("search")
	filter := make(map[string]string)

	if taskIDStr := c.Query("task_id"); taskIDStr != "" {
		if _, err := strconv.ParseInt(taskIDStr, 10, 64); err == nil {
			filter["taskid"] = taskIDStr
		} else {
			r.handleResponse(c, BadRequest, "Invalid task_id format", nil)
			return
		}
	}

	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	files, err := r.fileUC.List(c, uint64(limit), uint64(offset), filter, search)
	if err != nil {
		r.log.Error("Error while getting all files", map[string]any{"error": err.Error()})
		r.handleResponse(c, InternalServerError, "Error retrieving files", err.Error())
		return
	}

	r.handleResponse(c, OK, nil, files)
}

// @Router /files/{id} [put]
// @Summary Update a file record
// @Description Updates an existing file by its ID.
// @Tags FILES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "File ID to update"
// @Param file body entity.UpdateFileRequest true "Updated file details"
// @Success 200 {object} Response{data=string} "File updated successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID or request body"
// @Failure 404 {object} Response{data=string} "Not Found - File not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *fileRoutes) updateFile(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid file ID format", nil)
		return
	}

	var req entity.UpdateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	req.ID = id // Set the ID from the URL path

	err = r.fileUC.Update(c, &req)
	if err != nil {
		r.log.Error("Error while updating file", map[string]any{"error": err.Error(), "file_id": id})

		if strings.Contains(err.Error(), "no file found with ID") {
			r.handleResponse(c, NotFound, fmt.Sprintf("File with ID %d not found", id), nil)
			return
		}
		if strings.Contains(err.Error(), "no fields to update") {
			r.handleResponse(c, BadRequest, "No fields provided to update", err.Error())
			return
		}
		r.handleResponse(c, InternalServerError, "Error while updating file", err.Error())
		return
	}

	r.handleResponse(c, OK, "File updated successfully", nil)
}

// @Router /files/{id} [delete]
// @Summary Delete a file
// @Description Deletes a file by its unique ID
// @Tags FILES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "File ID to delete"
// @Success 200 {object} Response{data=string} "File deleted successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - File not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *fileRoutes) deleteFile(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid file ID format", nil)
		return
	}

	err = r.fileUC.Delete(c, id)
	if err != nil {
		r.log.Error("Error while deleting file", map[string]any{"error": err.Error(), "file_id": id})

		if strings.Contains(err.Error(), "no file found with ID") {
			r.handleResponse(c, NotFound, fmt.Sprintf("File with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error while deleting file", err.Error())
		return
	}

	r.handleResponse(c, OK, "File deleted successfully", nil)
}
