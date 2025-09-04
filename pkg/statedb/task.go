package statedb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// TaskStatus represents the status of a task in the queue
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "PENDING"
	TaskStatusProcessing TaskStatus = "PROCESSING"
	TaskStatusCompleted  TaskStatus = "COMPLETED"
	TaskStatusFailed     TaskStatus = "FAILED"
	TaskStatusRetrying   TaskStatus = "RETRYING"
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
	Error       *string         `gorm:"column:error"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at"`
	ScheduledAt *time.Time      `gorm:"column:scheduled_at"` // for delayed tasks
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

	// If task has a key, check for deduplication
	if task.TaskKey != nil {
		existingTask, err := state.GetTaskByKey(ctx, *task.TaskKey)
		if err != nil {
			return nil, fmt.Errorf("failed to check for existing task: %w", err)
		}
		if existingTask != nil {
			// Task already exists, return the existing one
			return existingTask, nil
		}
	}

	if err := state.DB.WithContext(ctx).Create(task).Error; err != nil {
		// Handle unique constraint violation gracefully
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE constraint") {
			// Another node beat us to it, try to fetch the existing task
			if task.TaskKey != nil {
				existingTask, fetchErr := state.GetTaskByKey(ctx, *task.TaskKey)
				if fetchErr == nil && existingTask != nil {
					return existingTask, nil
				}
			}
		}
		return nil, fmt.Errorf("failed to enqueue task: %w", err)
	}

	go func() {
		select {
		case state.pokeQueue <- struct{}{}:
			// wake up the queue processor
		default:
			// queue is already awake, do nothing
		}
	}()

	return task, nil
}

// DequeueTask retrieves the next available task from the queue and locks it
func (state *StatefulDB) DequeueTask(ctx context.Context, workerID string, taskTypes ...string) (*AppTask, error) {
	var task AppTask

	err := state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("status = ?", TaskStatusPending).
			Where("try_count < max_tries").
			Where("(lock_expires IS NULL OR lock_expires < ?)", time.Now()).
			Where("(scheduled_at IS NULL OR scheduled_at <= ?)", time.Now())

		if len(taskTypes) > 0 {
			query = query.Where("type IN ?", taskTypes)
		}

		// Use raw SQL for PostgreSQL-specific locking
		if state.Type == DBTypePostgres {
			baseQuery := "SELECT * FROM app_tasks WHERE status = ? AND try_count < max_tries AND (lock_expires IS NULL OR lock_expires < ?) AND (scheduled_at IS NULL OR scheduled_at <= ?)"
			if len(taskTypes) > 0 {
				baseQuery += " AND type IN ?"
				params := []interface{}{TaskStatusPending, time.Now(), time.Now(), taskTypes}
				err := tx.Raw(baseQuery+" ORDER BY priority DESC, created_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED", params...).
					Scan(&task).Error
				if err != nil {
					return err
				}
			} else {
				err := tx.Raw(baseQuery+" ORDER BY priority DESC, created_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED",
					TaskStatusPending, time.Now(), time.Now()).
					Scan(&task).Error
				if err != nil {
					return err
				}
			}
		} else {
			// Fallback for SQLite (no SKIP LOCKED support)
			err := query.Order("priority DESC, created_at ASC").First(&task).Error
			if err != nil {
				return err
			}
		}

		if task.ID == 0 {
			return gorm.ErrRecordNotFound
		}

		// Lock the task
		lockExpires := time.Now().Add(30 * time.Minute) // 30-minute lock
		updates := map[string]interface{}{
			"status":       TaskStatusProcessing,
			"worker_id":    workerID,
			"lock_expires": lockExpires,
			"try_count":    task.TryCount + 1,
		}

		return tx.Model(&task).Updates(updates).Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No tasks available
		}
		return nil, fmt.Errorf("failed to dequeue task: %w", err)
	}

	// Reload the task to get updated fields
	if err := state.DB.WithContext(ctx).First(&task, task.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload task: %w", err)
	}

	return &task, nil
}

// CompleteTask marks a task as completed
func (state *StatefulDB) CompleteTask(ctx context.Context, taskID uint) error {
	result := state.DB.WithContext(ctx).Model(&AppTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":       TaskStatusCompleted,
			"lock_expires": nil,
			"worker_id":    nil,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to complete task: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("task not found")
	}

	return nil
}

// FailTask marks a task as failed and optionally retries it
func (state *StatefulDB) FailTask(ctx context.Context, taskID uint, errorMsg string) error {
	var task AppTask
	err := state.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&task, taskID).Error; err != nil {
			return err
		}

		updates := map[string]interface{}{
			"error":        errorMsg,
			"lock_expires": nil,
			"worker_id":    nil,
		}

		if task.TryCount >= task.MaxTries {
			updates["status"] = TaskStatusFailed
		} else {
			updates["status"] = TaskStatusPending
		}

		return tx.Model(&task).Updates(updates).Error
	})

	if err != nil {
		return fmt.Errorf("failed to mark task as failed: %w", err)
	}

	return nil
}

// ReleaseTask releases a locked task back to the queue (e.g., worker shutdown)
func (state *StatefulDB) ReleaseTask(ctx context.Context, taskID uint) error {
	result := state.DB.WithContext(ctx).Model(&AppTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":       TaskStatusPending,
			"lock_expires": nil,
			"worker_id":    nil,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to release task: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("task not found")
	}

	return nil
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

// CleanupExpiredLocks releases tasks with expired locks
func (state *StatefulDB) CleanupExpiredLocks(ctx context.Context) (int64, error) {
	result := state.DB.WithContext(ctx).Model(&AppTask{}).
		Where("status = ? AND lock_expires < ?", TaskStatusProcessing, time.Now()).
		Updates(map[string]interface{}{
			"status":       TaskStatusPending,
			"lock_expires": nil,
			"worker_id":    nil,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup expired locks: %w", result.Error)
	}

	return result.RowsAffected, nil
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
