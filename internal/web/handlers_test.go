package web

import (
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
