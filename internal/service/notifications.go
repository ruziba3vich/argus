package service

import (
	"context" // Potentially needed for sql.Null* but 'message', 'type', 'read' are non-nullable
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
	notificationTableName = "notifications"
	notificationIDKey     = "notificationID: "
)

// NotificationRepoInterface defines the interface for Notification CRUD operations.
type NotificationRepoInterface interface {
	Create(ctx context.Context, req *entity.CreateNotificationRequest) (*entity.Notification, error)
	Get(ctx context.Context, params map[string]string) (*entity.Notification, error)
	List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllNotificationsResponse, error)
	Update(ctx context.Context, req *entity.UpdateNotificationRequest) error
	Delete(ctx context.Context, id int64) error
}

type notificationRepo struct {
	tableName string
	db        *postgres.Postgres
	log       logger.Logger
}

func NewNotificationRepo(db *postgres.Postgres, log logger.Logger) NotificationRepoInterface {
	return &notificationRepo{
		tableName: notificationTableName,
		db:        db,
		log:       log,
	}
}

func (p *notificationRepo) Create(ctx context.Context, req *entity.CreateNotificationRequest) (*entity.Notification, error) {
	var reqID = notificationIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("notificationRepo.Create - %s", reqID))
	}

	sqlStr, args, err := p.db.Sq.Builder.
		Insert(p.tableName).
		Columns("user_id", "message", "type", "read", "created_at", "updated_at").
		Values(req.UserID, req.Message, req.Type, req.Read, time.Now().UTC(), time.Now().UTC()).
		Suffix(`RETURNING id, user_id, message, type, read, created_at, updated_at`).
		ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" create")
	}

	var createdNotification entity.Notification

	tx, err := p.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("transaction start failed: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, sqlStr, args...)
	err = row.Scan(
		&createdNotification.ID,
		&createdNotification.UserID,
		&createdNotification.Message,
		&createdNotification.Type,
		&createdNotification.Read,
		&createdNotification.CreatedAt,
		&createdNotification.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return &createdNotification, nil
}

func (p *notificationRepo) Get(ctx context.Context, params map[string]string) (*entity.Notification, error) {
	var reqID = notificationIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("notificationRepo.Get - %s", reqID))
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("at least one filter parameter is required")
	}

	builder := p.db.Sq.Builder.Select(
		"id", "user_id", "message", "type", "read", "created_at", "updated_at",
	).From(p.tableName)

	for key, value := range params {
		builder = builder.Where(squirrel.Eq{key: value})
	}

	sqlStr, args, err := builder.ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" get")
	}

	var notification entity.Notification

	err = p.db.QueryRow(ctx, sqlStr, args...).Scan(
		notification.ID,
		notification.UserID,
		notification.Message,
		notification.Type,
		notification.Read,
		notification.CreatedAt,
		notification.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	return &notification, nil
}

func (p *notificationRepo) List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllNotificationsResponse, error) {
	var reqID = notificationIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("notificationRepo.List - %s", reqID))
	}

	var notifications entity.GetAllNotificationsResponse
	baseBuilder := p.db.Sq.Builder.Select(
		"id", "user_id", "message", "type", "read", "created_at", "updated_at",
	).From(p.tableName)

	countBuilder := p.db.Sq.Builder.Select("COUNT(*)").From(p.tableName)

	for key, value := range filter {
		baseBuilder = baseBuilder.Where(squirrel.Eq{key: value})
		countBuilder = countBuilder.Where(squirrel.Eq{key: value})
	}

	if search != "" {
		searchClause := squirrel.Or{
			squirrel.ILike{"message": fmt.Sprintf("%%%s%%", search)},
			squirrel.ILike{"type": fmt.Sprintf("%%%s%%", search)},
		}
		baseBuilder = baseBuilder.Where(searchClause)
		countBuilder = countBuilder.Where(searchClause)
	}

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
		var notification entity.Notification
		if err := rows.Scan(
			notification.ID, notification.UserID, notification.Message,
			notification.Type, notification.Read, notification.CreatedAt,
			notification.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		notifications.Items = append(notifications.Items, notification)
	}

	countSqlStr, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" count")
	}

	var totalCount uint64
	err = p.db.QueryRow(ctx, countSqlStr, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("count query error: %w", err)
	}

	notifications.Total = totalCount
	return &notifications, nil
}

func (p *notificationRepo) Update(ctx context.Context, req *entity.UpdateNotificationRequest) error {
	var reqID = notificationIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("notificationRepo.Update - %s", reqID))
	}

	if req == nil {
		return fmt.Errorf("notification update request cannot be nil")
	}
	if req.ID == 0 {
		return fmt.Errorf("notification ID is required for update")
	}

	clauses := map[string]interface{}{"updated_at": time.Now().UTC()}

	if req.Message != nil {
		clauses["message"] = *req.Message
	}
	if req.Type != nil {
		clauses["type"] = *req.Type
	}
	if req.Read != nil {
		clauses["read"] = *req.Read
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
		return fmt.Errorf("no notification found with ID %d", req.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (p *notificationRepo) Delete(ctx context.Context, id int64) error {
	var reqID = notificationIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("notificationRepo.Delete - %s", reqID))
	}

	if id == 0 {
		return fmt.Errorf("notification ID is required for deletion")
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
		return fmt.Errorf("no notification found with ID %d", id)
	}

	return nil
}
