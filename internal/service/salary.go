package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/ruziba3vich/argus/internal/entity"
	"github.com/ruziba3vich/argus/internal/pkg/postgres"
	logger "github.com/ruziba3vich/prodonik_lgger"
)

const (
	salaryTableName = "salaries"
	salaryIDKey     = "salaryID: "
)

// SalaryRepoInterface defines the interface for Salary CRUD operations.
type SalaryRepoInterface interface {
	Create(ctx context.Context, req *entity.CreateSalaryRequest) (*entity.Salary, error)
	Get(ctx context.Context, params map[string]string) (*entity.Salary, error)
	List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllSalariesResponse, error)
	Update(ctx context.Context, req *entity.UpdateSalaryRequest) error
	Delete(ctx context.Context, id int64) error
}

type salaryRepo struct {
	tableName string
	db        *postgres.Postgres
	log       logger.Logger
}

func NewSalaryRepo(db *postgres.Postgres, log logger.Logger) SalaryRepoInterface {
	return &salaryRepo{
		tableName: salaryTableName,
		db:        db,
		log:       log,
	}
}

func (p *salaryRepo) Create(ctx context.Context, req *entity.CreateSalaryRequest) (*entity.Salary, error) {
	var reqID = salaryIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("salaryRepo.Create - %s", reqID))
	}

	// Define columns and values for insertion.
	columns := []string{
		"amount", "user_id", "admin_id", "pay_date", "currency", "status",
		"created_at", "updated_at",
	}
	values := []interface{}{
		req.Amount, req.UserID, req.AdminID, req.PayDate, req.Currency, req.Status,
		time.Now().UTC(), time.Now().UTC(),
	}

	// Conditionally add nullable fields
	if req.UpdaterAdminID != nil {
		columns = append(columns, "updateradminid") // Database column name
		values = append(values, *req.UpdaterAdminID)
	}

	sqlStr, args, err := p.db.Sq.Builder.
		Insert(p.tableName).
		Columns(columns...).
		Values(values...).
		Suffix(`RETURNING id, amount, user_id, admin_id, updateradminid, pay_date, currency, status, created_at, updated_at`).
		ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" create")
	}

	var createdSalary entity.Salary
	var nullUpdaterAdminID sql.NullInt64 // For nullable updaterAdminId

	tx, err := p.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("transaction start failed: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, sqlStr, args...)
	err = row.Scan(
		&createdSalary.ID,
		&createdSalary.Amount,
		&createdSalary.UserID,
		&createdSalary.AdminID,
		&nullUpdaterAdminID, // Scan into NullInt64
		&createdSalary.PayDate,
		&createdSalary.Currency,
		&createdSalary.Status,
		&createdSalary.CreatedAt,
		&createdSalary.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	if nullUpdaterAdminID.Valid {
		createdSalary.UpdaterAdminID = &nullUpdaterAdminID.Int64
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return &createdSalary, nil
}

func (p *salaryRepo) Get(ctx context.Context, params map[string]string) (*entity.Salary, error) {
	var reqID = salaryIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("salaryRepo.Get - %s", reqID))
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("at least one filter parameter is required")
	}

	builder := p.db.Sq.Builder.Select(
		"id", "amount", "user_id", "admin_id", "updateradminid", "pay_date", "currency", "status",
		"created_at", "updated_at",
	).From(p.tableName)

	for key, value := range params {
		builder = builder.Where(squirrel.Eq{key: value})
	}

	sqlStr, args, err := builder.ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" get")
	}

	var salary entity.Salary
	var nullUpdaterAdminID sql.NullInt64

	err = p.db.QueryRow(ctx, sqlStr, args...).Scan(
		&salary.ID,
		&salary.Amount,
		&salary.UserID,
		&salary.AdminID,
		&nullUpdaterAdminID,
		&salary.PayDate,
		&salary.Currency,
		&salary.Status,
		&salary.CreatedAt,
		&salary.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	if nullUpdaterAdminID.Valid {
		salary.UpdaterAdminID = &nullUpdaterAdminID.Int64
	}

	return &salary, nil
}

func (p *salaryRepo) List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllSalariesResponse, error) {
	var reqID = salaryIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("salaryRepo.List - %s", reqID))
	}

	var salaries entity.GetAllSalariesResponse
	baseBuilder := p.db.Sq.Builder.Select(
		"id", "amount", "user_id", "admin_id", "updateradminid", "pay_date", "currency", "status",
		"created_at", "updated_at",
	).From(p.tableName)

	countBuilder := p.db.Sq.Builder.Select("COUNT(*)").From(p.tableName)

	// Apply filters
	for key, value := range filter {
		baseBuilder = baseBuilder.Where(squirrel.Eq{key: value})
		countBuilder = countBuilder.Where(squirrel.Eq{key: value})
	}

	// Apply search (e.g., by currency or status if relevant, or amount ranges)
	if search != "" {
		searchClause := squirrel.Or{
			squirrel.ILike{"currency": fmt.Sprintf("%%%s%%", search)}, // Assuming enum can be searched as string
			squirrel.ILike{"status": fmt.Sprintf("%%%s%%", search)},   // Assuming enum can be searched as string
		}
		baseBuilder = baseBuilder.Where(searchClause)
		countBuilder = countBuilder.Where(searchClause)
	}

	// Order, Limit, Offset for data query
	baseBuilder = baseBuilder.OrderBy("created_at DESC").Limit(limit).Offset(offset)

	sqlStr, args, err := baseBuilder.ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" list")
	}

	rows, err := p.db.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var salary entity.Salary
		var nullUpdaterAdminID sql.NullInt64
		if err := rows.Scan(
			&salary.ID, &salary.Amount, &salary.UserID, &salary.AdminID,
			&nullUpdaterAdminID, &salary.PayDate, &salary.Currency, &salary.Status,
			&salary.CreatedAt, &salary.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		if nullUpdaterAdminID.Valid {
			salary.UpdaterAdminID = &nullUpdaterAdminID.Int64
		}
		salaries.Items = append(salaries.Items, &salary)
	}

	// Get total count
	countSqlStr, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" count")
	}

	var totalCount uint64
	err = p.db.QueryRow(ctx, countSqlStr, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("count query error: %w", err)
	}

	salaries.Total = totalCount
	return &salaries, nil
}

func (p *salaryRepo) Update(ctx context.Context, req *entity.UpdateSalaryRequest) error {
	var reqID = salaryIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("salaryRepo.Update - %s", reqID))
	}

	if req == nil {
		return fmt.Errorf("salary update request cannot be nil")
	}
	if req.ID == 0 {
		return fmt.Errorf("salary ID is required for update")
	}

	clauses := map[string]interface{}{"updated_at": time.Now().UTC()}

	if req.Amount != nil {
		clauses["amount"] = *req.Amount
	}
	if req.UserID != nil {
		clauses["user_id"] = *req.UserID
	}
	if req.AdminID != nil {
		clauses["admin_id"] = *req.AdminID
	}
	if req.UpdaterAdminID != nil {
		clauses["updateradminid"] = *req.UpdaterAdminID
	}
	if req.PayDate != nil {
		clauses["pay_date"] = *req.PayDate
	}
	if req.Currency != nil {
		clauses["currency"] = *req.Currency
	}
	if req.Status != nil {
		clauses["status"] = *req.Status
	}

	if len(clauses) < 2 {
		return fmt.Errorf("no fields to update")
	}

	sqlStr, args, err := p.db.Sq.Builder.
		Update(p.tableName).
		SetMap(clauses).
		Where(p.db.Sq.Equal("id", req.ID)).
		ToSql()
	if err != nil {
		return p.db.ErrSQLBuild(err, p.tableName+" update")
	}

	tx, err := p.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("transaction start failed: %w", err)
	}
	defer tx.Rollback(ctx)

	commandTag, err := tx.Exec(ctx, sqlStr, args...)
	if err != nil {
		return p.db.Error(err)
	}

	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("no salary found with ID %d", req.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (p *salaryRepo) Delete(ctx context.Context, id int64) error {
	var reqID = salaryIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("salaryRepo.Delete - %s", reqID))
	}

	if id == 0 {
		return fmt.Errorf("salary ID is required for deletion")
	}

	sqlStr, args, err := p.db.Sq.Builder.
		Delete(p.tableName).
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("delete query build error: %w", err)
	}

	commandTag, err := p.db.Exec(ctx, sqlStr, args...)
	if err != nil {
		return p.db.Error(err)
	}

	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("no salary found with ID %d", id)
	}

	return nil
}
