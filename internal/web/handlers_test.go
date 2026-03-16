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
	focusDate := time.Date(2026, 3, 16, 9, 0, 0, 0, location)
	createdAt := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)

	scheduleToday := time.Date(2026, 3, 16, 0, 0, 0, 0, location)
	ddlToday := time.Date(2026, 3, 16, 0, 0, 0, 0, location)
	ddlTomorrow := time.Date(2026, 3, 17, 0, 0, 0, 0, location)

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
				Title:      "今天截止",
				Type:       domain.TaskTypeDDL,
				Importance: 5,
				Deadline:   &ddlToday,
				CreatedAt:  createdAt.Add(2 * time.Minute),
			},
			{
				ID:         uuid.New(),
				Title:      "明天截止",
				Type:       domain.TaskTypeDDL,
				Importance: 5,
				Deadline:   &ddlTomorrow,
				CreatedAt:  createdAt.Add(3 * time.Minute),
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

	cards := buildFocusTaskCards(dashboard, focusDate, location)
	if len(cards) != 4 {
		t.Fatalf("len(cards) = %d, want 4", len(cards))
	}

	gotTitles := []string{cards[0].Title, cards[1].Title, cards[2].Title, cards[3].Title}
	wantTitles := []string{"今天截止", "明天截止", "高优先待办", "普通日程"}
	for index := range wantTitles {
		if gotTitles[index] != wantTitles[index] {
			t.Fatalf("cards[%d] = %q, want %q", index, gotTitles[index], wantTitles[index])
		}
	}
}
