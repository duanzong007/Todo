package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
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
		normalizedImportance, err := normalizeImportanceValue(importance)
		if err != nil {
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

func (s *TaskService) CreateManualTask(ctx context.Context, userID uuid.UUID, input repository.TaskInput) (domain.Task, error) {
	cleanInput, err := s.normalizeManualTaskInput(input)
	if err != nil {
		return domain.Task{}, err
	}

	source := repository.SourceInput{
		Type:       domain.SourceTypeManualText,
		RawContent: manualSourceRawContent(cleanInput),
		Summary:    cleanInput.Title,
		Checksum:   checksumString(manualSourceRawContent(cleanInput)),
		Metadata: map[string]any{
			"entry":     "manual_form",
			"task_type": string(cleanInput.Type),
		},
	}

	return s.repo.CreateTask(ctx, userID, source, cleanInput)
}

func (s *TaskService) CreateManualTasks(ctx context.Context, userID uuid.UUID, inputs []repository.TaskInput) (int, error) {
	if len(inputs) == 0 {
		return 0, fmt.Errorf("没有可创建的任务")
	}
	if len(inputs) == 1 {
		if _, err := s.CreateManualTask(ctx, userID, inputs[0]); err != nil {
			return 0, err
		}
		return 1, nil
	}

	cleanInputs := make([]repository.TaskInput, 0, len(inputs))
	for _, input := range inputs {
		cleanInput, err := s.normalizeManualTaskInput(input)
		if err != nil {
			return 0, err
		}
		cleanInputs = append(cleanInputs, cleanInput)
	}

	rawContent := manualBatchSourceRawContent(cleanInputs)
	source := repository.SourceInput{
		Type:       domain.SourceTypeManualText,
		RawContent: rawContent,
		Summary:    cleanInputs[0].Title,
		Checksum:   checksumString(rawContent),
		Metadata: map[string]any{
			"entry":      "manual_form",
			"task_type":  string(cleanInputs[0].Type),
			"mode":       "batch",
			"task_count": len(cleanInputs),
		},
	}

	return s.repo.CreateTasks(ctx, userID, source, cleanInputs)
}

func (s *TaskService) normalizeManualTaskInput(input repository.TaskInput) (repository.TaskInput, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return repository.TaskInput{}, fmt.Errorf("标题不能为空")
	}

	normalizedImportance, err := normalizeImportanceValue(input.Importance)
	if err != nil {
		return repository.TaskInput{}, err
	}

	cleanInput := repository.TaskInput{
		Title:      title,
		Note:       strings.TrimSpace(input.Note),
		Type:       input.Type,
		Importance: normalizedImportance,
		Metadata: map[string]any{
			"creator": "manual_form",
		},
	}

	switch input.Type {
	case domain.TaskTypeTodo:
	case domain.TaskTypeSchedule:
		if input.ScheduledFor == nil {
			return repository.TaskInput{}, fmt.Errorf("请选择日程日期")
		}
		dateValue := normalizeDateInLocation(*input.ScheduledFor, s.location)
		cleanInput.ScheduledFor = &dateValue
	case domain.TaskTypeDDL:
		if input.Deadline == nil {
			return repository.TaskInput{}, fmt.Errorf("请选择截止时间")
		}
		deadline := time.Date(
			input.Deadline.In(s.location).Year(),
			input.Deadline.In(s.location).Month(),
			input.Deadline.In(s.location).Day(),
			input.Deadline.In(s.location).Hour(),
			input.Deadline.In(s.location).Minute(),
			0,
			0,
			s.location,
		)
		cleanInput.Deadline = &deadline
	default:
		return repository.TaskInput{}, repository.ErrUnsupportedOperation
	}

	return cleanInput, nil
}

func (s *TaskService) CreateFromSMSParse(ctx context.Context, userID uuid.UUID, input string) (int, error) {
	parsedTasks, err := s.parser.ParseSMSBatch(input, time.Now().In(s.location))
	if err != nil {
		return 0, err
	}

	inputs := make([]repository.TaskInput, 0, len(parsedTasks))
	for _, parsed := range parsedTasks {
		if parsed.SourceType != domain.SourceTypeSMSPaste {
			continue
		}
		parsed.Task.Importance = 2
		inputs = append(inputs, parsed.Task)
	}

	if len(inputs) == 0 {
		return 0, fmt.Errorf("暂时只支持解析这些取件短信")
	}

	source := repository.SourceInput{
		Type:       domain.SourceTypeSMSPaste,
		RawContent: input,
		Summary:    fmt.Sprintf("短信解析: %d 条", len(inputs)),
		Checksum:   checksumString(input),
		Metadata: map[string]any{
			"parser":        "pickup_sms",
			"message_count": len(inputs),
		},
	}

	return s.repo.CreateTasks(ctx, userID, source, inputs)
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

func (s *TaskService) Rename(ctx context.Context, userID uuid.UUID, rawID, title string, importance *int) (domain.Task, error) {
	taskID, err := uuid.Parse(rawID)
	if err != nil {
		return domain.Task{}, fmt.Errorf("invalid task id: %w", err)
	}

	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return domain.Task{}, fmt.Errorf("标题不能为空")
	}

	var normalizedImportance *int
	if importance != nil {
		value, err := normalizeImportanceValue(*importance)
		if err != nil {
			return domain.Task{}, err
		}
		normalizedImportance = &value
	}

	return s.repo.RenameTask(ctx, userID, taskID, trimmedTitle, normalizedImportance)
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

func normalizeImportanceValue(value int) (int, error) {
	normalizedImportance, err := domain.NormalizeTaskImportance(value)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidTaskImportance) {
			return 0, fmt.Errorf("重要等级只能在 %d 到 %d 之间", domain.MinTaskImportance, domain.MaxTaskImportance)
		}
		return 0, err
	}
	return normalizedImportance, nil
}

func manualSourceRawContent(input repository.TaskInput) string {
	lines := []string{
		"type=" + string(input.Type),
		"title=" + input.Title,
		"importance=" + strconv.Itoa(input.Importance),
	}
	if input.Note != "" {
		lines = append(lines, "note="+input.Note)
	}
	if input.ScheduledFor != nil {
		lines = append(lines, "scheduled_for="+input.ScheduledFor.In(time.UTC).Format(time.RFC3339))
	}
	if input.Deadline != nil {
		lines = append(lines, "deadline="+input.Deadline.In(time.UTC).Format(time.RFC3339))
	}
	return strings.Join(lines, "\n")
}

func manualBatchSourceRawContent(inputs []repository.TaskInput) string {
	blocks := make([]string, 0, len(inputs))
	for index, input := range inputs {
		blocks = append(blocks, fmt.Sprintf("[task %d]\n%s", index+1, manualSourceRawContent(input)))
	}
	return strings.Join(blocks, "\n\n")
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
