package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"todo/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTaskNotFound          = errors.New("task not found")
	ErrUnsupportedOperation  = errors.New("unsupported operation")
	ErrInvalidTaskTransition = errors.New("invalid task transition")
)

type Dashboard struct {
	Today []domain.Task
	DDL   []domain.Task
	Todo  []domain.Task
}

type SourceInput struct {
	Type       domain.SourceType
	RawContent string
	Summary    string
	Checksum   string
	Metadata   map[string]any
}

type TaskInput struct {
	Title        string
	Note         string
	Type         domain.TaskType
	ScheduledFor *time.Time
	Deadline     *time.Time
	Metadata     map[string]any
}

type TaskRepository struct {
	db *pgxpool.Pool
}

func NewTaskRepository(db *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) CreateTask(ctx context.Context, source SourceInput, input TaskInput) (domain.Task, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	sourceID, err := createSourceTx(ctx, tx, source)
	if err != nil {
		return domain.Task{}, err
	}

	task, inserted, err := insertTaskTx(ctx, tx, &sourceID, input, false)
	if err != nil {
		return domain.Task{}, err
	}
	if !inserted {
		return domain.Task{}, fmt.Errorf("task was not inserted")
	}

	if err := createTaskEventTx(ctx, tx, task.ID, "created", map[string]any{
		"source_type": source.Type,
	}); err != nil {
		return domain.Task{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Task{}, fmt.Errorf("commit create task: %w", err)
	}

	return task, nil
}

func (r *TaskRepository) ImportTasks(ctx context.Context, source SourceInput, inputs []TaskInput) (int, error) {
	if len(inputs) == 0 {
		return 0, nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	sourceID, err := createSourceTx(ctx, tx, source)
	if err != nil {
		return 0, err
	}

	insertedCount := 0
	for _, input := range inputs {
		task, inserted, err := insertTaskTx(ctx, tx, &sourceID, input, true)
		if err != nil {
			return 0, err
		}
		if !inserted {
			continue
		}

		insertedCount++
		if err := createTaskEventTx(ctx, tx, task.ID, "imported", map[string]any{
			"source_type": source.Type,
		}); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit import tasks: %w", err)
	}

	return insertedCount, nil
}

func (r *TaskRepository) ListDashboard(ctx context.Context, today time.Time) (Dashboard, error) {
	dateOnly := normalizeDate(today)

	todayTasks, err := r.listTasks(ctx, `
		SELECT id, source_id, title, note, task_type, status, scheduled_for, deadline, completed_at, postponed_count, metadata, created_at, updated_at
		FROM tasks
		WHERE status = 'active' AND task_type = 'schedule' AND scheduled_for = $1
		ORDER BY scheduled_for, created_at
	`, dateOnly)
	if err != nil {
		return Dashboard{}, err
	}

	ddlTasks, err := r.listTasks(ctx, `
		SELECT id, source_id, title, note, task_type, status, scheduled_for, deadline, completed_at, postponed_count, metadata, created_at, updated_at
		FROM tasks
		WHERE status = 'active' AND task_type = 'ddl'
		ORDER BY deadline, created_at
	`)
	if err != nil {
		return Dashboard{}, err
	}

	todoTasks, err := r.listTasks(ctx, `
		SELECT id, source_id, title, note, task_type, status, scheduled_for, deadline, completed_at, postponed_count, metadata, created_at, updated_at
		FROM tasks
		WHERE status = 'active' AND task_type = 'todo'
		ORDER BY created_at
	`)
	if err != nil {
		return Dashboard{}, err
	}

	return Dashboard{
		Today: todayTasks,
		DDL:   ddlTasks,
		Todo:  todoTasks,
	}, nil
}

func (r *TaskRepository) CompleteTask(ctx context.Context, id uuid.UUID) (domain.Task, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	task, err := getTaskTx(ctx, tx, id, true)
	if err != nil {
		return domain.Task{}, err
	}
	if !task.SupportsCompletion() {
		return domain.Task{}, ErrUnsupportedOperation
	}
	if task.Status != domain.TaskStatusActive {
		return domain.Task{}, ErrInvalidTaskTransition
	}

	row := tx.QueryRow(ctx, `
		UPDATE tasks
		SET status = 'done', completed_at = NOW()
		WHERE id = $1
		RETURNING id, source_id, title, note, task_type, status, scheduled_for, deadline, completed_at, postponed_count, metadata, created_at, updated_at
	`, id)

	updatedTask, err := scanTask(row)
	if err != nil {
		return domain.Task{}, err
	}

	if err := createTaskEventTx(ctx, tx, id, "completed", map[string]any{
		"previous_status": task.Status,
	}); err != nil {
		return domain.Task{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Task{}, fmt.Errorf("commit complete task: %w", err)
	}

	return updatedTask, nil
}

func (r *TaskRepository) PostponeTask(ctx context.Context, id uuid.UUID, targetDate time.Time) (domain.Task, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	task, err := getTaskTx(ctx, tx, id, true)
	if err != nil {
		return domain.Task{}, err
	}
	if !task.SupportsPostpone() {
		return domain.Task{}, ErrUnsupportedOperation
	}
	if task.Status != domain.TaskStatusActive {
		return domain.Task{}, ErrInvalidTaskTransition
	}

	targetDate = normalizeDate(targetDate)

	var previousDate string
	switch task.Type {
	case domain.TaskTypeSchedule:
		if task.ScheduledFor != nil {
			previousDate = task.ScheduledFor.Format("2006-01-02")
		}
	case domain.TaskTypeDDL:
		if task.Deadline != nil {
			previousDate = task.Deadline.Format("2006-01-02")
		}
	}

	row := tx.QueryRow(ctx, `
		UPDATE tasks
		SET
			scheduled_for = CASE WHEN task_type = 'schedule' THEN $2 ELSE scheduled_for END,
			deadline = CASE WHEN task_type = 'ddl' THEN $2 ELSE deadline END,
			postponed_count = postponed_count + 1
		WHERE id = $1
		RETURNING id, source_id, title, note, task_type, status, scheduled_for, deadline, completed_at, postponed_count, metadata, created_at, updated_at
	`, id, targetDate)

	updatedTask, err := scanTask(row)
	if err != nil {
		return domain.Task{}, err
	}

	if err := createTaskEventTx(ctx, tx, id, "postponed", map[string]any{
		"previous_date": previousDate,
		"target_date":   targetDate.Format("2006-01-02"),
	}); err != nil {
		return domain.Task{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Task{}, fmt.Errorf("commit postpone task: %w", err)
	}

	return updatedTask, nil
}

func (r *TaskRepository) listTasks(ctx context.Context, query string, args ...any) ([]domain.Task, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []domain.Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}

	return tasks, nil
}

func createSourceTx(ctx context.Context, tx pgx.Tx, input SourceInput) (uuid.UUID, error) {
	metadata, err := marshalMetadata(input.Metadata)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal source metadata: %w", err)
	}

	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO ingestion_sources (source_type, raw_content, summary, checksum, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, input.Type, input.RawContent, input.Summary, input.Checksum, metadata).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert ingestion source: %w", err)
	}

	return id, nil
}

func insertTaskTx(ctx context.Context, tx pgx.Tx, sourceID *uuid.UUID, input TaskInput, skipOnConflict bool) (domain.Task, bool, error) {
	metadata, err := marshalMetadata(input.Metadata)
	if err != nil {
		return domain.Task{}, false, fmt.Errorf("marshal task metadata: %w", err)
	}

	query := `
		INSERT INTO tasks (source_id, title, note, task_type, scheduled_for, deadline, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if skipOnConflict {
		query += ` ON CONFLICT DO NOTHING`
	}
	query += `
		RETURNING id, source_id, title, note, task_type, status, scheduled_for, deadline, completed_at, postponed_count, metadata, created_at, updated_at
	`

	row := tx.QueryRow(ctx, query, sourceID, input.Title, input.Note, input.Type, normalizeDatePtr(input.ScheduledFor), normalizeDatePtr(input.Deadline), metadata)
	task, err := scanTask(row)
	if err != nil {
		if skipOnConflict && errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, false, nil
		}
		return domain.Task{}, false, err
	}

	return task, true, nil
}

func getTaskTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, lock bool) (domain.Task, error) {
	query := `
		SELECT id, source_id, title, note, task_type, status, scheduled_for, deadline, completed_at, postponed_count, metadata, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`
	if lock {
		query += ` FOR UPDATE`
	}

	row := tx.QueryRow(ctx, query, id)
	task, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, ErrTaskNotFound
		}
		return domain.Task{}, err
	}
	return task, nil
}

func createTaskEventTx(ctx context.Context, tx pgx.Tx, taskID uuid.UUID, eventType string, payload map[string]any) error {
	body, err := marshalMetadata(payload)
	if err != nil {
		return fmt.Errorf("marshal task event payload: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO task_events (task_id, event_type, payload)
		VALUES ($1, $2, $3)
	`, taskID, eventType, body)
	if err != nil {
		return fmt.Errorf("insert task event: %w", err)
	}
	return nil
}

func scanTask(row interface{ Scan(dest ...any) error }) (domain.Task, error) {
	var task domain.Task
	var sourceID pgtype.UUID
	var scheduledFor pgtype.Date
	var deadline pgtype.Date
	var completedAt pgtype.Timestamptz
	var metadata []byte

	err := row.Scan(
		&task.ID,
		&sourceID,
		&task.Title,
		&task.Note,
		&task.Type,
		&task.Status,
		&scheduledFor,
		&deadline,
		&completedAt,
		&task.PostponedCount,
		&metadata,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return domain.Task{}, err
	}

	if sourceID.Valid {
		value := uuid.UUID(sourceID.Bytes)
		task.SourceID = &value
	}
	if scheduledFor.Valid {
		value := normalizeDate(scheduledFor.Time)
		task.ScheduledFor = &value
	}
	if deadline.Valid {
		value := normalizeDate(deadline.Time)
		task.Deadline = &value
	}
	if completedAt.Valid {
		value := completedAt.Time
		task.CompletedAt = &value
	}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &task.Metadata); err != nil {
			return domain.Task{}, fmt.Errorf("unmarshal task metadata: %w", err)
		}
	}
	if task.Metadata == nil {
		task.Metadata = map[string]any{}
	}

	return task, nil
}

func marshalMetadata(metadata map[string]any) ([]byte, error) {
	if len(metadata) == 0 {
		return []byte(`{}`), nil
	}
	return json.Marshal(metadata)
}

func normalizeDatePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	normalized := normalizeDate(*value)
	return normalized
}

func normalizeDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}
