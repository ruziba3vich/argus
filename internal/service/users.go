package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/ruziba3vich/argus/internal/entity"
	"github.com/ruziba3vich/argus/internal/pkg/helper"
	"github.com/ruziba3vich/argus/internal/postgres"
	"golang.org/x/crypto/bcrypt"

	logger "github.com/ruziba3vich/prodonik_lgger"
)

const (
	userTableName = "users"
	userIDKey     = "userID: "
)

// UserRepoInterface defines the interface for User CRUD operations.
type UserRepoInterface interface {
	Create(ctx context.Context, user *entity.CreateUserRequest) (*entity.User, error)
	Get(ctx context.Context, params map[string]string) (*entity.User, error)
	List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllUsersResponse, error)
	Update(ctx context.Context, user *entity.UpdateUserRequest) error
	Delete(ctx context.Context, id int64) error
}

type userRepo struct {
	tableName string
	db        *postgres.Postgres
	log       logger.Logger
}

func NewUserRepo(db *postgres.Postgres, log logger.Logger) UserRepoInterface {
	return &userRepo{
		tableName: userTableName,
		db:        db,
		log:       log,
	}
}

func (p *userRepo) Create(ctx context.Context, req *entity.CreateUserRequest) (*entity.User, error) {
	var reqID = userIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("userRepo.Create - %s", reqID))
	}
	/*  TODO
	if !helper.ValidatePhoneNumber(req.Phone) {
		return nil, errors.New("invalid phone number format")
	}
	*/

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Prepare data for insertion
	columns := []string{
		"first_name", "last_name", "role", "email", "phone",
		"hashed_password", "created_at", "updated_at",
	}
	values := []interface{}{
		req.FirstName, req.LastName, req.Role, req.Email, req.Phone,
		hashedPassword, time.Now().UTC(), time.Now().UTC(),
	}

	if req.PhotoURL != nil {
		columns = append(columns, "photo_url")
		values = append(values, *req.PhotoURL)
	}
	if req.Bio != nil {
		columns = append(columns, "bio")
		values = append(values, *req.Bio)
	}

	sqlStr, args, err := p.db.Sq.Builder.
		Insert(p.tableName).
		Columns(columns...).
		Values(values...).
		Suffix(`RETURNING id, first_name, last_name, role, email, phone, photo_url, bio, hashed_password, hashed_refresh_token, created_at, updated_at`).
		ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" create")
	}

	var createdUser entity.User
	var (
		nullPhotoURL           sql.NullString
		nullBio                sql.NullString
		nullHashedRefreshToken sql.NullString
	)

	tx, err := p.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("transaction start failed: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, sqlStr, args...)
	err = row.Scan(
		&createdUser.ID, &createdUser.FirstName, &createdUser.LastName, &createdUser.Role,
		&createdUser.Email, &createdUser.Phone, &nullPhotoURL, &nullBio,
		&createdUser.HashedPassword, &nullHashedRefreshToken, &createdUser.CreatedAt, &createdUser.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	if nullPhotoURL.Valid {
		createdUser.PhotoURL = &nullPhotoURL.String
	}
	if nullBio.Valid {
		createdUser.Bio = &nullBio.String
	}
	if nullHashedRefreshToken.Valid {
		createdUser.HashedRefreshToken = &nullHashedRefreshToken.String
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	return &createdUser, nil
}

func (p *userRepo) Get(ctx context.Context, params map[string]string) (*entity.User, error) {
	var reqID = userIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("userRepo.Get - %s", reqID))
	}

	if len(params) == 0 {
		return nil, errors.New("at least one filter parameter is required")
	}

	builder := p.db.Sq.Builder.Select(
		"id", "first_name", "last_name", "role", "email", "phone",
		"photo_url", "bio", "hashed_password", "hashed_refresh_token", "created_at", "updated_at",
	).From(p.tableName)

	for key, value := range params {
		builder = builder.Where(squirrel.Eq{key: value})
	}

	sqlStr, args, err := builder.ToSql()
	if err != nil {
		return nil, p.db.ErrSQLBuild(err, p.tableName+" get")
	}

	var user entity.User
	var (
		nullPhotoURL           sql.NullString
		nullBio                sql.NullString
		nullHashedRefreshToken sql.NullString
	)

	err = p.db.QueryRow(ctx, sqlStr, args...).Scan(
		&user.ID, &user.FirstName, &user.LastName, &user.Role,
		&user.Email, &user.Phone, &nullPhotoURL, &nullBio,
		&user.HashedPassword, &nullHashedRefreshToken, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, p.db.Error(err)
	}

	if nullPhotoURL.Valid {
		user.PhotoURL = &nullPhotoURL.String
	}
	if nullBio.Valid {
		user.Bio = &nullBio.String
	}
	if nullHashedRefreshToken.Valid {
		user.HashedRefreshToken = &nullHashedRefreshToken.String
	}

	return &user, nil
}

func (p *userRepo) List(ctx context.Context, limit, offset uint64, filter map[string]string, search string) (*entity.GetAllUsersResponse, error) {
	var reqID = userIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("userRepo.List - %s", reqID))
	}

	var users entity.GetAllUsersResponse
	baseBuilder := p.db.Sq.Builder.Select(
		"id", "first_name", "last_name", "role", "email", "phone",
		"photo_url", "bio", "created_at", "updated_at", // Note: Hashed password/token not selected for list
	).From(p.tableName)

	countBuilder := p.db.Sq.Builder.Select("COUNT(*)").From(p.tableName)

	// Apply filters
	for key, value := range filter {
		baseBuilder = baseBuilder.Where(squirrel.Eq{key: value})
		countBuilder = countBuilder.Where(squirrel.Eq{key: value})
	}

	// Apply search
	if search != "" {
		searchClause := squirrel.Or{
			squirrel.ILike{"first_name": fmt.Sprintf("%%%s%%", search)},
			squirrel.ILike{"last_name": fmt.Sprintf("%%%s%%", search)},
			squirrel.ILike{"email": fmt.Sprintf("%%%s%%", search)},
			squirrel.ILike{"phone": fmt.Sprintf("%%%s%%", search)},
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
		var user entity.User
		var (
			nullPhotoURL sql.NullString
			nullBio      sql.NullString
		)

		if err := rows.Scan(
			&user.ID, &user.FirstName, &user.LastName, &user.Role,
			&user.Email, &user.Phone, &nullPhotoURL, &nullBio,
			&user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		if nullPhotoURL.Valid {
			user.PhotoURL = &nullPhotoURL.String
		}
		if nullBio.Valid {
			user.Bio = &nullBio.String
		}
		users.Items = append(users.Items, &user)
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

	users.Total = totalCount
	return &users, nil
}

func (p *userRepo) Update(ctx context.Context, req *entity.UpdateUserRequest) error {
	var reqID = userIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("userRepo.Update - %s", reqID))
	}

	if req == nil {
		return fmt.Errorf("user update request cannot be nil")
	}
	if req.ID == 0 { // Assuming ID is bigint, so 0 is invalid
		return fmt.Errorf("user ID is required for update")
	}

	clauses := map[string]interface{}{"updated_at": time.Now().UTC()}

	if req.FirstName != nil {
		clauses["first_name"] = *req.FirstName
	}
	if req.LastName != nil {
		clauses["last_name"] = *req.LastName
	}
	if req.Role != nil {
		clauses["role"] = *req.Role
	}
	if req.Email != nil {
		clauses["email"] = *req.Email
	}
	if req.Phone != nil {
		if !helper.ValidatePhoneNumber(*req.Phone) { // Assuming helper.ValidatePhoneNumber exists
			return fmt.Errorf("invalid phone number format")
		}
		clauses["phone"] = *req.Phone
	}
	if req.PhotoURL != nil {
		clauses["photo_url"] = *req.PhotoURL
	}
	if req.Bio != nil {
		clauses["bio"] = *req.Bio
	}
	if req.HashedPassword != nil {
		clauses["hashed_password"] = *req.HashedPassword
	}
	if req.HashedRefreshToken != nil {
		clauses["hashed_refresh_token"] = *req.HashedRefreshToken
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
		return fmt.Errorf("no user found with ID %d", req.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (p *userRepo) Delete(ctx context.Context, id int64) error {
	var reqID = userIDKey
	if rID := ctx.Value(middleware.RequestIDKey); rID != nil {
		reqID += rID.(string)
		p.log.Info(fmt.Sprintf("userRepo.Delete - %s", reqID))
	}

	if id == 0 {
		return fmt.Errorf("user ID is required for deletion")
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
		return fmt.Errorf("no user found with ID %d", id)
	}

	return nil
}
