package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"todo/internal/domain"
	"todo/internal/repository"

	"github.com/google/uuid"
)

type TaskService struct {
	repo        *repository.TaskRepository
	parser      *TextParser
	icsImporter *ICSImporter
	location    *time.Location
}

func NewTaskService(repo *repository.TaskRepository, parser *TextParser, icsImporter *ICSImporter, location *time.Location) *TaskService {
	return &TaskService{
		repo:        repo,
		parser:      parser,
		icsImporter: icsImporter,
		location:    location,
	}
}

func (s *TaskService) Dashboard(ctx context.Context, userID uuid.UUID) (repository.Dashboard, time.Time, error) {
	now := time.Now().In(s.location)
	dashboard, err := s.repo.ListDashboard(ctx, userID, now)
	if err != nil {
		return repository.Dashboard{}, time.Time{}, err
	}
	return dashboard, now, nil
}

func (s *TaskService) DashboardForDate(ctx context.Context, userID uuid.UUID, focusDate time.Time) (repository.Dashboard, error) {
	return s.repo.ListDashboard(ctx, userID, focusDate.In(s.location))
}

func (s *TaskService) CompletedTasksForDate(ctx context.Context, userID uuid.UUID, focusDate time.Time, limit int) ([]domain.Task, error) {
	dayStart := normalizeDateInLocation(focusDate, s.location)
	dayEnd := dayStart.AddDate(0, 0, 1)
	return s.repo.ListCompletedTasksForDate(ctx, userID, dayStart, dayEnd, limit)
}

func (s *TaskService) CreateFromInput(ctx context.Context, userID uuid.UUID, input string) (domain.Task, error) {
	return s.CreateFromInputWithImportance(ctx, userID, input, 0)
}

func (s *TaskService) CreateFromInputWithImportance(ctx context.Context, userID uuid.UUID, input string, importance int) (domain.Task, error) {
	parsed, err := s.parser.Parse(input, time.Now().In(s.location))
	if err != nil {
		return domain.Task{}, err
	}
	if importance != 0 {
		normalizedImportance, err := domain.NormalizeTaskImportance(importance)
		if err != nil {
			if errors.Is(err, domain.ErrInvalidTaskImportance) {
				return domain.Task{}, fmt.Errorf("重要等级只能在 %d 到 %d 之间", domain.MinTaskImportance, domain.MaxTaskImportance)
			}
			return domain.Task{}, err
		}
		parsed.Task.Importance = normalizedImportance
	}

	source := repository.SourceInput{
		Type:       parsed.SourceType,
		RawContent: input,
		Summary:    parsed.SourceSummary,
		Checksum:   checksumString(input),
		Metadata:   parsed.SourceMetadata,
	}

	return s.repo.CreateTask(ctx, userID, source, parsed.Task)
}

func (s *TaskService) ImportICS(ctx context.Context, userID uuid.UUID, filename string, body []byte) (int, error) {
	result, err := s.icsImporter.Parse(filename, body, time.Now().In(s.location))
	if err != nil {
		return 0, err
	}

	source := repository.SourceInput{
		Type:       domain.SourceTypeICSImport,
		RawContent: string(body),
		Summary:    result.SourceSummary,
		Checksum:   checksumBytes(body),
		Metadata:   result.SourceMeta,
	}

	return s.repo.ImportTasks(ctx, userID, source, result.Tasks)
}

func (s *TaskService) Complete(ctx context.Context, userID uuid.UUID, rawID string) (domain.Task, error) {
	taskID, err := uuid.Parse(rawID)
	if err != nil {
		return domain.Task{}, fmt.Errorf("invalid task id: %w", err)
	}

	return s.repo.CompleteTask(ctx, userID, taskID)
}

func (s *TaskService) Restore(ctx context.Context, userID uuid.UUID, rawID string) (domain.Task, error) {
	taskID, err := uuid.Parse(rawID)
	if err != nil {
		return domain.Task{}, fmt.Errorf("invalid task id: %w", err)
	}

	return s.repo.RestoreTask(ctx, userID, taskID)
}

func (s *TaskService) Postpone(ctx context.Context, userID uuid.UUID, rawID, targetDate string) error {
	taskID, err := uuid.Parse(rawID)
	if err != nil {
		return fmt.Errorf("invalid task id: %w", err)
	}

	task, err := s.repo.GetTask(ctx, userID, taskID)
	if err != nil {
		return err
	}

	targetValue, err := parsePostponeTarget(task, strings.TrimSpace(targetDate), time.Now().In(s.location), s.location)
	if err != nil {
		return err
	}

	_, err = s.repo.PostponeTask(ctx, userID, taskID, targetValue)
	return err
}

func checksumString(input string) string {
	return checksumBytes([]byte(input))
}

func checksumBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func mergeDeadlineDateWithExistingClock(targetDate time.Time, existing *time.Time, location *time.Location) time.Time {
	dateValue := normalizeDateInLocation(targetDate, location)
	if existing == nil {
		return time.Date(dateValue.Year(), dateValue.Month(), dateValue.Day(), 23, 59, 0, 0, location)
	}

	existingLocal := existing.In(location)
	return time.Date(
		dateValue.Year(),
		dateValue.Month(),
		dateValue.Day(),
		existingLocal.Hour(),
		existingLocal.Minute(),
		0,
		0,
		location,
	)
}

func parsePostponeTarget(task domain.Task, rawTarget string, now time.Time, location *time.Location) (time.Time, error) {
	switch task.Type {
	case domain.TaskTypeSchedule:
		parsedDate, err := time.ParseInLocation("2006-01-02", rawTarget, location)
		if err != nil {
			return time.Time{}, fmt.Errorf("日程延期日期格式不正确")
		}

		targetValue := normalizeDateInLocation(parsedDate, location)
		minimum := earliestSchedulePostponeDate(task, now, location)
		if targetValue.Before(minimum) {
			return time.Time{}, fmt.Errorf("日程只能延期到更晚的日期")
		}
		return targetValue, nil
	case domain.TaskTypeDDL:
		parsedDateTime, err := time.ParseInLocation("2006-01-02T15:04", rawTarget, location)
		if err != nil {
			return time.Time{}, fmt.Errorf("DDL 延期时间格式不正确")
		}

		targetValue := time.Date(
			parsedDateTime.Year(),
			parsedDateTime.Month(),
			parsedDateTime.Day(),
			parsedDateTime.Hour(),
			parsedDateTime.Minute(),
			0,
			0,
			location,
		)
		minimum := earliestDDLPostponeTime(task, now, location)
		if targetValue.Before(minimum) {
			return time.Time{}, fmt.Errorf("DDL 只能延期到更晚的时间")
		}
		return targetValue, nil
	default:
		return time.Time{}, repository.ErrUnsupportedOperation
	}
}

func earliestSchedulePostponeDate(task domain.Task, now time.Time, location *time.Location) time.Time {
	base := normalizeDateInLocation(now, location)
	if task.ScheduledFor != nil {
		scheduled := normalizeDateInLocation(*task.ScheduledFor, location)
		if scheduled.After(base) {
			base = scheduled
		}
	}
	return base.AddDate(0, 0, 1)
}

func earliestDDLPostponeTime(task domain.Task, now time.Time, location *time.Location) time.Time {
	base := now.In(location)
	if task.Deadline != nil {
		deadline := task.Deadline.In(location)
		if deadline.After(base) {
			base = deadline
		}
	}

	local := base.In(location)
	rounded := time.Date(
		local.Year(),
		local.Month(),
		local.Day(),
		local.Hour(),
		local.Minute(),
		0,
		0,
		location,
	)
	if !rounded.After(local) {
		rounded = rounded.Add(time.Minute)
	}
	return rounded
}
