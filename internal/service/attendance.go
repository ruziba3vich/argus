package service

import (
	"context"
	"database/sql" // For sql.NullTime
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
	attendanceTableName = "attendances"
	attendanceIDKey     = "attendanceID: "
)

// AttendanceRepoInterface defines the interface for Attendance CRUD operations.
type AttendanceRepoInterface interface {
	Create(ctx context.Context, req *entity.CreateAttendanceRequest) (*entity.Attendance, error)
	Get(ctx context.Context, params map[string]string) (*entity.Attendance, error)
	List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllAttendancesResponse, error)
	Update(ctx context.Context, req *entity.UpdateAttendanceRequest) error
	Delete(ctx context.Context, id int64) error
}

type attendanceRepo struct {
	tableName string
	db        *postgres.Postgres
	log       logger.Logger
}

func NewAttendanceRepo(db *postgres.Postgres, log logger.Logger) AttendanceRepoInterface {
	return &attendanceRepo{
		tableName: attendanceTableName,
		db:        db,
		log:       log,
	}
}

func (p *attendanceRepo) Create(ctx context.Context, req *entity.CreateAttendanceRequest) (*entity.Attendance, error) {
	var reqID = attendanceIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("attendanceRepo.Create - %s", reqID))
	}

	// Define columns and values for insertion.
	columns := []string{
		"user_id", "date", "intime", "status",
		"created_at", "updated_at",
	}
	values := []interface{}{
		req.UserID, req.Date, req.InTime, req.Status,
		time.Now().UTC(), time.Now().UTC(),
	}

	// Conditionally add nullable fields
	if req.OutTime != nil {
		columns = append(columns, "outtime") // Database column name
		values = append(values, *req.OutTime)
	}

	sqlStr, args, err := p.db.Sq.Builder.
		Insert(p.tableName).
		Columns(columns...).
		Values(values...).
		Suffix(`RETURNING id, user_id, date, intime, outtime, status, created_at, updated_at`).
		ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" create")
	}

	var createdAttendance entity.Attendance
	var nullOutTime sql.NullTime // For nullable outTime

	tx, err := p.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("transaction start failed: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, sqlStr, args...)
	err = row.Scan(
		&createdAttendance.ID,
		&createdAttendance.UserID,
		&createdAttendance.Date,
		&createdAttendance.InTime,
		&nullOutTime, // Scan into NullTime
		&createdAttendance.Status,
		&createdAttendance.CreatedAt,
		&createdAttendance.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	if nullOutTime.Valid {
		createdAttendance.OutTime = &nullOutTime.Time
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return &createdAttendance, nil
}

func (p *attendanceRepo) Get(ctx context.Context, params map[string]string) (*entity.Attendance, error) {
	var reqID = attendanceIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("attendanceRepo.Get - %s", reqID))
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("at least one filter parameter is required")
	}

	builder := p.db.Sq.Builder.Select(
		"id", "user_id", "date", "intime", "outtime", "status",
		"created_at", "updated_at",
	).From(p.tableName)

	for key, value := range params {
		builder = builder.Where(squirrel.Eq{key: value})
	}

	sqlStr, args, err := builder.ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" get")
	}

	var attendance entity.Attendance
	var nullOutTime sql.NullTime

	err = p.db.QueryRow(ctx, sqlStr, args...).Scan(
		&attendance.ID,
		&attendance.UserID,
		&attendance.Date,
		&attendance.InTime,
		&nullOutTime,
		&attendance.Status,
		&attendance.CreatedAt,
		&attendance.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	if nullOutTime.Valid {
		attendance.OutTime = &nullOutTime.Time
	}

	return &attendance, nil
}

func (p *attendanceRepo) List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllAttendancesResponse, error) {
	var reqID = attendanceIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("attendanceRepo.List - %s", reqID))
	}

	var attendances entity.GetAllAttendancesResponse
	baseBuilder := p.db.Sq.Builder.Select(
		"id", "user_id", "date", "intime", "outtime", "status",
		"created_at", "updated_at",
	).From(p.tableName)

	countBuilder := p.db.Sq.Builder.Select("COUNT(*)").From(p.tableName)

	// Apply filters
	for key, value := range filter {
		baseBuilder = baseBuilder.Where(squirrel.Eq{key: value})
		countBuilder = countBuilder.Where(squirrel.Eq{key: value})
	}

	// Apply search (e.g., by status or user_id)
	if search != "" {
		searchClause := squirrel.Or{
			squirrel.ILike{"status": fmt.Sprintf("%%%s%%", search)}, // Assuming enum can be searched as string
			// You might want to convert user_id to text for search, or use specific filters
			// squirrel.Eq{"user_id": search} if search is an exact ID
		}
		baseBuilder = baseBuilder.Where(searchClause)
		countBuilder = countBuilder.Where(searchClause)
	}

	// Order, Limit, Offset for data query
	baseBuilder = baseBuilder.OrderBy("date DESC, intime DESC").Limit(limit).Offset(offset) // Order by date then intime

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
		var attendance entity.Attendance
		var nullOutTime sql.NullTime
		if err := rows.Scan(
			&attendance.ID, &attendance.UserID, &attendance.Date, &attendance.InTime,
			&nullOutTime, &attendance.Status, &attendance.CreatedAt, &attendance.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		if nullOutTime.Valid {
			attendance.OutTime = &nullOutTime.Time
		}
		attendances.Items = append(attendances.Items, attendance)
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

	attendances.Total = totalCount
	return &attendances, nil
}

func (p *attendanceRepo) Update(ctx context.Context, req *entity.UpdateAttendanceRequest) error {
	var reqID = attendanceIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("attendanceRepo.Update - %s", reqID))
	}

	if req == nil {
		return fmt.Errorf("attendance update request cannot be nil")
	}
	if req.ID == 0 {
		return fmt.Errorf("attendance ID is required for update")
	}

	clauses := map[string]interface{}{"updated_at": time.Now().UTC()}

	if req.UserID != nil {
		clauses["user_id"] = *req.UserID
	}
	if req.Date != nil {
		clauses["date"] = *req.Date
	}
	if req.InTime != nil {
		clauses["intime"] = *req.InTime
	}
	// Special handling for OutTime: allows setting to a value or to NULL
	if req.OutTime != nil {
		clauses["outtime"] = *req.OutTime
	} else {
		// If OutTime is explicitly nil in the request, it means set to NULL
		// This assumes that a nil pointer *intentionally* means NULL in the DB
		// If *not* provided, the pointer should be checked for nil, and not added to clauses
		// If it's provided as nil (e.g. OutTime: nil in JSON), it will pass here.
		// If you want to distinguish 'not provided' vs 'set to null', need a separate bool flag in request.
		// For simplicity now, if req.OutTime is nil, we set DB to NULL.
		clauses["outtime"] = nil
	}
	if req.Status != nil {
		clauses["status"] = *req.Status
	}

	if len(clauses) < 2 && (req.OutTime == nil && clauses["outtime"] == nil) { // Check if only updated_at or if only outtime nulling happened
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
		return fmt.Errorf("no attendance record found with ID %d", req.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (p *attendanceRepo) Delete(ctx context.Context, id int64) error {
	var reqID = attendanceIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("attendanceRepo.Delete - %s", reqID))
	}

	if id == 0 {
		return fmt.Errorf("attendance ID is required for deletion")
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
		return fmt.Errorf("no attendance record found with ID %d", id)
	}

	return nil
}
