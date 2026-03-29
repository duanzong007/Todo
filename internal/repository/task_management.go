package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"todo/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type TaskManagementFilter struct {
	Query      string
	Status     string
	Scope      string
	Types      []domain.TaskType
	Importance []int
	DateField  string
	DateFrom   *time.Time
	DateTo     *time.Time
	Sort       string
	TimeZone   string
	Limit      int
}

type ManagedTask struct {
	Task             domain.Task
	OwnerID          uuid.UUID
	OwnerDisplayName string
	OwnerUsername    string
	SharedWithMe     bool
	ShareCount       int
}

func (r *TaskRepository) ListManagedTasks(ctx context.Context, userID uuid.UUID, filter TaskManagementFilter) ([]ManagedTask, error) {
	args := []any{userID}
	var builder strings.Builder

	builder.WriteString(`
		SELECT
			t.user_id,
			owner.display_name,
			owner.username,
			(t.user_id <> $1) AS shared_with_me,
			COALESCE((
				SELECT COUNT(*)
				FROM task_shares share_count
				WHERE share_count.task_id = t.id
			), 0)::int AS share_count,
			t.id,
			t.source_id,
			t.title,
			t.note,
			t.task_type,
			t.status,
			t.importance,
			t.scheduled_for,
			t.deadline,
			t.completed_at,
			t.postponed_count,
			t.metadata,
			t.created_at,
			t.updated_at
		FROM tasks t
		JOIN app_users owner ON owner.id = t.user_id
		WHERE (
			t.user_id = $1
			OR EXISTS (
				SELECT 1
				FROM task_shares visible_share
				WHERE visible_share.task_id = t.id
					AND visible_share.user_id = $1
			)
		)
	`)

	filter.Query = strings.TrimSpace(filter.Query)
	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		index := len(args)
		fmt.Fprintf(&builder, " AND (t.title ILIKE $%d OR t.note ILIKE $%d)", index, index)
	}

	switch filter.Status {
	case "active":
		builder.WriteString(" AND t.status = 'active'")
	case "done":
		builder.WriteString(" AND t.status = 'done'")
	}

	switch filter.Scope {
	case "mine":
		builder.WriteString(" AND t.user_id = $1")
	case "shared":
		builder.WriteString(" AND t.user_id <> $1")
	}

	if len(filter.Types) > 0 {
		builder.WriteString(" AND t.task_type IN (")
		for index, taskType := range filter.Types {
			if index > 0 {
				builder.WriteString(", ")
			}
			args = append(args, taskType)
			fmt.Fprintf(&builder, "$%d", len(args))
		}
		builder.WriteString(")")
	}

	if len(filter.Importance) > 0 {
		builder.WriteString(" AND t.importance IN (")
		for index, importance := range filter.Importance {
			if index > 0 {
				builder.WriteString(", ")
			}
			args = append(args, importance)
			fmt.Fprintf(&builder, "$%d", len(args))
		}
		builder.WriteString(")")
	}

	timeZoneIndex := 0
	ensureTimeZoneArg := func() string {
		if timeZoneIndex != 0 {
			return fmt.Sprintf("$%d", timeZoneIndex)
		}
		timeZone := strings.TrimSpace(filter.TimeZone)
		if timeZone == "" {
			timeZone = "UTC"
		}
		args = append(args, timeZone)
		timeZoneIndex = len(args)
		return fmt.Sprintf("$%d", timeZoneIndex)
	}

	if filter.DateField != "" && (filter.DateFrom != nil || filter.DateTo != nil) {
		dateExpression := taskManagementDateExpression(filter.DateField, ensureTimeZoneArg())
		if dateExpression != "" {
			if filter.DateFrom != nil {
				args = append(args, normalizeDate(*filter.DateFrom))
				fmt.Fprintf(&builder, " AND %s >= $%d", dateExpression, len(args))
			}
			if filter.DateTo != nil {
				args = append(args, normalizeDate(*filter.DateTo))
				fmt.Fprintf(&builder, " AND %s <= $%d", dateExpression, len(args))
			}
		}
	}

	builder.WriteString(" ORDER BY ")
	switch filter.Sort {
	case "created_desc":
		builder.WriteString("t.created_at DESC, t.updated_at DESC")
	case "importance_desc":
		builder.WriteString("t.importance DESC, t.updated_at DESC, t.created_at DESC")
	case "planned_asc":
		dateExpression := taskManagementDateExpression("planned", ensureTimeZoneArg())
		fmt.Fprintf(&builder, "%s ASC NULLS LAST, t.importance DESC, t.updated_at DESC", dateExpression)
	default:
		builder.WriteString("t.updated_at DESC, t.created_at DESC")
	}

	limit := filter.Limit
	if limit <= 0 || limit > 400 {
		limit = 240
	}
	args = append(args, limit)
	fmt.Fprintf(&builder, " LIMIT $%d", len(args))

	rows, err := r.db.Query(ctx, builder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list managed tasks: %w", err)
	}
	defer rows.Close()

	var tasks []ManagedTask
	for rows.Next() {
		task, err := scanManagedTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate managed tasks: %w", err)
	}
	return tasks, nil
}

func (r *TaskRepository) GetManagedTasksByIDs(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) ([]ManagedTask, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			t.user_id,
			owner.display_name,
			owner.username,
			(t.user_id <> $1) AS shared_with_me,
			COALESCE((
				SELECT COUNT(*)
				FROM task_shares share_count
				WHERE share_count.task_id = t.id
			), 0)::int AS share_count,
			t.id,
			t.source_id,
			t.title,
			t.note,
			t.task_type,
			t.status,
			t.importance,
			t.scheduled_for,
			t.deadline,
			t.completed_at,
			t.postponed_count,
			t.metadata,
			t.created_at,
			t.updated_at
		FROM tasks t
		JOIN app_users owner ON owner.id = t.user_id
		WHERE t.id = ANY($2)
			AND (
				t.user_id = $1
				OR EXISTS (
					SELECT 1
					FROM task_shares visible_share
					WHERE visible_share.task_id = t.id
						AND visible_share.user_id = $1
				)
			)
		ORDER BY t.updated_at DESC, t.created_at DESC
	`, userID, uniqueUUIDs(ids))
	if err != nil {
		return nil, fmt.Errorf("get managed tasks by ids: %w", err)
	}
	defer rows.Close()

	var tasks []ManagedTask
	for rows.Next() {
		task, err := scanManagedTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate managed tasks by ids: %w", err)
	}
	return tasks, nil
}

func (r *TaskRepository) ListVisibleUserIDsForTasks(ctx context.Context, taskIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}

	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT visible.user_id
		FROM (
			SELECT user_id
			FROM tasks
			WHERE id = ANY($1)
			UNION ALL
			SELECT user_id
			FROM task_shares
			WHERE task_id = ANY($1)
		) AS visible
		ORDER BY visible.user_id
	`, uniqueUUIDs(taskIDs))
	if err != nil {
		return nil, fmt.Errorf("list visible users for tasks: %w", err)
	}
	defer rows.Close()

	var userIDs []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate visible users for tasks: %w", err)
	}
	return userIDs, nil
}

func (r *TaskRepository) DeleteOwnedTasks(ctx context.Context, ownerID uuid.UUID, taskIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}

	rows, err := r.db.Query(ctx, `
		DELETE FROM tasks
		WHERE user_id = $1
			AND id = ANY($2)
		RETURNING id
	`, ownerID, uniqueUUIDs(taskIDs))
	if err != nil {
		return nil, fmt.Errorf("delete owned tasks: %w", err)
	}
	defer rows.Close()

	var deleted []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		deleted = append(deleted, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deleted tasks: %w", err)
	}
	return deleted, nil
}

func (r *TaskRepository) ShareOwnedTasks(ctx context.Context, ownerID uuid.UUID, taskIDs, userIDs []uuid.UUID) error {
	if len(taskIDs) == 0 || len(userIDs) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	taskIDs = uniqueUUIDs(taskIDs)
	userIDs = uniqueUUIDs(userIDs)

	commandTag, err := tx.Exec(ctx, `
		INSERT INTO task_shares (task_id, user_id, shared_by)
		SELECT DISTINCT owned.id, target_users.id, $1
		FROM tasks AS owned
		JOIN app_users AS target_users
			ON target_users.id = ANY($3)
			AND target_users.is_active = TRUE
			AND target_users.approval_status = 'approved'
		WHERE owned.user_id = $1
			AND owned.id = ANY($2)
			AND target_users.id <> $1
		ON CONFLICT DO NOTHING
	`, ownerID, taskIDs, userIDs)
	if err != nil {
		return fmt.Errorf("share owned tasks: %w", err)
	}

	if commandTag.RowsAffected() > 0 {
		rows, err := tx.Query(ctx, `
			SELECT id
			FROM tasks
			WHERE user_id = $1
				AND id = ANY($2)
		`, ownerID, taskIDs)
		if err != nil {
			return fmt.Errorf("list shared tasks for events: %w", err)
		}

		var sharedTaskIDs []uuid.UUID
		for rows.Next() {
			var taskID uuid.UUID
			if err := rows.Scan(&taskID); err != nil {
				rows.Close()
				return err
			}
			sharedTaskIDs = append(sharedTaskIDs, taskID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate shared task ids: %w", err)
		}
		rows.Close()

		for _, taskID := range sharedTaskIDs {
			if err := createTaskEventTx(ctx, tx, taskID, "shared", map[string]any{
				"shared_user_count": len(userIDs),
			}); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit share owned tasks: %w", err)
	}
	return nil
}

func (r *TaskRepository) RescheduleTask(ctx context.Context, userID, id uuid.UUID, targetValue time.Time) (domain.Task, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	task, err := getTaskTx(ctx, tx, userID, id, true)
	if err != nil {
		return domain.Task{}, err
	}

	eventPayload := map[string]any{}
	var row pgx.Row
	switch task.Type {
	case domain.TaskTypeSchedule:
		scheduledFor := normalizeDate(targetValue)
		if task.ScheduledFor != nil {
			eventPayload["previous_date"] = task.ScheduledFor.Format("2006-01-02")
		}
		eventPayload["target_date"] = scheduledFor.Format("2006-01-02")

		row = tx.QueryRow(ctx, `
			UPDATE tasks
			SET scheduled_for = $2
			WHERE id = $1
			RETURNING id, source_id, title, note, task_type, status, importance, scheduled_for, deadline, completed_at, postponed_count, metadata, created_at, updated_at
		`, id, scheduledFor)
	case domain.TaskTypeDDL:
		deadline := normalizeDeadlineTime(targetValue)
		if task.Deadline != nil {
			eventPayload["previous_at"] = task.Deadline.UTC().Format(time.RFC3339)
		}
		eventPayload["target_at"] = deadline.UTC().Format(time.RFC3339)

		row = tx.QueryRow(ctx, `
			UPDATE tasks
			SET deadline = $2
			WHERE id = $1
			RETURNING id, source_id, title, note, task_type, status, importance, scheduled_for, deadline, completed_at, postponed_count, metadata, created_at, updated_at
		`, id, deadline)
	default:
		return domain.Task{}, ErrUnsupportedOperation
	}

	updatedTask, err := scanTask(row)
	if err != nil {
		return domain.Task{}, err
	}

	if err := createTaskEventTx(ctx, tx, id, "rescheduled", eventPayload); err != nil {
		return domain.Task{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Task{}, fmt.Errorf("commit reschedule task: %w", err)
	}
	return updatedTask, nil
}

func taskManagementDateExpression(dateField string, timeZonePlaceholder string) string {
	switch dateField {
	case "created":
		return fmt.Sprintf("((t.created_at AT TIME ZONE %s)::date)", timeZonePlaceholder)
	case "completed":
		return fmt.Sprintf("((t.completed_at AT TIME ZONE %s)::date)", timeZonePlaceholder)
	case "planned":
		return fmt.Sprintf(`(
			CASE
				WHEN t.task_type = 'schedule' THEN t.scheduled_for::date
				WHEN t.task_type = 'ddl' THEN (t.deadline AT TIME ZONE %s)::date
				ELSE NULL
			END
		)`, timeZonePlaceholder)
	default:
		return ""
	}
}

func scanManagedTask(row interface{ Scan(dest ...any) error }) (ManagedTask, error) {
	var managed ManagedTask
	var sharedWithMe bool
	var sourceID pgtype.UUID
	var scheduledFor pgtype.Date
	var deadline pgtype.Timestamptz
	var completedAt pgtype.Timestamptz
	var metadata []byte

	err := row.Scan(
		&managed.OwnerID,
		&managed.OwnerDisplayName,
		&managed.OwnerUsername,
		&sharedWithMe,
		&managed.ShareCount,
		&managed.Task.ID,
		&sourceID,
		&managed.Task.Title,
		&managed.Task.Note,
		&managed.Task.Type,
		&managed.Task.Status,
		&managed.Task.Importance,
		&scheduledFor,
		&deadline,
		&completedAt,
		&managed.Task.PostponedCount,
		&metadata,
		&managed.Task.CreatedAt,
		&managed.Task.UpdatedAt,
	)
	if err != nil {
		return ManagedTask{}, err
	}

	managed.SharedWithMe = sharedWithMe
	if sourceID.Valid {
		value := uuid.UUID(sourceID.Bytes)
		managed.Task.SourceID = &value
	}
	if scheduledFor.Valid {
		value := normalizeDate(scheduledFor.Time)
		managed.Task.ScheduledFor = &value
	}
	if deadline.Valid {
		value := deadline.Time.UTC()
		managed.Task.Deadline = &value
	}
	if completedAt.Valid {
		value := completedAt.Time
		managed.Task.CompletedAt = &value
	}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &managed.Task.Metadata); err != nil {
			return ManagedTask{}, fmt.Errorf("unmarshal managed task metadata: %w", err)
		}
	}
	if managed.Task.Metadata == nil {
		managed.Task.Metadata = map[string]any{}
	}

	return managed, nil
}

func uniqueUUIDs(values []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(values))
	unique := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}
