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
	taskTableName = "tasks"
	taskIDKey     = "taskID: "
)

// TaskRepoInterface defines the interface for Task CRUD operations.
type TaskRepoInterface interface {
	Create(ctx context.Context, req *entity.CreateTaskRequest) (*entity.Task, error)
	Get(ctx context.Context, params map[string]string) (*entity.Task, error)
	List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllTasksResponse, error)
	Update(ctx context.Context, req *entity.UpdateTaskRequest) error
	Delete(ctx context.Context, id int64) error
}

type taskRepo struct {
	tableName string
	db        *postgres.Postgres
	log       logger.Logger
}

func NewTaskRepo(db *postgres.Postgres, log logger.Logger) TaskRepoInterface {
	return &taskRepo{
		tableName: taskTableName,
		db:        db,
		log:       log,
	}
}

func (p *taskRepo) Create(ctx context.Context, req *entity.CreateTaskRequest) (*entity.Task, error) {
	var reqID = taskIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("taskRepo.Create - %s", reqID))
	}

	// Define columns and values for insertion.
	columns := []string{
		"admin_id", "title", "status", "priority",
		"created_at", "updated_at",
	}
	values := []interface{}{
		req.AdminID, req.Title, req.Status, req.Priority,
		time.Now().UTC(), time.Now().UTC(),
	}

	// Conditionally add nullable fields
	if req.AssignedTo != nil {
		columns = append(columns, "assignedto") // Database column name
		values = append(values, *req.AssignedTo)
	}
	if req.Description != nil {
		columns = append(columns, "description")
		values = append(values, *req.Description)
	}
	if req.DueDate != nil {
		columns = append(columns, "duedate") // Database column name
		values = append(values, *req.DueDate)
	}

	sqlStr, args, err := p.db.Sq.Builder.
		Insert(p.tableName).
		Columns(columns...).
		Values(values...).
		Suffix(`RETURNING id, assignedto, admin_id, title, description, status, priority, duedate, created_at, updated_at`).
		ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" create")
	}

	var createdTask entity.Task
	// Use sql.Null* types for scanning nullable columns
	var (
		nullAssignedTo  sql.NullInt64
		nullDescription sql.NullString
		nullDueDate     sql.NullTime
	)

	tx, err := p.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("transaction start failed: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, sqlStr, args...)
	err = row.Scan(
		&createdTask.ID,
		&nullAssignedTo,
		&createdTask.AdminID,
		&createdTask.Title,
		&nullDescription,
		&createdTask.Status,
		&createdTask.Priority,
		&nullDueDate,
		&createdTask.CreatedAt,
		&createdTask.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	// Assign scanned null values to pointers in the entity struct
	if nullAssignedTo.Valid {
		createdTask.AssignedTo = &nullAssignedTo.Int64
	}
	if nullDescription.Valid {
		createdTask.Description = &nullDescription.String
	}
	if nullDueDate.Valid {
		createdTask.DueDate = &nullDueDate.Time
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return &createdTask, nil
}

func (p *taskRepo) Get(ctx context.Context, params map[string]string) (*entity.Task, error) {
	var reqID = taskIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("taskRepo.Get - %s", reqID))
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("at least one filter parameter is required")
	}

	builder := p.db.Sq.Builder.Select(
		"id", "assignedto", "admin_id", "title", "description", "status", "priority", "duedate",
		"created_at", "updated_at",
	).From(p.tableName)

	for key, value := range params {
		builder = builder.Where(squirrel.Eq{key: value})
	}

	sqlStr, args, err := builder.ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" get")
	}

	var task entity.Task
	var (
		nullAssignedTo  sql.NullInt64
		nullDescription sql.NullString
		nullDueDate     sql.NullTime
	)

	err = p.db.QueryRow(ctx, sqlStr, args...).Scan(
		&task.ID,
		&nullAssignedTo,
		&task.AdminID,
		&task.Title,
		&nullDescription,
		&task.Status,
		&task.Priority,
		&nullDueDate,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	if nullAssignedTo.Valid {
		task.AssignedTo = &nullAssignedTo.Int64
	}
	if nullDescription.Valid {
		task.Description = &nullDescription.String
	}
	if nullDueDate.Valid {
		task.DueDate = &nullDueDate.Time
	}

	return &task, nil
}

func (p *taskRepo) List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllTasksResponse, error) {
	var reqID = taskIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("taskRepo.List - %s", reqID))
	}

	var tasks entity.GetAllTasksResponse
	baseBuilder := p.db.Sq.Builder.Select(
		"id", "assignedto", "admin_id", "title", "description", "status", "priority", "duedate",
		"created_at", "updated_at",
	).From(p.tableName)

	countBuilder := p.db.Sq.Builder.Select("COUNT(*)").From(p.tableName)

	// Apply filters
	for key, value := range filter {
		baseBuilder = baseBuilder.Where(squirrel.Eq{key: value})
		countBuilder = countBuilder.Where(squirrel.Eq{key: value})
	}

	// Apply search (e.g., on title or description)
	if search != "" {
		searchClause := squirrel.Or{
			squirrel.ILike{"title": fmt.Sprintf("%%%s%%", search)},
			squirrel.ILike{"description": fmt.Sprintf("%%%s%%", search)},
			squirrel.ILike{"status": fmt.Sprintf("%%%s%%", search)},   // Assuming enum can be searched as string
			squirrel.ILike{"priority": fmt.Sprintf("%%%s%%", search)}, // Assuming enum can be searched as string
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
		var task entity.Task
		var (
			nullAssignedTo  sql.NullInt64
			nullDescription sql.NullString
			nullDueDate     sql.NullTime
		)
		if err := rows.Scan(
			&task.ID, &nullAssignedTo, &task.AdminID, &task.Title, &nullDescription,
			&task.Status, &task.Priority, &nullDueDate, &task.CreatedAt, &task.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		if nullAssignedTo.Valid {
			task.AssignedTo = &nullAssignedTo.Int64
		}
		if nullDescription.Valid {
			task.Description = &nullDescription.String
		}
		if nullDueDate.Valid {
			task.DueDate = &nullDueDate.Time
		}
		tasks.Items = append(tasks.Items, &task)
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

	tasks.Total = totalCount
	return &tasks, nil
}

func (p *taskRepo) Update(ctx context.Context, req *entity.UpdateTaskRequest) error {
	var reqID = taskIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("taskRepo.Update - %s", reqID))
	}

	if req == nil {
		return fmt.Errorf("task update request cannot be nil")
	}
	if req.ID == 0 {
		return fmt.Errorf("task ID is required for update")
	}

	clauses := map[string]interface{}{"updated_at": time.Now().UTC()}

	if req.AssignedTo != nil {
		clauses["assignedto"] = *req.AssignedTo
	}
	if req.AdminID != nil {
		clauses["admin_id"] = *req.AdminID
	}
	if req.Title != nil {
		clauses["title"] = *req.Title
	}
	if req.Description != nil {
		clauses["description"] = *req.Description
	}
	if req.Status != nil {
		clauses["status"] = *req.Status
	}
	if req.Priority != nil {
		clauses["priority"] = *req.Priority
	}
	if req.DueDate != nil {
		clauses["duedate"] = *req.DueDate
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
		return fmt.Errorf("no task found with ID %d", req.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (p *taskRepo) Delete(ctx context.Context, id int64) error {
	var reqID = taskIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("taskRepo.Delete - %s", reqID))
	}

	if id == 0 {
		return fmt.Errorf("task ID is required for deletion")
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
		return fmt.Errorf("no task found with ID %d", id)
	}

	return nil
}
