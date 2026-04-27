package web

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"todo/internal/domain"
	"todo/internal/repository"

	"github.com/google/uuid"
)

func TestBuildFocusTaskCardsSortsByImportanceThenUrgency(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 3, 16, 10, 15, 0, 0, location)
	focusDate := time.Date(2026, 3, 16, 9, 0, 0, 0, location)
	createdAt := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)

	scheduleToday := time.Date(2026, 3, 16, 0, 0, 0, 0, location)
	ddlToday := time.Date(2026, 3, 16, 18, 0, 0, 0, location)
	ddlTodayHigh := time.Date(2026, 3, 16, 20, 0, 0, 0, location)
	ddlTomorrow := time.Date(2026, 3, 17, 18, 0, 0, 0, location)

	dashboard := repository.Dashboard{
		Today: []domain.Task{
			{
				ID:           uuid.New(),
				Title:        "普通日程",
				Type:         domain.TaskTypeSchedule,
				Importance:   4,
				ScheduledFor: &scheduleToday,
				CreatedAt:    createdAt.Add(1 * time.Minute),
			},
		},
		DDL: []domain.Task{
			{
				ID:         uuid.New(),
				Title:      "今天截止低优先",
				Type:       domain.TaskTypeDDL,
				Importance: 2,
				Deadline:   &ddlToday,
				CreatedAt:  createdAt.Add(2 * time.Minute),
			},
			{
				ID:         uuid.New(),
				Title:      "今天截止高优先",
				Type:       domain.TaskTypeDDL,
				Importance: 5,
				Deadline:   &ddlTodayHigh,
				CreatedAt:  createdAt.Add(3 * time.Minute),
			},
			{
				ID:         uuid.New(),
				Title:      "明天截止",
				Type:       domain.TaskTypeDDL,
				Importance: 5,
				Deadline:   &ddlTomorrow,
				CreatedAt:  createdAt.Add(4 * time.Minute),
			},
		},
		Todo: []domain.Task{
			{
				ID:         uuid.New(),
				Title:      "高优先待办",
				Type:       domain.TaskTypeTodo,
				Importance: 5,
				CreatedAt:  createdAt,
			},
		},
	}

	cards := buildFocusTaskCards(dashboard, now, focusDate, location)
	if len(cards) != 5 {
		t.Fatalf("len(cards) = %d, want 5", len(cards))
	}

	gotTitles := []string{cards[0].Title, cards[1].Title, cards[2].Title, cards[3].Title, cards[4].Title}
	wantTitles := []string{"今天截止高优先", "今天截止低优先", "明天截止", "高优先待办", "普通日程"}
	for index := range wantTitles {
		if gotTitles[index] != wantTitles[index] {
			t.Fatalf("cards[%d] = %q, want %q", index, gotTitles[index], wantTitles[index])
		}
	}
}

func TestBuildFocusTaskCardsShowsDDLOnlyBetweenCreatedDayAndDeadlineDay(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 3, 16, 10, 15, 0, 0, location)
	createdAt := time.Date(2026, 3, 16, 8, 0, 0, 0, location)
	deadline := time.Date(2026, 3, 18, 20, 0, 0, 0, location)

	dashboard := repository.Dashboard{
		DDL: []domain.Task{
			{
				ID:         uuid.New(),
				Title:      "窗口内可见",
				Type:       domain.TaskTypeDDL,
				Importance: 3,
				Deadline:   &deadline,
				CreatedAt:  createdAt,
			},
		},
	}

	beforeCreate := buildFocusTaskCards(dashboard, now, time.Date(2026, 3, 15, 0, 0, 0, 0, location), location)
	if len(beforeCreate) != 0 {
		t.Fatalf("before create len = %d, want 0", len(beforeCreate))
	}

	onCreate := buildFocusTaskCards(dashboard, now, time.Date(2026, 3, 16, 0, 0, 0, 0, location), location)
	if len(onCreate) != 1 {
		t.Fatalf("on create len = %d, want 1", len(onCreate))
	}

	onDeadline := buildFocusTaskCards(dashboard, now, time.Date(2026, 3, 18, 0, 0, 0, 0, location), location)
	if len(onDeadline) != 1 {
		t.Fatalf("on deadline len = %d, want 1", len(onDeadline))
	}

	afterDeadline := buildFocusTaskCards(dashboard, now, time.Date(2026, 3, 19, 0, 0, 0, 0, location), location)
	if len(afterDeadline) != 0 {
		t.Fatalf("after deadline len = %d, want 0", len(afterDeadline))
	}
}

func TestBuildFocusTaskCardsFallsBackToTitleOrder(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 3, 16, 10, 15, 0, 0, location)
	focusDate := time.Date(2026, 3, 16, 0, 0, 0, 0, location)
	createdAt := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)

	dashboard := repository.Dashboard{
		Todo: []domain.Task{
			{
				ID:         uuid.New(),
				Title:      "9号柜 608340",
				Type:       domain.TaskTypeTodo,
				Importance: 2,
				CreatedAt:  createdAt.Add(4 * time.Minute),
			},
			{
				ID:         uuid.New(),
				Title:      "1号柜 813835",
				Type:       domain.TaskTypeTodo,
				Importance: 2,
				CreatedAt:  createdAt.Add(3 * time.Minute),
			},
			{
				ID:         uuid.New(),
				Title:      "驿站：A-11-4608",
				Type:       domain.TaskTypeTodo,
				Importance: 2,
				CreatedAt:  createdAt.Add(2 * time.Minute),
			},
		},
	}

	cards := buildFocusTaskCards(dashboard, now, focusDate, location)
	if len(cards) != 3 {
		t.Fatalf("len(cards) = %d, want 3", len(cards))
	}

	gotTitles := []string{cards[0].Title, cards[1].Title, cards[2].Title}
	wantTitles := []string{"1号柜 813835", "9号柜 608340", "驿站：A-11-4608"}
	for index := range wantTitles {
		if gotTitles[index] != wantTitles[index] {
			t.Fatalf("cards[%d] = %q, want %q", index, gotTitles[index], wantTitles[index])
		}
	}
}

func TestFormatDDLCountdownSwitchesFromDaysToHoursToMinutes(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	deadline := time.Date(2026, 3, 1, 20, 0, 0, 0, location)

	if got := formatDDLCountdown(deadline, time.Date(2026, 2, 28, 9, 0, 0, 0, location), time.Date(2026, 2, 28, 9, 0, 0, 0, location), location); got != "还有 1 天" {
		t.Fatalf("day countdown = %q, want %q", got, "还有 1 天")
	}

	if got := formatDDLCountdown(deadline, time.Date(2026, 3, 1, 0, 30, 0, 0, location), time.Date(2026, 3, 1, 0, 0, 0, 0, location), location); got != "还有 20 小时" {
		t.Fatalf("hour countdown = %q, want %q", got, "还有 20 小时")
	}

	if got := formatDDLCountdown(deadline, time.Date(2026, 3, 1, 19, 21, 0, 0, location), time.Date(2026, 3, 1, 0, 0, 0, 0, location), location); got != "还有 39 分钟" {
		t.Fatalf("minute countdown = %q, want %q", got, "还有 39 分钟")
	}
}

func TestFormatDDLCountdownUsesFocusDateForNonTodayViews(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 3, 16, 10, 0, 0, 0, location)
	deadline := time.Date(2026, 3, 21, 20, 0, 0, 0, location)

	if got := formatDDLCountdown(deadline, now, time.Date(2026, 3, 18, 0, 0, 0, 0, location), location); got != "还有 3 天" {
		t.Fatalf("focused future countdown = %q, want %q", got, "还有 3 天")
	}

	if got := formatDDLCountdown(deadline, now, time.Date(2026, 3, 21, 0, 0, 0, 0, location), location); got != "今天" {
		t.Fatalf("focused same-day countdown = %q, want %q", got, "今天")
	}
}

func TestNormalizeDateForViewUsesFourAMBoundary(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)

	beforeBoundary := time.Date(2026, 4, 27, 1, 30, 0, 0, location)
	if got := normalizeDateForView(beforeBoundary, location); !got.Equal(time.Date(2026, 4, 26, 0, 0, 0, 0, location)) {
		t.Fatalf("before boundary normalized = %s, want 2026-04-26", got.Format("2006-01-02"))
	}

	afterBoundary := time.Date(2026, 4, 27, 4, 0, 0, 0, location)
	if got := normalizeDateForView(afterBoundary, location); !got.Equal(time.Date(2026, 4, 27, 0, 0, 0, 0, location)) {
		t.Fatalf("after boundary normalized = %s, want 2026-04-27", got.Format("2006-01-02"))
	}
}

func TestFormatDDLCountdownKeepsCalendarDayLogicBeforeFourAM(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 4, 27, 1, 0, 0, 0, location)
	deadline := time.Date(2026, 4, 26, 20, 0, 0, 0, location)
	focusDate := time.Date(2026, 4, 26, 0, 0, 0, 0, location)

	if got := formatDDLCountdown(deadline, now, focusDate, location); got != "今天" {
		t.Fatalf("countdown before 4am = %q, want %q", got, "今天")
	}
}

func TestFormatDDLCountdownUsesPreviousDisplayDayForPreFourAMDeadline(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 4, 27, 1, 0, 0, 0, location)
	deadline := time.Date(2026, 4, 27, 2, 0, 0, 0, location)
	focusDate := time.Date(2026, 4, 26, 0, 0, 0, 0, location)

	if got := formatDDLCountdown(deadline, now, focusDate, location); got != "还有 1 小时" {
		t.Fatalf("pre-4am deadline countdown = %q, want %q", got, "还有 1 小时")
	}

	task := domain.Task{
		ID:         uuid.New(),
		Title:      "凌晨前截止",
		Type:       domain.TaskTypeDDL,
		Importance: 3,
		Deadline:   &deadline,
		CreatedAt:  time.Date(2026, 4, 26, 10, 0, 0, 0, location),
	}
	if !shouldDisplayDDLOnFocusDate(task, focusDate, location) {
		t.Fatal("expected pre-4am deadline to display on previous day")
	}
}

func TestFormatCompletedAtAlwaysIncludesTime(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	completedAt := time.Date(2026, 3, 18, 14, 37, 0, 0, location)

	todo := domain.Task{
		Type:        domain.TaskTypeTodo,
		CompletedAt: &completedAt,
	}
	if got := formatCompletedAt(todo, location); got != "完成于 3月18日 14:37" {
		t.Fatalf("todo completed line = %q, want %q", got, "完成于 3月18日 14:37")
	}

	schedule := domain.Task{
		Type:        domain.TaskTypeSchedule,
		CompletedAt: &completedAt,
	}
	if got := formatCompletedAt(schedule, location); got != "完成于 3月18日 14:37" {
		t.Fatalf("schedule completed line = %q, want %q", got, "完成于 3月18日 14:37")
	}

	ddl := domain.Task{
		Type:        domain.TaskTypeDDL,
		CompletedAt: &completedAt,
	}
	if got := formatCompletedAt(ddl, location); got != "完成于 3月18日 14:37" {
		t.Fatalf("ddl completed line = %q, want %q", got, "完成于 3月18日 14:37")
	}
}

func TestParseManualTaskFormExpandsScheduleBatchInclusively(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	handler := &Handler{location: location}

	form := url.Values{
		"task_type":         {"schedule"},
		"title":             {"固定组会"},
		"importance":        {"2"},
		"schedule_mode":     {"batch"},
		"batch_start_value": {"2026-03-16"},
		"batch_end_value":   {"2026-03-22"},
		"batch_weekdays":    {"mon", "wed", "sun"},
	}

	request := httptest.NewRequest("POST", "/tasks/manual", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := request.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}

	inputs, err := handler.parseManualTaskForm(request)
	if err != nil {
		t.Fatalf("parseManualTaskForm() error = %v", err)
	}
	if len(inputs) != 3 {
		t.Fatalf("len(inputs) = %d, want 3", len(inputs))
	}

	wantDates := []time.Time{
		time.Date(2026, 3, 16, 0, 0, 0, 0, location),
		time.Date(2026, 3, 18, 0, 0, 0, 0, location),
		time.Date(2026, 3, 22, 0, 0, 0, 0, location),
	}
	for index, want := range wantDates {
		if inputs[index].ScheduledFor == nil {
			t.Fatalf("inputs[%d].ScheduledFor is nil", index)
		}
		if !inputs[index].ScheduledFor.Equal(want) {
			t.Fatalf(
				"inputs[%d].ScheduledFor = %s, want %s",
				index,
				inputs[index].ScheduledFor.Format("2006-01-02"),
				want.Format("2006-01-02"),
			)
		}
		if inputs[index].Title != "固定组会" {
			t.Fatalf("inputs[%d].Title = %q, want %q", index, inputs[index].Title, "固定组会")
		}
	}
}

func TestParseRequestFormSupportsMultipartFields(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	handler := &Handler{location: location}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("sms_input", "【菜鸟驿站】您的包裹已到站，凭140-1-3005到重庆双福状元路41号店取件。"); err != nil {
		t.Fatalf("WriteField() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	request := httptest.NewRequest("POST", "/tasks/parse-sms", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	if err := handler.parseRequestForm(request); err != nil {
		t.Fatalf("parseRequestForm() error = %v", err)
	}

	if got := request.FormValue("sms_input"); got == "" {
		t.Fatalf("FormValue(sms_input) = empty, want non-empty")
	}
}
