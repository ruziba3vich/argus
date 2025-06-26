package entity

import (
	"time"
)

// Common fields for created_at and updated_at
type BaseModel struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Enums (assuming these are string-based enums in your DB)
type NotificationType string

const (
	NotificationTypeEmail NotificationType = "email"
	NotificationTypeSMS   NotificationType = "sms"
	NotificationTypeApp   NotificationType = "app"
)

type UserRole string

const (
	UserRoleAdmin      UserRole = "admin"
	UserRoleUser       UserRole = "user"
	UserRoleSuperAdmin UserRole = "super_admin"
)

type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencyGBP Currency = "GBP"
)

type AttendanceStatus string

const (
	AttendanceStatusPresent AttendanceStatus = "present"
	AttendanceStatusAbsent  AttendanceStatus = "absent"
	AttendanceStatusLate    AttendanceStatus = "late"
)

type SalaryStatus string

const (
	SalaryStatusPaid    SalaryStatus = "paid"
	SalaryStatusPending SalaryStatus = "pending"
	SalaryStatusOverdue SalaryStatus = "overdue"
)

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityHigh   TaskPriority = "high"
)

// Bonuses
type Bonus struct {
	ID           int64    `json:"id"`
	SuperAdminID *int64   `json:"super_admin_id"` // Assuming nullable
	UserID       int64    `json:"user_id"`
	Amount       float64  `json:"amount"`
	Currency     Currency `json:"currency"` // Enum
	Reason       *string  `json:"reason"`   // Assuming nullable
	BaseModel
}

type CreateBonusRequest struct {
	SuperAdminID *int64   `json:"super_admin_id"`
	UserID       int64    `json:"user_id"`
	Amount       float64  `json:"amount"`
	Currency     Currency `json:"currency"`
	Reason       *string  `json:"reason"`
}

type UpdateBonusRequest struct {
	ID           int64     `json:"id"`
	SuperAdminID *int64    `json:"super_admin_id"`
	UserID       *int64    `json:"user_id"` // Optional update
	Amount       *float64  `json:"amount"`
	Currency     *Currency `json:"currency"`
	Reason       *string   `json:"reason"`
}

type GetAllBonusesResponse struct {
	Items []Bonus `json:"items"`
	Total uint64  `json:"total"`
}

// Notifications
type Notification struct {
	ID      int64            `json:"id"`
	UserID  int64            `json:"user_id"`
	Message string           `json:"message"`
	Type    NotificationType `json:"type"` // Enum
	Read    bool             `json:"read"`
	BaseModel
}

type CreateNotificationRequest struct {
	UserID  int64            `json:"user_id"`
	Message string           `json:"message"`
	Type    NotificationType `json:"type"`
	Read    bool             `json:"read"`
}

type UpdateNotificationRequest struct {
	ID      int64             `json:"id"`
	Message *string           `json:"message"`
	Type    *NotificationType `json:"type"`
	Read    *bool             `json:"read"`
}

type GetAllNotificationsResponse struct {
	Items []Notification `json:"items"`
	Total uint64         `json:"total"`
}

// Users
type User struct {
	ID                 int64    `json:"id"`
	FirstName          string   `json:"first_name"`
	LastName           string   `json:"last_name"`
	Role               UserRole `json:"role"` // Enum
	Email              string   `json:"email"`
	Phone              string   `json:"phone"`
	PhotoURL           *string  `json:"photo_url"` // Assuming nullable
	Bio                *string  `json:"bio"`       // Assuming nullable
	HashedPassword     string   `json:"-"`         // Never expose in JSON
	HashedRefreshToken *string  `json:"-"`         // Never expose in JSON, assuming nullable
	BaseModel
}

type CreateUserRequest struct {
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Role      UserRole `json:"role"`
	Email     string   `json:"email"`
	Phone     string   `json:"phone"`
	PhotoURL  *string  `json:"photo_url"`
	Bio       *string  `json:"bio"`
	Password  string   `json:"password"` // For creation, will be hashed
}

type UpdateUserRequest struct {
	ID                 int64     `json:"id"`
	FirstName          *string   `json:"first_name"`
	LastName           *string   `json:"last_name"`
	Role               *UserRole `json:"role"`
	Email              *string   `json:"email"`
	Phone              *string   `json:"phone"`
	PhotoURL           *string   `json:"photo_url"`
	Bio                *string   `json:"bio"`
	HashedPassword     *string   `json:"-"`
	HashedRefreshToken *string   `json:"-"`
}

type GetAllUsersResponse struct {
	Items []User `json:"items"`
	Total uint64 `json:"total"`
}

// Salaries
type Salary struct {
	ID             int64        `json:"id"`
	Amount         float64      `json:"amount"`
	UserID         int64        `json:"user_id"`
	AdminID        int64        `json:"admin_id"`
	UpdaterAdminID *int64       `json:"updater_admin_id"` // Assuming nullable
	PayDate        time.Time    `json:"pay_date"`
	Currency       Currency     `json:"currency"` // Enum
	Status         SalaryStatus `json:"status"`   // Enum
	BaseModel
}

type CreateSalaryRequest struct {
	Amount         float64      `json:"amount"`
	UserID         int64        `json:"user_id"`
	AdminID        int64        `json:"admin_id"`
	UpdaterAdminID *int64       `json:"updater_admin_id"`
	PayDate        time.Time    `json:"pay_date"`
	Currency       Currency     `json:"currency"`
	Status         SalaryStatus `json:"status"`
}

type UpdateSalaryRequest struct {
	ID             int64         `json:"id"`
	Amount         *float64      `json:"amount"`
	UserID         *int64        `json:"user_id"`
	AdminID        *int64        `json:"admin_id"`
	UpdaterAdminID *int64        `json:"updater_admin_id"`
	PayDate        *time.Time    `json:"pay_date"`
	Currency       *Currency     `json:"currency"`
	Status         *SalaryStatus `json:"status"`
}

type GetAllSalariesResponse struct {
	Items []Salary `json:"items"`
	Total uint64   `json:"total"`
}

// Tasks
type Task struct {
	ID          int64        `json:"id"`
	AssignedTo  *int64       `json:"assigned_to"` // Assuming nullable, refers to UserID
	AdminID     int64        `json:"admin_id"`    // Refers to UserID
	Title       string       `json:"title"`
	Description *string      `json:"description"` // Assuming nullable
	Status      TaskStatus   `json:"status"`      // Enum
	Priority    TaskPriority `json:"priority"`    // Enum
	DueDate     *time.Time   `json:"due_date"`    // Assuming nullable
	BaseModel
}

type CreateTaskRequest struct {
	AssignedTo  *int64       `json:"assigned_to"`
	AdminID     int64        `json:"admin_id"`
	Title       string       `json:"title"`
	Description *string      `json:"description"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`
	DueDate     *time.Time   `json:"due_date"`
}

type UpdateTaskRequest struct {
	ID          int64         `json:"id"`
	AssignedTo  *int64        `json:"assigned_to"`
	AdminID     *int64        `json:"admin_id"`
	Title       *string       `json:"title"`
	Description *string       `json:"description"`
	Status      *TaskStatus   `json:"status"`
	Priority    *TaskPriority `json:"priority"`
	DueDate     *time.Time    `json:"due_date"`
}

type GetAllTasksResponse struct {
	Items []Task `json:"items"`
	Total uint64 `json:"total"`
}

// Files
type File struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	TaskID *int64 `json:"task_id"`
	BaseModel
}

type CreateFileRequest struct {
	Name   string `json:"name"`
	TaskID *int64 `json:"task_id"`
}

type UpdateFileRequest struct {
	ID     int64   `json:"id"`
	Name   *string `json:"name"`
	TaskID *int64  `json:"task_id"`
}

type GetAllFilesResponse struct {
	Items []File `json:"items"`
	Total uint64 `json:"total"`
}
