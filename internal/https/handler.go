package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/ruziba3vich/argus/api/middleware"
	"github.com/ruziba3vich/argus/internal/infrastructure/minio"
	"github.com/ruziba3vich/argus/internal/pkg/config"
	"github.com/ruziba3vich/argus/internal/pkg/token"
	"github.com/ruziba3vich/argus/internal/postgres"
	"github.com/ruziba3vich/argus/internal/service"
	logger "github.com/ruziba3vich/prodonik_lgger"

	"golang.org/x/net/context"
)

// HandlerOption represents the dependencies for all handlers
type HandlerOption struct {
	Attendance     service.AttendanceRepoInterface
	Bonuses        service.BonusesRepoInterface
	File           service.FileRepoInterface
	Salary         service.SalaryRepoInterface
	Task           service.TaskRepoInterface
	User           service.UserRepoInterface
	Logger         *logger.Logger
	Config         *config.Config
	Enforcer       *casbin.CachedEnforcer
	ContextTimeout time.Duration
	Server         *http.Server
	ShutdownOTLP   func() error

	// DEPENDENCIES GO HERE

	DB    *postgres.Postgres
	MinIO *minio.MinIOClient
}

type BaseHandler struct {
	Config *config.Config
	// Client grpcClients.ServiceClient
}

func (h *BaseHandler) GetAuthData(ctx context.Context) (map[string]string, bool) {
	// tracing
	// ctx, span := otlp.Start(ctx, "handler", "GetAuthData")
	// defer span.End()

	data, ok := ctx.Value(middleware.RequestAuthCtx).(map[string]string)
	return data, ok
}

func (h *BaseHandler) GetUserID(ctx *gin.Context) string {
	jwtToken := ctx.Request.Header.Get("Authorization")

	fmt.Println("T>>>>>>>>>>>>>>>>>>>>>> ", jwtToken, " <<<<<<<<<>>>>>>> ", h.Config.Token.SigningKey)

	claims, err := token.ExtractClaims(h.Config.Token.SigningKey, jwtToken)

	if err != nil {
		fmt.Println("error while extracting claims from token")
		return ""
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		fmt.Println("error while extracting userID from claims ")
		return ""
	}

	return userID
}
