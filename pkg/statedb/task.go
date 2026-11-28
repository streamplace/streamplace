package statedb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"stream.place/streamplace/pkg/stationdb"
)

// TaskStatus represents the status of a task in the queue
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "PENDING"
	TaskStatusProcessing TaskStatus = "PROCESSING"
	TaskStatusCompleted  TaskStatus = "COMPLETED"
	TaskStatusFailed     TaskStatus = "FAILED"
	TaskStatusRetrying   TaskStatus = "RETRYING"
	TaskStatusBlocked    TaskStatus = "BLOCKED"
)

const taskLeaseDuration = 30 * time.Minute

var (
	ErrTaskClaimLost       = errors.New("task claim lost")
	ErrTaskEffectUncertain = errors.New("task has an unconfirmed external effect; do not replay without reconciliation")
)

// AppTask represents a task in the queue
type AppTask struct {
	ID          uint            `gorm:"column:id;primarykey"`
	Type        string          `gorm:"column:type;not null;index"`
	TaskKey     *string         `gorm:"column:task_key;index:idx_task_dedup,unique"`
	Status      TaskStatus      `gorm:"column:status;not null;index;default:'PENDING'"`
	Payload     json.RawMessage `gorm:"column:payload;type:jsonb"`
	Priority    int             `gorm:"column:priority;default:0;index"`
	TryCount    int             `gorm:"column:try_count;default:0"`
	MaxTries    int             `gorm:"column:max_tries;default:3"`
	LockExpires *time.Time      `gorm:"column:lock_expires"`
	WorkerID    *string         `gorm:"column:worker_id"`
	ClaimToken  string          `gorm:"column:claim_token" json:"-"`
	Error       *string         `gorm:"column:error"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at"`
	ScheduledAt *time.Time      `gorm:"column:scheduled_at"` // for delayed tasks
}

// AppTaskEffect journals one external write. A NULL receipt requires reconciliation.
type AppTaskEffect struct {
	TaskID    uint   `gorm:"primaryKey;autoIncrement:false;not null;index:idx_app_task_effects_pending,where:receipt IS NULL"`
	EffectKey string `gorm:"primaryKey;not null"`
	Target    string // public AT URI only; never credentials or delivery endpoints
	Receipt   []byte
}

// EnqueueTask adds a new task to the queue
func (state *StatefulDB) EnqueueTask(ctx context.Context, taskType string, payload any, options ...TaskOption) (*AppTask, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := &AppTask{
		Type:     taskType,
		Status:   TaskStatusPending,
		Payload:  payloadBytes,
		Priority: 0,
		MaxTries: 3,
	}

	// Apply options
	for _, opt := range options {
		opt(task)
	}

	db := state.DB.WithContext(ctx)
	if task.TaskKey != nil {
		db = db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "task_key"}},
			DoNothing: true,
		})
	}
	result := db.Create(task)
	if task.TaskKey != nil && (result.Error == nil && result.RowsAffected == 0 ||
		errors.Is(result.Error, stationdb.ErrConflict) && !errors.Is(result.Error, stationdb.ErrCommitUnknown)) {
		existing, err := state.GetTaskByKey(ctx, *task.TaskKey)
		if err != nil {
			return nil, errors.Join(result.Error, err)
		}
		if existing != nil {
			return existing, nil
		}
	}
	if result.Error != nil {
		return nil, fmt.Errorf("failed to enqueue task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, errors.New("enqueue task returned no durable row")
	}

	select {
	case state.pokeQueue <- struct{}{}:
	default:
	}

	return task, nil
}

// taskClock uses the database clock for lease comparisons between nodes.
func (state *StatefulDB) taskClock(column string) (value, now string) {
	if state.Type == DBTypePostgres {
		return column, "clock_timestamp()"
	}
	return "julianday(" + column + ")", "julianday('now')"
}

func (state *StatefulDB) taskLeaseExpiry() string {
	if state.Type == DBTypePostgres {
		return postgresTaskLeaseExpiry
	}
	return sqliteTaskLeaseExpiry
}

var (
	postgresTaskLeaseExpiry = fmt.Sprintf("clock_timestamp() + interval '%d seconds'", int(taskLeaseDuration.Seconds()))
	sqliteTaskLeaseExpiry   = fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:%%M:%%f', 'now', '+%d seconds')", int(taskLeaseDuration.Seconds()))
)

func (state *StatefulDB) expiredTaskLease() string {
	expiry, now := state.taskClock("app_tasks.lock_expires")
	return "(app_tasks.lock_expires IS NULL OR " + expiry + " <= " + now + ")"
}

// DequeueTask atomically claims and returns the next eligible task, including
// abandoned claims that have attempts left.
func (state *StatefulDB) DequeueTask(ctx context.Context, workerID string, taskTypes ...string) (*AppTask, error) {
	db := state.DB.WithContext(ctx)
	expired := state.expiredTaskLease()
	if _, err := state.cleanupTaskLocks(db, taskTypes, true); err != nil {
		return nil, err
	}

	token, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("generate task claim token: %w", err)
	}
	schedule, now := state.taskClock("scheduled_at")
	next := db.Model(&AppTask{}).Select("id").
		Where("status IN ? AND "+expired+" AND try_count < max_tries", []TaskStatus{TaskStatusPending, TaskStatusProcessing}).
		Where("(scheduled_at IS NULL OR " + schedule + " <= " + now + ")").
		Order("priority DESC, created_at ASC, id ASC").Limit(1)
	if len(taskTypes) != 0 {
		next = next.Where("type IN ?", taskTypes)
	}
	if state.Type == DBTypePostgres {
		next = next.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
	}
	var task AppTask
	result := db.Raw(`UPDATE app_tasks
		SET status = ?, worker_id = ?, claim_token = ?, lock_expires = `+state.taskLeaseExpiry()+`,
			try_count = try_count + 1, updated_at = ?
		WHERE id = (?) RETURNING *`, TaskStatusProcessing, workerID, token.String(), time.Now().UTC(), next).Scan(&task)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to dequeue task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &task, nil
}

func (state *StatefulDB) taskClaim(db *gorm.DB, id uint, token string) *gorm.DB {
	expiry, now := state.taskClock("app_tasks.lock_expires")
	return db.Model(&AppTask{}).Where("app_tasks.id = ? AND app_tasks.status = ? AND app_tasks.claim_token = ? AND app_tasks.claim_token <> '' AND "+expiry+" > "+now,
		id, TaskStatusProcessing, token)
}

func taskClaimResult(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskClaimLost
	}
	return nil
}

const (
	taskEffectsAny         = "EXISTS (SELECT 1 FROM app_task_effects WHERE app_task_effects.task_id = app_tasks.id)"
	taskEffectsOutstanding = "EXISTS (SELECT 1 FROM app_task_effects WHERE app_task_effects.task_id = app_tasks.id AND app_task_effects.receipt IS NULL)"
)

func (state *StatefulDB) lockTaskClaim(tx *gorm.DB, id uint, token string) error {
	claim := state.taskClaim(tx, id, token)
	if state.Type == DBTypePostgres {
		var lockedID uint
		return taskClaimScanError(claim.Select("app_tasks.id").
			Clauses(clause.Locking{Strength: "UPDATE"}).Row().Scan(&lockedID))
	}
	// SQLite must acquire its writer lock before reading any effect rows.
	return taskClaimResult(claim.UpdateColumn("claim_token", token))
}

func (state *StatefulDB) withTaskClaimTx(ctx context.Context, id uint, token string, write func(*gorm.DB) error) error {
	return state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := state.lockTaskClaim(tx, id, token); err != nil {
			return err
		}
		return write(tx)
	})
}

func endTaskClaim(dst TaskStatus, retry, blockOnUnknown bool, msg any) map[string]any {
	unknown := taskEffectsOutstanding
	var next any = dst
	if retry {
		next = gorm.Expr("CASE WHEN app_tasks.try_count < app_tasks.max_tries THEN ? ELSE ? END", TaskStatusPending, dst)
		if !blockOnUnknown {
			unknown = "(app_tasks.try_count >= app_tasks.max_tries AND " + unknown + ")"
		}
	}
	return map[string]any{
		"status":       gorm.Expr("CASE WHEN "+unknown+" THEN ? ELSE ? END", TaskStatusBlocked, next),
		"error":        gorm.Expr("CASE WHEN "+unknown+" THEN ? ELSE ? END", ErrTaskEffectUncertain.Error(), msg),
		"lock_expires": nil,
		"worker_id":    nil,
		"claim_token":  "",
	}
}

func (state *StatefulDB) finishTaskClaim(ctx context.Context, id uint, token string, updates map[string]any) (TaskStatus, error) {
	var task AppTask
	err := state.withTaskClaimTx(ctx, id, token, func(tx *gorm.DB) error {
		// Use a new statement snapshot after locking the task row on Postgres.
		return taskClaimResult(state.taskClaim(tx, id, token).Model(&task).
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "status"}}}).Updates(updates))
	})
	return task.Status, err
}

// CompleteTask acknowledges a claim. Unconfirmed effects make it BLOCKED, not COMPLETED.
func (state *StatefulDB) CompleteTask(ctx context.Context, id uint, token string) error {
	status, err := state.finishTaskClaim(ctx, id, token, endTaskClaim(TaskStatusCompleted, false, false, gorm.Expr(`"error"`)))
	if err != nil {
		return fmt.Errorf("complete task: %w", err)
	}
	if status == TaskStatusBlocked {
		return ErrTaskEffectUncertain
	}
	return nil
}

// FailTask records an error and retries the task if it has attempts left.
func (state *StatefulDB) FailTask(ctx context.Context, id uint, token, msg string) error {
	return state.failTask(ctx, id, token, msg, false)
}

func (state *StatefulDB) failTask(ctx context.Context, id uint, token, msg string, blockOnUnknown bool) error {
	_, err := state.finishTaskClaim(ctx, id, token, endTaskClaim(TaskStatusFailed, true, blockOnUnknown, msg))
	return err
}

// ReleaseTask gives up the claim without refunding the attempt.
func (state *StatefulDB) ReleaseTask(ctx context.Context, id uint, token string) error {
	msg := gorm.Expr(`CASE WHEN try_count < max_tries THEN "error" ELSE ? END`, "task claim released")
	_, err := state.finishTaskClaim(ctx, id, token, endTaskClaim(TaskStatusFailed, true, false, msg))
	return err
}

// RenewTask extends a current, unexpired claim.
func (state *StatefulDB) RenewTask(ctx context.Context, id uint, token string) error {
	db := state.DB.WithContext(ctx)
	if state.Type == DBTypePostgres {
		return db.Transaction(func(tx *gorm.DB) error { return state.renewTask(tx, id, token) })
	}
	return state.renewTask(db, id, token)
}

func (state *StatefulDB) renewTask(tx *gorm.DB, id uint, token string) error {
	// PostgreSQL can wait on a row lock past its lease; recheck after locking.
	if state.Type == DBTypePostgres {
		if err := state.lockTaskClaim(tx, id, token); err != nil {
			return err
		}
	}
	return taskClaimResult(state.taskClaim(tx, id, token).
		UpdateColumn("lock_expires", gorm.Expr(state.taskLeaseExpiry())))
}

// The processor needs only these atomic effect operations; a shared state
// backend can provide them without exposing SQL to dispatchers.
type taskEffectStore interface {
	beginTaskEffect(context.Context, uint, string, string, string) ([]byte, error)
	readTaskEffect(context.Context, uint, string, string) ([]byte, error)
	setTaskEffect(context.Context, uint, string, string, []byte) error
}

func (state *StatefulDB) beginTaskEffect(ctx context.Context, id uint, token, key, target string) ([]byte, error) {
	var receipt []byte
	err := state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := state.renewTask(tx, id, token); err != nil {
			return err
		}
		effect := AppTaskEffect{TaskID: id, EffectKey: key, Target: target}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&effect)
		if result.Error != nil || result.RowsAffected != 0 {
			return result.Error
		}
		var err error
		receipt, err = selectTaskEffect(state.taskClaim(tx, id, token), key)
		return err
	})
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func (state *StatefulDB) readTaskEffect(ctx context.Context, id uint, token, key string) ([]byte, error) {
	return selectTaskEffect(state.taskClaim(state.DB.WithContext(ctx), id, token), key)
}

func taskClaimScanError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTaskClaimLost
	}
	return err
}

func selectTaskEffect(claim *gorm.DB, key string) ([]byte, error) {
	const point = "FROM app_task_effects WHERE app_task_effects.task_id = app_tasks.id AND app_task_effects.effect_key = ?"
	var receipt []byte
	var outstanding bool
	err := claim.Select("(SELECT receipt "+point+" AND receipt IS NOT NULL), EXISTS (SELECT 1 "+point+" AND receipt IS NULL)", key, key).
		Row().Scan(&receipt, &outstanding)
	if err != nil {
		return nil, taskClaimScanError(err)
	}
	if outstanding {
		return nil, fmt.Errorf("%w: %s", ErrTaskEffectUncertain, key)
	}
	return receipt, nil
}

func (state *StatefulDB) hasTaskEffects(ctx context.Context, id uint, token string) (bool, error) {
	var exists bool
	err := state.taskClaim(state.DB.WithContext(ctx), id, token).Select(taskEffectsAny).Row().Scan(&exists)
	return exists, taskClaimScanError(err)
}

// A nil receipt clears a reservation only when its caller knows it never dispatched.
func (state *StatefulDB) setTaskEffect(ctx context.Context, id uint, token, key string, receipt []byte) error {
	return state.withTaskClaimTx(ctx, id, token, func(tx *gorm.DB) error {
		claim := state.taskClaim(tx, id, token).Select("1")
		pending := tx.Model(&AppTaskEffect{}).
			Where("app_task_effects.task_id = ? AND app_task_effects.effect_key = ? AND app_task_effects.receipt IS NULL", id, key).
			Where("EXISTS (?)", claim)
		if receipt == nil {
			return taskClaimResult(pending.Delete(&AppTaskEffect{}))
		}
		return taskClaimResult(pending.UpdateColumn("receipt", receipt))
	})
}

func taskEffectKey(kind, target string) string {
	return fmt.Sprintf("%s-%x", kind, sha256.Sum256([]byte(target)))
}

// GetTask retrieves a task by ID
func (state *StatefulDB) GetTask(ctx context.Context, taskID uint) (*AppTask, error) {
	var task AppTask
	if err := state.DB.WithContext(ctx).First(&task, taskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	return &task, nil
}

// GetTaskByKey retrieves a task by its unique task key
func (state *StatefulDB) GetTaskByKey(ctx context.Context, taskKey string) (*AppTask, error) {
	var task AppTask
	if err := state.DB.WithContext(ctx).Where("task_key = ?", taskKey).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get task by key: %w", err)
	}
	return &task, nil
}

// ListTasks retrieves tasks with optional filters
func (state *StatefulDB) ListTasks(ctx context.Context, filters TaskFilters) ([]AppTask, error) {
	var tasks []AppTask
	query := state.DB.WithContext(ctx).Model(&AppTask{})

	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.Type != "" {
		query = query.Where("type = ?", filters.Type)
	}
	if filters.TaskKey != "" {
		query = query.Where("task_key = ?", filters.TaskKey)
	}
	if filters.WorkerID != "" {
		query = query.Where("worker_id = ?", filters.WorkerID)
	}
	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	}
	if filters.Offset > 0 {
		query = query.Offset(filters.Offset)
	}

	query = query.Order("created_at DESC")

	if err := query.Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	return tasks, nil
}

// CleanupExpiredLocks releases a batch of expired claims, terminalizing exhausted attempts.
func (state *StatefulDB) CleanupExpiredLocks(ctx context.Context) (int64, error) {
	return state.cleanupTaskLocks(state.DB.WithContext(ctx), nil, false)
}

func (state *StatefulDB) cleanupTaskLocks(db *gorm.DB, taskTypes []string, exhaustedOnly bool) (int64, error) {
	const batch = 128
	var cleaned int64
	err := db.Transaction(func(tx *gorm.DB) error {
		pick := tx.Model(&AppTask{}).Select("app_tasks.id").Order("app_tasks.id").Limit(batch).
			Where("((app_tasks.status = ? AND "+state.expiredTaskLease()+") OR (app_tasks.status = ? AND app_tasks.try_count >= app_tasks.max_tries))",
				TaskStatusProcessing, TaskStatusPending)
		if exhaustedOnly {
			pick = pick.Where("app_tasks.try_count >= app_tasks.max_tries")
		}
		if len(taskTypes) > 0 {
			pick = pick.Where("app_tasks.type IN ?", taskTypes)
		}
		query := tx.Model(&AppTask{})
		if state.Type == DBTypePostgres {
			var ids []uint
			if err := pick.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Pluck("id", &ids).Error; err != nil {
				return err
			}
			if len(ids) == 0 {
				return nil
			}
			query = query.Where("app_tasks.id IN ?", ids)
		} else {
			query = query.Where("app_tasks.id IN (?)", pick)
		}
		msg := gorm.Expr(`CASE WHEN try_count < max_tries THEN "error" ELSE ? END`, "task claim expired")
		result := query.Updates(endTaskClaim(TaskStatusFailed, !exhaustedOnly, false, msg))
		cleaned = result.RowsAffected
		return result.Error
	})
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired locks: %w", err)
	}
	return cleaned, nil
}

// TaskOption is a function that configures a task
type TaskOption func(*AppTask)

// WithPriority sets the task priority (higher numbers = higher priority)
func WithPriority(priority int) TaskOption {
	return func(t *AppTask) {
		t.Priority = priority
	}
}

// WithMaxTries sets the maximum number of retry attempts
func WithMaxTries(maxTries int) TaskOption {
	return func(t *AppTask) {
		t.MaxTries = maxTries
	}
}

// WithScheduledAt sets when the task should be processed (for delayed tasks)
func WithScheduledAt(scheduledAt time.Time) TaskOption {
	return func(t *AppTask) {
		t.ScheduledAt = &scheduledAt
	}
}

// WithTaskKey sets a unique key for task deduplication
func WithTaskKey(taskKey string) TaskOption {
	return func(t *AppTask) {
		t.TaskKey = &taskKey
	}
}

// TaskFilters holds filters for listing tasks
type TaskFilters struct {
	Status   TaskStatus
	Type     string
	TaskKey  string
	WorkerID string
	Limit    int
	Offset   int
}
