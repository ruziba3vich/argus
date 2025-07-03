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
	"github.com/ruziba3vich/argus/internal/postgres"
	logger "github.com/ruziba3vich/prodonik_lgger"
)

const (
	bonusesTableName = "bonuses"
	bonusesIDKey     = "bonusID: "
)

// BonusesRepoInterface defines the interface for Bonus CRUD operations.
type BonusesRepoInterface interface {
	Create(ctx context.Context, req *entity.CreateBonusRequest) (*entity.Bonus, error)
	Get(ctx context.Context, params map[string]string) (*entity.Bonus, error)
	List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllBonusesResponse, error)
	Update(ctx context.Context, req *entity.UpdateBonusRequest) error
	Delete(ctx context.Context, id int64) error
}

type bonusesRepo struct {
	tableName string
	db        *postgres.Postgres
	log       *logger.Logger
}

func NewBonusesRepo(db *postgres.Postgres, log *logger.Logger) BonusesRepoInterface {
	return &bonusesRepo{
		tableName: bonusesTableName,
		db:        db,
		log:       log,
	}
}

func (p *bonusesRepo) Create(ctx context.Context, req *entity.CreateBonusRequest) (*entity.Bonus, error) {
	var reqID = bonusesIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("bonusesRepo.Create - %s", reqID))
	}

	// Define columns and values for insertion.
	// Always include non-nullable fields.
	columns := []string{
		"user_id", "amount", "currency",
		"created_at", "updated_at",
	}
	values := []interface{}{
		req.UserID, req.Amount, req.Currency,
		time.Now().UTC(), time.Now().UTC(),
	}

	// Conditionally add nullable fields
	if req.SuperAdminID != nil {
		columns = append(columns, "superadminid") // Database column name
		values = append(values, *req.SuperAdminID)
	}
	if req.Reason != nil {
		columns = append(columns, "reason")
		values = append(values, *req.Reason)
	}

	sqlStr, args, err := p.db.Sq.Builder.
		Insert(p.tableName).
		Columns(columns...).
		Values(values...).
		Suffix(`RETURNING id, superadminid, user_id, amount, currency, reason, created_at, updated_at`).
		ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" create")
	}

	var createdBonus entity.Bonus
	// Use sql.Null* types for scanning nullable columns
	var (
		nullSuperAdminID sql.NullInt64
		nullReason       sql.NullString
	)

	tx, err := p.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("transaction start failed: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, sqlStr, args...)
	err = row.Scan(
		&createdBonus.ID,
		&nullSuperAdminID,
		&createdBonus.UserID,
		&createdBonus.Amount,
		&createdBonus.Currency,
		&nullReason,
		&createdBonus.CreatedAt,
		&createdBonus.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	// Assign scanned null values to pointers in the entity struct
	if nullSuperAdminID.Valid {
		createdBonus.SuperAdminID = &nullSuperAdminID.Int64
	}
	if nullReason.Valid {
		createdBonus.Reason = &nullReason.String
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return &createdBonus, nil
}

func (p *bonusesRepo) Get(ctx context.Context, params map[string]string) (*entity.Bonus, error) {
	var reqID = bonusesIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("bonusesRepo.Get - %s", reqID))
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("at least one filter parameter is required")
	}

	builder := p.db.Sq.Builder.Select(
		"id", "superadminid", "user_id", "amount", "currency", "reason",
		"created_at", "updated_at",
	).From(p.tableName)

	for key, value := range params {
		builder = builder.Where(squirrel.Eq{key: value})
	}

	sqlStr, args, err := builder.ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" get")
	}

	var bonus entity.Bonus
	var (
		nullSuperAdminID sql.NullInt64
		nullReason       sql.NullString
	)

	err = p.db.QueryRow(ctx, sqlStr, args...).Scan(
		&bonus.ID,
		&nullSuperAdminID,
		&bonus.UserID,
		&bonus.Amount,
		&bonus.Currency,
		&nullReason,
		&bonus.CreatedAt,
		&bonus.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	if nullSuperAdminID.Valid {
		bonus.SuperAdminID = &nullSuperAdminID.Int64
	}
	if nullReason.Valid {
		bonus.Reason = &nullReason.String
	}

	return &bonus, nil
}

func (p *bonusesRepo) List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllBonusesResponse, error) {
	var reqID = bonusesIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("bonusesRepo.List - %s", reqID))
	}

	var bonuses entity.GetAllBonusesResponse
	baseBuilder := p.db.Sq.Builder.Select(
		"id", "superadminid", "user_id", "amount", "currency", "reason",
		"created_at", "updated_at",
	).From(p.tableName)

	countBuilder := p.db.Sq.Builder.Select("COUNT(*)").From(p.tableName)

	// Apply filters
	for key, value := range filter {
		baseBuilder = baseBuilder.Where(squirrel.Eq{key: value})
		countBuilder = countBuilder.Where(squirrel.Eq{key: value})
	}

	// Apply search (e.g., on reason or user_id if searchable)
	if search != "" {
		// Example: searching by reason or converting user_id to string for search
		// Note: Searching by `user_id` or `superadminid` as string might require casting in SQL.
		// For simplicity, let's search only on 'reason' if it's text.
		searchClause := squirrel.Or{
			squirrel.ILike{"reason": fmt.Sprintf("%%%s%%", search)},
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
		var bonus entity.Bonus
		var (
			nullSuperAdminID sql.NullInt64
			nullReason       sql.NullString
		)

		if err := rows.Scan(
			&bonus.ID, &nullSuperAdminID, &bonus.UserID, &bonus.Amount,
			&bonus.Currency, &nullReason, &bonus.CreatedAt, &bonus.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		if nullSuperAdminID.Valid {
			bonus.SuperAdminID = &nullSuperAdminID.Int64
		}
		if nullReason.Valid {
			bonus.Reason = &nullReason.String
		}
		bonuses.Items = append(bonuses.Items, &bonus)
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

	bonuses.Total = totalCount
	return &bonuses, nil
}

func (p *bonusesRepo) Update(ctx context.Context, req *entity.UpdateBonusRequest) error {
	var reqID = bonusesIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("bonusesRepo.Update - %s", reqID))
	}

	if req == nil {
		return fmt.Errorf("bonus update request cannot be nil")
	}
	if req.ID == 0 {
		return fmt.Errorf("bonus ID is required for update")
	}

	clauses := map[string]interface{}{"updated_at": time.Now().UTC()}

	if req.SuperAdminID != nil {
		clauses["superadminid"] = *req.SuperAdminID
	}
	if req.UserID != nil {
		clauses["user_id"] = *req.UserID
	}
	if req.Amount != nil {
		clauses["amount"] = *req.Amount
	}
	if req.Currency != nil {
		clauses["currency"] = *req.Currency
	}
	if req.Reason != nil {
		clauses["reason"] = *req.Reason
	}

	if len(clauses) <= 1 { // Only updated_at means no actual fields were passed
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
		return fmt.Errorf("no bonus found with ID %d", req.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (p *bonusesRepo) Delete(ctx context.Context, id int64) error {
	var reqID = bonusesIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("bonusesRepo.Delete - %s", reqID))
	}

	if id == 0 {
		return fmt.Errorf("bonus ID is required for deletion")
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
		return fmt.Errorf("no bonus found with ID %d", id)
	}

	return nil
}
