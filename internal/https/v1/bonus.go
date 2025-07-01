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
	Bonuses    service.BonusesRepoInterface // Added dependency
	Logger     logger.Logger
	Config     *config.Config
	Enforcer   *casbin.CachedEnforcer
}
*/

type bonusesRoutes struct {
	handlers.BaseHandler // Inherit common methods like handleResponse
	bonusesUC            service.BonusesRepoInterface
	log                  logger.Logger
	cfg                  *config.Config
	enforcer             *casbin.CachedEnforcer
}

// NewBonusesRoutes sets up the routes for bonus management.
func NewBonusesRoutes(apiV1Group *gin.RouterGroup, option *handlers.HandlerOption) {
	r := &bonusesRoutes{
		bonusesUC: option.Bonuses,
		log:       option.Logger,
		cfg:       option.Config,
		enforcer:  option.Enforcer,
	}

	// Define authorization policies for bonus endpoints
	policies := [][]string{
		{"admin", "/v1/bonuses/", "GET"},
		{"admin", "/v1/bonuses/", "POST"},
		{"admin", "/v1/bonuses/", "PUT"},
		{"admin", "/v1/bonuses/", "DELETE"},
		// Assuming regular users cannot see a list of all bonuses
		// A separate endpoint like /users/me/bonuses might be used for that
	}

	for _, policy := range policies {
		_, err := option.Enforcer.AddPolicy(policy)
		if err != nil {
			option.Logger.Error("error during bonuses enforcer add policies", map[string]any{"error": err.Error()})
		}
	}

	bonusesGroup := apiV1Group.Group("/bonuses")
	{
		// bonusesGroup.Use(middleware.Authorizer(option.Enforcer, option.Logger))

		bonusesGroup.POST("", r.createBonus)
		bonusesGroup.GET("/:id", r.getBonusByID)
		bonusesGroup.GET("", r.getAllBonuses)
		bonusesGroup.PUT("/:id", r.updateBonus)
		bonusesGroup.DELETE("/:id", r.deleteBonus)
	}
}

// handleResponse is a generic response handler.
// In a real project, this would be a method of a common BaseHandler.
func (r *bonusesRoutes) handleResponse(c *gin.Context, status Status, customMessage any, data any) {
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

// @Router /bonuses [post]
// @Summary Create a new bonus
// @Description Creates a new bonus record for a user
// @Tags BONUSES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param bonus body entity.CreateBonusRequest true "Bonus details"
// @Success 201 {object} Response{data=entity.Bonus} "Bonus created successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid input or missing data"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *bonusesRoutes) createBonus(c *gin.Context) {
	var req entity.CreateBonusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	if req.UserID == 0 || req.Amount <= 0 || req.Currency == "" {
		r.handleResponse(c, BadRequest, "Missing or invalid required fields: user_id, amount, currency", nil)
		return
	}

	createdBonus, err := r.bonusesUC.Create(c, &req)
	if err != nil {
		r.log.Error("Error while creating bonus", map[string]any{"error": err.Error()})
		r.handleResponse(c, InternalServerError, "Error while creating bonus", err.Error())
		return
	}

	r.handleResponse(c, Created, "Bonus created successfully", createdBonus)
}

// @Router /bonuses/{id} [get]
// @Summary Get a bonus by ID
// @Description Retrieves a single bonus record by its unique ID
// @Tags BONUSES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Bonus ID"
// @Success 200 {object} Response{data=entity.Bonus} "Bonus details"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - Bonus not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *bonusesRoutes) getBonusByID(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid bonus ID format", nil)
		return
	}

	bonus, err := r.bonusesUC.Get(c, map[string]string{"id": idStr})
	if err != nil {
		r.log.Error("Error while getting bonus by ID", map[string]any{"error": err.Error(), "bonus_id": id})

		if strings.Contains(err.Error(), "no rows in result set") {
			r.handleResponse(c, NotFound, fmt.Sprintf("Bonus with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error retrieving bonus", err.Error())
		return
	}

	r.handleResponse(c, OK, nil, bonus)
}

// @Router /bonuses [get]
// @Summary Get all bonuses
// @Description Retrieves a list of bonuses with optional filtering, pagination, and search
// @Tags BONUSES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user_id query int false "Filter by User ID"
// @Param superadminid query int false "Filter by Super Admin ID"
// @Param search query string false "Search by reason"
// @Param page query int false "Page number for pagination" default(1)
// @Param limit query int false "Number of records per page" default(10)
// @Success 200 {object} Response{data=entity.GetAllBonusesResponse} "Successfully retrieved bonuses"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid query parameters"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *bonusesRoutes) getAllBonuses(c *gin.Context) {
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
	if superAdminIDStr := c.Query("superadminid"); superAdminIDStr != "" {
		if _, err := strconv.ParseInt(superAdminIDStr, 10, 64); err == nil {
			filter["superadminid"] = superAdminIDStr
		} else {
			r.handleResponse(c, BadRequest, "Invalid superadminid format", nil)
			return
		}
	}

	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	bonuses, err := r.bonusesUC.List(c, uint64(limit), uint64(offset), filter, search)
	if err != nil {
		r.log.Error("Error while getting all bonuses", map[string]any{"error": err.Error()})
		r.handleResponse(c, InternalServerError, "Error retrieving bonuses", err.Error())
		return
	}

	r.handleResponse(c, OK, nil, bonuses)
}

// @Router /bonuses/{id} [put]
// @Summary Update a bonus record
// @Description Updates an existing bonus by its ID.
// @Tags BONUSES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Bonus ID to update"
// @Param bonus body entity.UpdateBonusRequest true "Updated bonus details"
// @Success 200 {object} Response{data=string} "Bonus updated successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID or request body"
// @Failure 404 {object} Response{data=string} "Not Found - Bonus not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *bonusesRoutes) updateBonus(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid bonus ID format", nil)
		return
	}

	var req entity.UpdateBonusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		r.handleResponse(c, BadRequest, "Invalid request body", err.Error())
		return
	}

	req.ID = id // Set the ID from the URL path

	err = r.bonusesUC.Update(c, &req)
	if err != nil {
		r.log.Error("Error while updating bonus", map[string]any{"error": err.Error(), "bonus_id": id})

		if strings.Contains(err.Error(), "no bonus found with ID") {
			r.handleResponse(c, NotFound, fmt.Sprintf("Bonus with ID %d not found", id), nil)
			return
		}
		if strings.Contains(err.Error(), "no fields to update") {
			r.handleResponse(c, BadRequest, "No fields provided to update", err.Error())
			return
		}
		r.handleResponse(c, InternalServerError, "Error while updating bonus", err.Error())
		return
	}

	r.handleResponse(c, OK, "Bonus updated successfully", nil)
}

// @Router /bonuses/{id} [delete]
// @Summary Delete a bonus
// @Description Deletes a bonus by its unique ID
// @Tags BONUSES
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Bonus ID to delete"
// @Success 200 {object} Response{data=string} "Bonus deleted successfully"
// @Failure 400 {object} Response{data=string} "Bad Request - Invalid ID format"
// @Failure 404 {object} Response{data=string} "Not Found - Bonus not found"
// @Failure 500 {object} Response{data=string} "Internal Server Error"
func (r *bonusesRoutes) deleteBonus(c *gin.Context) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		r.handleResponse(c, BadRequest, "Invalid bonus ID format", nil)
		return
	}

	err = r.bonusesUC.Delete(c, id)
	if err != nil {
		r.log.Error("Error while deleting bonus", map[string]any{"error": err.Error(), "bonus_id": id})

		if strings.Contains(err.Error(), "no bonus found with ID") {
			r.handleResponse(c, NotFound, fmt.Sprintf("Bonus with ID %d not found", id), nil)
			return
		}
		r.handleResponse(c, InternalServerError, "Error while deleting bonus", err.Error())
		return
	}

	r.handleResponse(c, OK, "Bonus deleted successfully", nil)
}
