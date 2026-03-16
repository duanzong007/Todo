package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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

func (s *TaskService) CompletedTasks(ctx context.Context, userID uuid.UUID, limit int) ([]domain.Task, error) {
	return s.repo.ListCompletedTasks(ctx, userID, limit)
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

func (s *TaskService) Postpone(ctx context.Context, userID uuid.UUID, rawID, targetDate string) error {
	taskID, err := uuid.Parse(rawID)
	if err != nil {
		return fmt.Errorf("invalid task id: %w", err)
	}

	parsedDate, err := time.ParseInLocation("2006-01-02", targetDate, s.location)
	if err != nil {
		return fmt.Errorf("invalid target date: %w", err)
	}

	task, err := s.repo.GetTask(ctx, userID, taskID)
	if err != nil {
		return err
	}

	targetValue := normalizeDateInLocation(parsedDate, s.location)
	if task.Type == domain.TaskTypeDDL {
		targetValue = mergeDeadlineDateWithExistingClock(parsedDate.In(s.location), task.Deadline, s.location)
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
