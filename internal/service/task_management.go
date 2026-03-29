package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"todo/internal/domain"
	"todo/internal/repository"

	"github.com/google/uuid"
)

type TaskManagementActionInput struct {
	Action          string
	SelectedTaskIDs []string
	ReplaceTitle    string
	Prefix          string
	Suffix          string
	Importance      string
	ScheduleDate    string
	DeadlineDate    string
	DeadlineTime    string
	DeadlineValue   string
	ShareUserIDs    []string
}

type TaskManagementActionResult struct {
	Message         string
	AudienceUserIDs []uuid.UUID
}

func (s *TaskService) ListManagedTasks(ctx context.Context, userID uuid.UUID, filter repository.TaskManagementFilter) ([]repository.ManagedTask, int, error) {
	return s.repo.ListManagedTasks(ctx, userID, filter)
}

func (s *TaskService) VisibleUserIDsForTask(ctx context.Context, rawID string) ([]uuid.UUID, error) {
	taskID, err := parseUUID(rawID)
	if err != nil {
		return nil, fmt.Errorf("invalid task id: %w", err)
	}
	return s.repo.ListVisibleUserIDsForTasks(ctx, []uuid.UUID{taskID})
}

func (s *TaskService) VisibleUserIDsForTasks(ctx context.Context, rawIDs []string) ([]uuid.UUID, error) {
	taskIDs, err := parseUUIDList(rawIDs)
	if err != nil {
		return nil, fmt.Errorf("invalid task ids: %w", err)
	}
	return s.repo.ListVisibleUserIDsForTasks(ctx, taskIDs)
}

func (s *TaskService) ApplyManagementAction(ctx context.Context, actor domain.User, input TaskManagementActionInput) (TaskManagementActionResult, error) {
	taskIDs, err := parseUUIDList(input.SelectedTaskIDs)
	if err != nil {
		return TaskManagementActionResult{}, fmt.Errorf("任务选择无效: %w", err)
	}
	if len(taskIDs) == 0 {
		return TaskManagementActionResult{}, fmt.Errorf("请先选择至少一条任务")
	}

	selectedTasks, err := s.repo.GetManagedTasksByIDs(ctx, actor.ID, taskIDs)
	if err != nil {
		return TaskManagementActionResult{}, err
	}
	if len(selectedTasks) == 0 {
		return TaskManagementActionResult{}, fmt.Errorf("没有可操作的任务")
	}
	if len(selectedTasks) != len(taskIDs) {
		return TaskManagementActionResult{}, fmt.Errorf("部分任务不存在或你没有权限")
	}

	switch strings.TrimSpace(input.Action) {
	case "delete":
		return s.applyDeleteAction(ctx, actor, taskIDs, selectedTasks)
	case "share":
		return s.applyShareAction(ctx, actor, taskIDs, selectedTasks, input.ShareUserIDs)
	default:
		return s.applyPatchAction(ctx, actor, selectedTasks, input)
	}
}

func (s *TaskService) applyDeleteAction(ctx context.Context, actor domain.User, taskIDs []uuid.UUID, selectedTasks []repository.ManagedTask) (TaskManagementActionResult, error) {
	for _, task := range selectedTasks {
		if task.OwnerID != actor.ID {
			return TaskManagementActionResult{}, fmt.Errorf("共享给你的任务不能删除")
		}
	}

	audience, err := s.repo.ListVisibleUserIDsForTasks(ctx, taskIDs)
	if err != nil {
		return TaskManagementActionResult{}, err
	}

	deleted, err := s.repo.DeleteOwnedTasks(ctx, actor.ID, taskIDs)
	if err != nil {
		return TaskManagementActionResult{}, err
	}
	if len(deleted) == 0 {
		return TaskManagementActionResult{}, fmt.Errorf("没有可删除的任务")
	}

	return TaskManagementActionResult{
		Message:         fmt.Sprintf("已删除 %d 条任务", len(deleted)),
		AudienceUserIDs: audience,
	}, nil
}

func (s *TaskService) applyShareAction(ctx context.Context, actor domain.User, taskIDs []uuid.UUID, selectedTasks []repository.ManagedTask, rawUserIDs []string) (TaskManagementActionResult, error) {
	for _, task := range selectedTasks {
		if task.OwnerID != actor.ID {
			return TaskManagementActionResult{}, fmt.Errorf("只有你创建的任务才能继续共享")
		}
	}

	userIDs, err := parseUUIDList(rawUserIDs)
	if err != nil {
		return TaskManagementActionResult{}, fmt.Errorf("共享用户无效: %w", err)
	}
	if len(userIDs) == 0 {
		return TaskManagementActionResult{}, fmt.Errorf("请至少选择一个共享对象")
	}

	if err := s.repo.ShareOwnedTasks(ctx, actor.ID, taskIDs, userIDs); err != nil {
		return TaskManagementActionResult{}, err
	}

	audience, err := s.repo.ListVisibleUserIDsForTasks(ctx, taskIDs)
	if err != nil {
		return TaskManagementActionResult{}, err
	}

	return TaskManagementActionResult{
		Message:         fmt.Sprintf("已把 %d 条任务共享出去", len(selectedTasks)),
		AudienceUserIDs: audience,
	}, nil
}

func (s *TaskService) applyPatchAction(ctx context.Context, actor domain.User, selectedTasks []repository.ManagedTask, input TaskManagementActionInput) (TaskManagementActionResult, error) {
	_ = actor

	importanceValue, hasImportance, err := parseOptionalImportance(input.Importance)
	if err != nil {
		return TaskManagementActionResult{}, err
	}

	replaceTitle := strings.TrimSpace(input.ReplaceTitle)
	prefix := strings.TrimSpace(input.Prefix)
	suffix := strings.TrimSpace(input.Suffix)

	appliedCount := 0
	for _, selected := range selectedTasks {
		currentTask := selected.Task
		var nextImportance *int
		if hasImportance {
			value := importanceValue
			nextImportance = &value
		}

		nextTitle := currentTask.Title
		if len(selectedTasks) == 1 && replaceTitle != "" {
			nextTitle = replaceTitle
		}
		if prefix != "" || suffix != "" {
			nextTitle = prefix + nextTitle + suffix
		}

		changedTitle := strings.TrimSpace(nextTitle) != currentTask.Title
		changedImportance := nextImportance != nil && *nextImportance != currentTask.Importance
		if changedTitle || changedImportance {
			if _, err := s.repo.RenameTask(ctx, actor.ID, currentTask.ID, strings.TrimSpace(nextTitle), nextImportance); err != nil {
				return TaskManagementActionResult{}, err
			}
			appliedCount++
		}
	}

	if len(selectedTasks) == 1 {
		selected := selectedTasks[0]
		timeChanged, err := s.applySingleTimePatch(ctx, actor, selected.Task, input)
		if err != nil {
			return TaskManagementActionResult{}, err
		}
		if timeChanged {
			appliedCount++
		}
	} else {
		if strings.TrimSpace(input.ScheduleDate) != "" || strings.TrimSpace(input.DeadlineDate) != "" || strings.TrimSpace(input.DeadlineTime) != "" || strings.TrimSpace(input.DeadlineValue) != "" {
			return TaskManagementActionResult{}, fmt.Errorf("时间修改只支持单选任务")
		}
		if replaceTitle != "" {
			return TaskManagementActionResult{}, fmt.Errorf("批量修改标题时请使用前缀或后缀")
		}
	}

	if appliedCount == 0 {
		return TaskManagementActionResult{}, fmt.Errorf("没有可应用的修改")
	}

	taskIDs := make([]uuid.UUID, 0, len(selectedTasks))
	for _, task := range selectedTasks {
		taskIDs = append(taskIDs, task.Task.ID)
	}

	audience, err := s.repo.ListVisibleUserIDsForTasks(ctx, taskIDs)
	if err != nil {
		return TaskManagementActionResult{}, err
	}

	return TaskManagementActionResult{
		Message:         fmt.Sprintf("已更新 %d 条任务", len(selectedTasks)),
		AudienceUserIDs: audience,
	}, nil
}

func (s *TaskService) applySingleTimePatch(ctx context.Context, actor domain.User, task domain.Task, input TaskManagementActionInput) (bool, error) {
	switch task.Type {
	case domain.TaskTypeSchedule:
		rawDate := strings.TrimSpace(input.ScheduleDate)
		if rawDate == "" {
			return false, nil
		}
		targetDate, err := time.ParseInLocation("2006-01-02", rawDate, s.location)
		if err != nil {
			return false, fmt.Errorf("日程日期无效")
		}
		targetDate = normalizeDateInLocation(targetDate, s.location)
		if task.ScheduledFor != nil && normalizeDateInLocation(*task.ScheduledFor, s.location).Equal(targetDate) {
			return false, nil
		}
		if _, err := s.repo.RescheduleTask(ctx, actor.ID, task.ID, targetDate); err != nil {
			return false, err
		}
		return true, nil
	case domain.TaskTypeDDL:
		rawDate := strings.TrimSpace(input.DeadlineDate)
		rawTime := strings.TrimSpace(input.DeadlineTime)
		if combined := strings.TrimSpace(input.DeadlineValue); combined != "" {
			if parsed, err := time.ParseInLocation("2006-01-02T15:04", combined, s.location); err == nil {
				rawDate = parsed.Format("2006-01-02")
				rawTime = parsed.Format("15:04")
			} else {
				return false, fmt.Errorf("DDL 时间无效")
			}
		}
		if rawDate == "" && rawTime == "" {
			return false, nil
		}
		if rawDate == "" || rawTime == "" {
			return false, fmt.Errorf("请完整填写 DDL 的日期和时间")
		}
		targetDate, err := time.ParseInLocation("2006-01-02 15:04", rawDate+" "+rawTime, s.location)
		if err != nil {
			return false, fmt.Errorf("DDL 时间无效")
		}
		if task.Deadline != nil {
			current := task.Deadline.In(s.location)
			if current.Year() == targetDate.Year() &&
				current.Month() == targetDate.Month() &&
				current.Day() == targetDate.Day() &&
				current.Hour() == targetDate.Hour() &&
				current.Minute() == targetDate.Minute() {
				return false, nil
			}
		}
		if _, err := s.repo.RescheduleTask(ctx, actor.ID, task.ID, targetDate); err != nil {
			return false, err
		}
		return true, nil
	default:
		if strings.TrimSpace(input.ScheduleDate) != "" || strings.TrimSpace(input.DeadlineDate) != "" || strings.TrimSpace(input.DeadlineTime) != "" || strings.TrimSpace(input.DeadlineValue) != "" {
			return false, fmt.Errorf("Todo 没有可修改的时间")
		}
		return false, nil
	}
}

func parseOptionalImportance(raw string) (int, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false, nil
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, false, fmt.Errorf("重要等级无效")
	}
	normalized, err := normalizeImportanceValue(value)
	if err != nil {
		return 0, false, err
	}
	return normalized, true, nil
}

func parseUUIDList(values []string) ([]uuid.UUID, error) {
	seen := make(map[uuid.UUID]struct{}, len(values))
	parsed := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		id, err := parseUUID(trimmed)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		parsed = append(parsed, id)
	}
	return parsed, nil
}
