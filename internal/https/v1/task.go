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
	File       service.FileRepoInterface
	Salary     service.SalaryRepoInterface
	Task       service.TaskRepoInterface // Added dependency
	Logger     logger.Logger
	Config     *config.Config
	Enforcer   *casbin.CachedEnforcer
}
*/

type taskRoutes struct {
	handlers.BaseHandler // Inherit common methods like handleResponse
	taskUC               service.TaskRepoInterface
	log                  *logger.Logger
	cfg                  *config.Config
	enforcer             *casbin.CachedEnforcer
}

// NewTaskRoutes sets up the routes for task management.
func NewTaskRoutes(apiV1Group *gin.RouterGroup, option *handlers.HandlerOption) {
	r := &taskRoutes{
		taskUC:   option.Task, // Assuming option.Task exists
		log:      option.Logger,
		cfg:      option.Config,
		enforcer: option.Enforcer,
	}

	// Define authorization policies for task endpoints
	policies := [][]string{
		{"admin", "/v1/tasks/", "GET"},
		{"admin", "/v1/tasks/", "POST"},
		{"admin", "/v1/tasks/", "PUT"},
		{"admin", "/v1/tasks/", "DELETE"},
		{"user", "/v1/tasks/", "GET"}, // Users can view tasks assigned to them
		{"user", "/v1/tasks/", "PUT"}, // Users can update the status of their own tasks
	}

	for _, policy := range policies {
		_, err := option.Enforcer.AddPolicy(policy)
		if err != nil {
			option.Logger.Error("error during task enforcer add policies", map[string]any{"error": err.Error()})
		}
	}

	taskGroup := apiV1Group.Group("/tasks")
	{
		// taskGroup.Use(middleware.Authorizer(option.Enforcer, option.Logger))

		taskGroup.POST("", r.createTask)
		taskGroup.GET("/:id", r.getTaskByID)
		taskGroup.GET("", r.getAllTasks)
		taskGroup.PUT("/:id", r.updateTask)
		taskGroup.DELETE("/:id", r.deleteTask)
	}
}

// handleResponse is a generic response handler.
// In a real project, this would be a method of a common BaseHandler.
func (r *taskRoutes) handleResponse(c *gin.Context, status Status, customMessage any, data any) {
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

// @Router /tasks [post]
// @Summary Create a new task
// @Description Creates a new task and can assign it to a user
// @Tags TASKS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param task body entity.CreateTaskRequest true "Task details"
// @Success 201 {object} Response{data=entity.Task} "Task created successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid input or missing data"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *taskRoutes) createTask(c *gin.Context) {
	var req entity.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	if req.AdminID == 0 || req.Title == "" || req.Status == "" || req.Priority == "" {
		r.handleResponse(c, BadRequest, "Missing required fields: admin_id, title, status, priority", nil)
		return
	}

	createdTask, err := r.taskUC.Create(c, &req)
	if err != nil {
		r.log.Error("Error while creating task", map[string]any{"error": err.Error()})
		r.handleResponse(c, InternalServerError, "Error while creating task", err.Error())
		return
	}

	r.handleResponse(c, Created, "Task created successfully", createdTask)
}

// @Router /tasks/{id} [get]
// @Summary Get a task by ID
// @Description Retrieves a single task by its unique ID
// @Tags TASKS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Task ID"
// @Success 200 {object} Response{data=entity.Task} "Task details"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - Task not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *taskRoutes) getTaskByID(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid task ID format", nil)
		return
	}

	task, err := r.taskUC.Get(c, map[string]string{"id": idStr})
	if err != nil {
		r.log.Error("Error while getting task by ID", map[string]any{"error": err.Error(), "task_id": id})

		if strings.Contains(err.Error(), "no rows in result set") {
			r.handleResponse(c, NotFound, fmt.Sprintf("Task with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error retrieving task", err.Error())
		return
	}

	r.handleResponse(c, OK, nil, task)
}

// @Router /tasks [get]
// @Summary Get all tasks
// @Description Retrieves a list of tasks with optional filtering, pagination, and search
// @Tags TASKS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param assigned_to query int false "Filter by Assigned User ID"
// @Param admin_id query int false "Filter by Admin Creator ID"
// @Param status query string false "Filter by Status (e.g., 'todo', 'in_progress')"
// @Param priority query string false "Filter by Priority (e.g., 'high', 'low')"
// @Param search query string false "Search by title, description, status, or priority"
// @Param page query int false "Page number for pagination" default(1)
// @Param limit query int false "Number of records per page" default(10)
// @Success 200 {object} Response{data=entity.GetAllTasksResponse} "Successfully retrieved tasks"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid query parameters"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *taskRoutes) getAllTasks(c *gin.Context) {
	page, limit := helper.GetPaginationParams(c)
	search := c.Query("search")
	filter := make(map[string]string)

	if assignedToStr := c.Query("assigned_to"); assignedToStr != "" {
		if _, err := strconv.ParseInt(assignedToStr, 10, 64); err == nil {
			filter["assignedto"] = assignedToStr
		} else {
			r.handleResponse(c, BadRequest, "Invalid assigned_to format", nil)
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
	if priority := c.Query("priority"); priority != "" {
		filter["priority"] = priority
	}

	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	tasks, err := r.taskUC.List(c, uint64(limit), uint64(offset), filter, search)
	if err != nil {
		r.log.Error("Error while getting all tasks", map[string]any{"error": err.Error()})
		r.handleResponse(c, InternalServerError, "Error retrieving tasks", err.Error())
		return
	}

	r.handleResponse(c, OK, nil, tasks)
}

// @Router /tasks/{id} [put]
// @Summary Update a task
// @Description Updates an existing task by its ID.
// @Tags TASKS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Task ID to update"
// @Param task body entity.UpdateTaskRequest true "Updated task details"
// @Success 200 {object} Response{data=string} "Task updated successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID or request body"
// @Failure 404 {object} Response{data=string} "Not Found - Task not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *taskRoutes) updateTask(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid task ID format", nil)
		return
	}

	var req entity.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	req.ID = id // Set the ID from the URL path

	err = r.taskUC.Update(c, &req)
	if err != nil {
		r.log.Error("Error while updating task", map[string]any{"error": err.Error(), "task_id": id})

		if strings.Contains(err.Error(), "no task found with ID") {
			r.handleResponse(c, NotFound, fmt.Sprintf("Task with ID %d not found", id), nil)
			return
		}
		if strings.Contains(err.Error(), "no fields to update") {
			r.handleResponse(c, BadRequest, "No fields provided to update", err.Error())
			return
		}
		r.handleResponse(c, InternalServerError, "Error while updating task", err.Error())
		return
	}

	r.handleResponse(c, OK, "Task updated successfully", nil)
}

// @Router /tasks/{id} [delete]
// @Summary Delete a task
// @Description Deletes a task by its unique ID (Admins only)
// @Tags TASKS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Task ID to delete"
// @Success 200 {object} Response{data=string} "Task deleted successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - Task not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *taskRoutes) deleteTask(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid task ID format", nil)
		return
	}

	err = r.taskUC.Delete(c, id)
	if err != nil {
		r.log.Error("Error while deleting task", map[string]any{"error": err.Error(), "task_id": id})

		if strings.Contains(err.Error(), "no task found with ID") {
			r.handleResponse(c, NotFound, fmt.Sprintf("Task with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error while deleting task", err.Error())
		return
	}

	r.handleResponse(c, OK, "Task deleted successfully", nil)
}
