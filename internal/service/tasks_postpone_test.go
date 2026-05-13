package service

import (
	"testing"
	"time"

	"todo/internal/domain"
)

func TestEarliestSchedulePostponeDateUsesOriginalScheduleDate(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 3, 16, 10, 30, 0, 0, location)
	scheduled := time.Date(2026, 3, 20, 0, 0, 0, 0, location)

	got := earliestSchedulePostponeDate(domain.Task{
		Type:         domain.TaskTypeSchedule,
		ScheduledFor: &scheduled,
	}, now, location)

	want := time.Date(2026, 3, 21, 0, 0, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestEarliestSchedulePostponeDateIgnoresCurrentDate(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 6, 30, 10, 30, 0, 0, location)
	scheduled := time.Date(2026, 6, 24, 0, 0, 0, 0, location)

	got := earliestSchedulePostponeDate(domain.Task{
		Type:         domain.TaskTypeSchedule,
		ScheduledFor: &scheduled,
	}, now, location)

	want := time.Date(2026, 6, 25, 0, 0, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestEarliestDDLPostponeTimeRoundsOriginalDeadlineUpToNextMinute(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 3, 16, 10, 30, 45, 0, location)
	deadline := time.Date(2026, 3, 16, 12, 5, 0, 0, location)

	got := earliestDDLPostponeTime(domain.Task{
		Type:     domain.TaskTypeDDL,
		Deadline: &deadline,
	}, now, location)

	want := time.Date(2026, 3, 16, 12, 6, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestEarliestDDLPostponeTimeIgnoresCurrentTime(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 6, 25, 10, 30, 45, 0, location)
	deadline := time.Date(2026, 6, 24, 18, 0, 0, 0, location)

	got := earliestDDLPostponeTime(domain.Task{
		Type:     domain.TaskTypeDDL,
		Deadline: &deadline,
	}, now, location)

	want := time.Date(2026, 6, 24, 18, 1, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestParsePostponeTargetRejectsEarlierScheduleDate(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 3, 16, 9, 0, 0, 0, location)
	scheduled := time.Date(2026, 3, 16, 0, 0, 0, 0, location)

	_, err := parsePostponeTarget(domain.Task{
		Type:         domain.TaskTypeSchedule,
		ScheduledFor: &scheduled,
	}, "2026-03-16", now, location)
	if err == nil {
		t.Fatalf("expected error for non-later schedule date")
	}
}

func TestParsePostponeTargetAcceptsLaterDDLMinute(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 3, 16, 9, 0, 0, 0, location)
	deadline := time.Date(2026, 3, 16, 18, 20, 0, 0, location)

	got, err := parsePostponeTarget(domain.Task{
		Type:     domain.TaskTypeDDL,
		Deadline: &deadline,
	}, "2026-03-16T18:21", now, location)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := time.Date(2026, 3, 16, 18, 21, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestParsePostponeTargetAcceptsDDLTargetBeforeNowWhenAfterOriginalDeadline(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 6, 25, 10, 30, 0, 0, location)
	deadline := time.Date(2026, 6, 24, 18, 20, 0, 0, location)

	got, err := parsePostponeTarget(domain.Task{
		Type:     domain.TaskTypeDDL,
		Deadline: &deadline,
	}, "2026-06-24T18:21", now, location)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := time.Date(2026, 6, 24, 18, 21, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
