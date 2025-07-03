package app

import (
	"context"
	"net/http"

	"github.com/ruziba3vich/argus/internal/infrastructure/minio"
	"github.com/ruziba3vich/argus/internal/pkg/config"
	"github.com/ruziba3vich/argus/internal/pkg/otlp"
	"github.com/ruziba3vich/argus/internal/postgres"
	"github.com/ruziba3vich/argus/internal/service"
	logger "github.com/ruziba3vich/prodonik_lgger"
)

type App struct {
	Config       *config.Config
	Logger       *logger.Logger
	attendance   service.AttendanceRepoInterface
	bonus        service.BonusesRepoInterface
	file         service.FileRepoInterface
	salary       service.SalaryRepoInterface
	task         service.TaskRepoInterface
	user         service.UserRepoInterface
	server       *http.Server
	ShutdownOTLP func() error

	DB    *postgres.Postgres
	minIO *minio.MinIOClient
}

func (a *App) Stop() {

	// close database
	a.DB.Close()

	// close grpc connections
	// a.Clients.Close()

	// shutdown server http
	if err := a.server.Shutdown(context.Background()); err != nil {
		a.Logger.Error("shutdown server http ", map[string]any{"error": err.Error()})
	}

	// shutdown otlp collector
	if err := a.ShutdownOTLP(); err != nil {
		a.Logger.Error("shutdown otlp collector", map[string]any{"error": err.Error()})
	}

}

func NewApp(cfg *config.Config) (*App, error) {
	l, err := logger.NewLogger("/app.log")

	// postgres init
	db, err := postgres.New(cfg, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		return nil, err
	}

	// otlp collector init
	shutdownOTLP, err := otlp.InitOTLPProvider(cfg)
	if err != nil {
		return nil, err
	}

	pocket := builder(db, l)

	attendanceUC := pocket("attendance").(service.AttendanceRepoInterface)
	bonusUC := pocket("bonus").(service.BonusesRepoInterface)
	fileUC := pocket("file").(service.FileRepoInterface)
	salaryUC := pocket("salary").(service.SalaryRepoInterface)
	taskUC := pocket("task").(service.TaskRepoInterface)
	userUC := pocket("user").(service.UserRepoInterface)

	minIO, err := minio.NewMinIOClient(cfg)
	if err != nil {
		l.Error("minio.NewMinIOClient failed", map[string]any{"error": err})
		return nil, err
	}

	return &App{
		Config:       cfg,
		Logger:       l,
		attendance:   attendanceUC,
		bonus:        bonusUC,
		file:         fileUC,
		salary:       salaryUC,
		task:         taskUC,
		user:         userUC,
		minIO:        minIO,
		ShutdownOTLP: shutdownOTLP,
	}, nil
}

func NewService[T any](constructor func(*postgres.Postgres, *logger.Logger) T) func(*postgres.Postgres, *logger.Logger) any {
	return func(db *postgres.Postgres, log *logger.Logger) any {
		return constructor(db, log)
	}
}

var constructors = map[string]func(*postgres.Postgres, *logger.Logger) any{
	"bonus":      NewService(service.NewBonusesRepo),
	"attendance": NewService(service.NewAttendanceRepo),
	"file":       NewService(service.NewFileRepo),
	"salary":     NewService(service.NewSalaryRepo),
	"task":       NewService(service.NewTaskRepo),
	"user":       NewService(service.NewUserRepo),
}

func builder(db *postgres.Postgres, log *logger.Logger) func(serviceName string) any {
	return func(serviceName string) any {
		return constructors[serviceName](db, log)
	}
}
