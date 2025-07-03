package api

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/ruziba3vich/argus/api/middleware"
	handlers "github.com/ruziba3vich/argus/internal/https"
	v1 "github.com/ruziba3vich/argus/internal/https/v1"
	"github.com/ruziba3vich/argus/internal/infrastructure/minio"
	"github.com/ruziba3vich/argus/internal/pkg/config"
	"github.com/ruziba3vich/argus/internal/postgres"
	"github.com/ruziba3vich/argus/internal/service"
	lgg "github.com/ruziba3vich/prodonik_lgger"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type RouteOption struct {
	Config *config.Config
	Logger *lgg.Logger

	Attendance   service.AttendanceRepoInterface
	Bonus        service.BonusesRepoInterface
	File         service.FileRepoInterface
	Salary       service.SalaryRepoInterface
	Task         service.TaskRepoInterface
	User         service.UserRepoInterface
	Server       *http.Server
	ShutdownOTLP func() error

	DB    *postgres.Postgres
	MinIO *minio.MinIOClient
}

// NewRouter -.
// Swagger spec:
// @title        Argus APIs
// @description  Argus api routes
// @version     1.0
// @BasePath    /v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func NewRouter(option *RouteOption) *gin.Engine {
	handleOption := &handlers.HandlerOption{
		Config: option.Config,
		Logger: option.Logger,

		Attendance:   option.Attendance,
		Bonuses:      option.Bonus,
		File:         option.File,
		Salary:       option.Salary,
		Task:         option.Task,
		User:         option.User,
		MinIO:        option.MinIO,
		DB:           option.DB,
		ShutdownOTLP: option.ShutdownOTLP,
		Server:       option.Server,
	}

	app := gin.New()

	app.Use(gin.Logger())
	app.Use(gin.Recovery())

	app.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			// Accept all origins
			return true
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-Id", "Device-id", "User-Agent", "Accept-Language", "Accept-Encoding"},
		ExposeHeaders:    []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// Swagger
	app.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	apiV1Group := app.Group("/v1")

	apiV1Group.Use(middleware.AuthContext(option.Config.Token.SigningKey))

	routeRegistrars := []func(*gin.RouterGroup, *handlers.HandlerOption){
		v1.NewAttendanceRoutes,
		v1.NewBonusesRoutes,
		v1.NewFileRoutes,
		v1.NewSalaryRoutes,
		v1.NewTaskRoutes,
		v1.NewUserRoutes,
	}

	for i := range routeRegistrars {
		routeRegistrars[i](apiV1Group, handleOption)
	}

	return app
}
