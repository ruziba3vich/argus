// Package postgres implements postgres connection.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	pgxadapter "github.com/pckhoi/casbin-pgx-adapter/v2"
	"github.com/ruziba3vich/argus/internal/entity"
	"github.com/ruziba3vich/argus/internal/pkg/config"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	_defaultMaxPoolSize  = 2
	_defaultConnAttempts = 10
	_defaultConnTimeout  = time.Second
)

// Postgres -.
type Postgres struct {
	maxPoolSize  int
	connAttempts int
	connTimeout  time.Duration

	Sq      *Squirrel
	Builder squirrel.StatementBuilderType
	*pgxpool.Pool

	Adapter *pgxadapter.Adapter
}

// New -.
func New(config *config.Config, opts ...Option) (*Postgres, error) {
	pg := &Postgres{
		maxPoolSize:  _defaultMaxPoolSize,
		connAttempts: _defaultConnAttempts,
		connTimeout:  _defaultConnTimeout,
	}

	// Custom options
	for _, opt := range opts {
		opt(pg)
	}

	pg.Builder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

	url := GetStrConfig(config)

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		fmt.Println("err :", err)
		return nil, fmt.Errorf("postgres - NewPostgres - pgxpool.ParseConfig: %w", err)
	}

	poolConfig.MaxConns = int32(pg.maxPoolSize) //nolint:gosec // skip integer overflow conversion int -> int32

	for pg.connAttempts > 0 {
		pg.Pool, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
		if err == nil {
			break
		}

		fmt.Println("err in : ", err)

		log.Printf("Postgres is trying to connect, attempts left: %d", pg.connAttempts)

		time.Sleep(pg.connTimeout)

		pg.connAttempts--
	}

	if err != nil {
		return nil, fmt.Errorf("postgres - NewPostgres - connAttempts == 0: %w", err)
	}

	pg.Sq = NewSquirrel()

	log.Printf("Postgres connection established to %s successfully", url)
	return pg, nil
}

// Close -.
func (p *Postgres) Close() {
	if p.Pool != nil {
		p.Pool.Close()
	}
}

func (p *Postgres) Error(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return entity.ErrorConflict
		}
	}

	if err == pgx.ErrNoRows {
		return pgx.ErrNoRows
	}
	return err
}

func (p *Postgres) ErrSQLBuild(err error, message string) error {
	return fmt.Errorf("error during sql build, %s: %w", message, err)
}

func GetStrConfig(config *config.Config) string {
	var conn []string

	if len(config.DB.Host) != 0 {
		conn = append(conn, "host="+config.DB.Host)
	}

	if len(config.DB.Port) != 0 {
		conn = append(conn, "port="+config.DB.Port)
	}

	if len(config.DB.User) != 0 {
		conn = append(conn, "user="+config.DB.User)
	}

	if len(config.DB.Password) != 0 {
		conn = append(conn, "password="+config.DB.Password)
	}

	if len(config.DB.Name) != 0 {
		conn = append(conn, "dbname="+config.DB.Name)
	}

	if len(config.DB.SSLMode) != 0 {
		conn = append(conn, "sslmode="+config.DB.SSLMode)
	}

	return strings.Join(conn, " ")
}

func GetPgxPoolConfig(config *config.Config) (*pgx.ConnConfig, error) {
	return pgx.ParseConfig(GetStrConfig(config))
}

func GetAdapter(config *config.Config) (*pgxadapter.Adapter, error) {
	connStr := GetStrConfig(config)

	pg := &Postgres{
		connAttempts: _defaultConnAttempts,
		connTimeout:  _defaultConnTimeout,
	}

	fmt.Println("Postgres connection string:", connStr)

	var (
		err     error
		adapter *pgxadapter.Adapter
	)
	for attempts := 0; attempts < pg.connAttempts; attempts++ {
		adapter, err = pgxadapter.NewAdapter(connStr, pgxadapter.WithDatabase(config.DB.Name))
		if err == nil {
			break
		}
		log.Printf("Casbin adapter connect failed, retrying… (%d left)", pg.connAttempts-attempts-1)
		time.Sleep(pg.connTimeout)
	}
	if err != nil {
		return nil, fmt.Errorf("GetAdapter: %w", err)
	}

	return adapter, nil
}
