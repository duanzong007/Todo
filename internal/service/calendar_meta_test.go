package service

import (
	"testing"
	"time"
)

func TestCalendarMetaForDateWeekday(t *testing.T) {
	date := time.Date(2026, time.March, 16, 12, 0, 0, 0, time.UTC)

	meta := CalendarMetaForDate(date, time.UTC)

	if meta.WeekdayLabel != "星期一" {
		t.Fatalf("expected 星期一, got %s", meta.WeekdayLabel)
	}
}

func TestCalendarMetaForDateHolidayDedup(t *testing.T) {
	date := time.Date(2019, time.May, 1, 12, 0, 0, 0, time.UTC)

	meta := CalendarMetaForDate(date, time.UTC)

	if countTag(meta.Tags, "劳动节") != 1 {
		t.Fatalf("expected exactly one 劳动节 tag, got %v", meta.Tags)
	}
}

func TestCalendarMetaForDateSolarTerm(t *testing.T) {
	date := time.Date(2021, time.December, 21, 12, 0, 0, 0, time.UTC)

	meta := CalendarMetaForDate(date, time.UTC)

	if !hasTag(meta.Tags, "冬至") {
		t.Fatalf("expected 冬至 tag, got %v", meta.Tags)
	}
}

func TestCalendarMetaForDateLunarFestival(t *testing.T) {
	date := time.Date(2022, time.January, 31, 12, 0, 0, 0, time.UTC)

	meta := CalendarMetaForDate(date, time.UTC)

	if !hasTag(meta.Tags, "除夕") {
		t.Fatalf("expected 除夕 tag, got %v", meta.Tags)
	}
}

func hasTag(tags []string, target string) bool {
	return countTag(tags, target) > 0
}

func countTag(tags []string, target string) int {
	count := 0
	for _, tag := range tags {
		if tag == target {
			count++
		}
	}
	return count
}
