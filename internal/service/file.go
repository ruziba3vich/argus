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
	fileTableName = "files"
	fileIDKey     = "fileID: "
)

// FileRepoInterface defines the interface for File CRUD operations.
type FileRepoInterface interface {
	Create(ctx context.Context, req *entity.CreateFileRequest) (*entity.File, error)
	Get(ctx context.Context, params map[string]string) (*entity.File, error)
	List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllFilesResponse, error)
	Update(ctx context.Context, req *entity.UpdateFileRequest) error
	Delete(ctx context.Context, id int64) error
}

type fileRepo struct {
	tableName string
	db        *postgres.Postgres
	log       logger.Logger
}

func NewFileRepo(db *postgres.Postgres, log logger.Logger) FileRepoInterface {
	return &fileRepo{
		tableName: fileTableName,
		db:        db,
		log:       log,
	}
}

func (p *fileRepo) Create(ctx context.Context, req *entity.CreateFileRequest) (*entity.File, error) {
	var reqID = fileIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("fileRepo.Create - %s", reqID))
	}

	// Define columns and values for insertion.
	columns := []string{
		"name",
		"created_at", "updated_at",
	}
	values := []interface{}{
		req.Name,
		time.Now().UTC(), time.Now().UTC(),
	}

	// Conditionally add nullable fields
	if req.TaskID != nil {
		columns = append(columns, "taskid") // Database column name
		values = append(values, *req.TaskID)
	}

	sqlStr, args, err := p.db.Sq.Builder.
		Insert(p.tableName).
		Columns(columns...).
		Values(values...).
		Suffix(`RETURNING id, name, taskid, created_at, updated_at`).
		ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" create")
	}

	var createdFile entity.File
	var nullTaskID sql.NullInt64 // For nullable taskid

	tx, err := p.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("transaction start failed: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, sqlStr, args...)
	err = row.Scan(
		&createdFile.ID,
		&createdFile.Name,
		&nullTaskID, // Scan into NullInt64
		&createdFile.CreatedAt,
		&createdFile.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	if nullTaskID.Valid {
		createdFile.TaskID = &nullTaskID.Int64
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return &createdFile, nil
}

func (p *fileRepo) Get(ctx context.Context, params map[string]string) (*entity.File, error) {
	var reqID = fileIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("fileRepo.Get - %s", reqID))
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("at least one filter parameter is required")
	}

	builder := p.db.Sq.Builder.Select(
		"id", "name", "taskid", "created_at", "updated_at",
	).From(p.tableName)

	for key, value := range params {
		builder = builder.Where(squirrel.Eq{key: value})
	}

	sqlStr, args, err := builder.ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" get")
	}

	var file entity.File
	var nullTaskID sql.NullInt64

	err = p.db.QueryRow(ctx, sqlStr, args...).Scan(
		&file.ID,
		&file.Name,
		&nullTaskID,
		&file.CreatedAt,
		&file.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	if nullTaskID.Valid {
		file.TaskID = &nullTaskID.Int64
	}

	return &file, nil
}

func (p *fileRepo) List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllFilesResponse, error) {
	var reqID = fileIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("fileRepo.List - %s", reqID))
	}

	var files entity.GetAllFilesResponse
	baseBuilder := p.db.Sq.Builder.Select(
		"id", "name", "taskid", "created_at", "updated_at",
	).From(p.tableName)

	countBuilder := p.db.Sq.Builder.Select("COUNT(*)").From(p.tableName)

	// Apply filters
	for key, value := range filter {
		baseBuilder = baseBuilder.Where(squirrel.Eq{key: value})
		countBuilder = countBuilder.Where(squirrel.Eq{key: value})
	}

	// Apply search (e.g., on file name)
	if search != "" {
		searchClause := squirrel.Or{
			squirrel.ILike{"name": fmt.Sprintf("%%%s%%", search)},
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
		var file entity.File
		var nullTaskID sql.NullInt64
		if err := rows.Scan(
			&file.ID, &file.Name, &nullTaskID, &file.CreatedAt, &file.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		if nullTaskID.Valid {
			file.TaskID = &nullTaskID.Int64
		}
		files.Items = append(files.Items, file)
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

	files.Total = totalCount
	return &files, nil
}

func (p *fileRepo) Update(ctx context.Context, req *entity.UpdateFileRequest) error {
	var reqID = fileIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("fileRepo.Update - %s", reqID))
	}

	if req == nil {
		return fmt.Errorf("file update request cannot be nil")
	}
	if req.ID == 0 {
		return fmt.Errorf("file ID is required for update")
	}

	clauses := map[string]interface{}{"updated_at": time.Now().UTC()}

	if req.Name != nil {
		clauses["name"] = *req.Name
	}
	if req.TaskID != nil {
		clauses["taskid"] = *req.TaskID
	} else { // Handle case where TaskID should be set to NULL
		// You might want a specific way to signal setting a nullable field to NULL
		// e.g., by having a bool flag, or checking if the pointer itself is nil
		// For simplicity, if the pointer is non-nil and its value is 0 or -1, consider setting to NULL
		// A more robust way is to use a dedicated 'SetToNull' field in Update request
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
		return fmt.Errorf("no file found with ID %d", req.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (p *fileRepo) Delete(ctx context.Context, id int64) error {
	var reqID = fileIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("fileRepo.Delete - %s", reqID))
	}

	if id == 0 {
		return fmt.Errorf("file ID is required for deletion")
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
		return fmt.Errorf("no file found with ID %d", id)
	}

	return nil
}
