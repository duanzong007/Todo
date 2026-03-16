package service

import (
	"testing"
	"time"
)

func TestICSImporterParseRecurringWeekly(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	importer := NewICSImporter(location, 30)
	now := time.Date(2026, 3, 16, 8, 0, 0, 0, location)

	body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:course-1\r\nSUMMARY:高数\r\nDTSTART;TZID=Asia/Shanghai:20260316T090000\r\nDTEND;TZID=Asia/Shanghai:20260316T103000\r\nRRULE:FREQ=WEEKLY;COUNT=4;BYDAY=MO,WE\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")

	result, err := importer.Parse("course.ics", body, now)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(result.Tasks) != 4 {
		t.Fatalf("len(result.Tasks) = %d, want 4", len(result.Tasks))
	}

	wantDates := []string{"2026-03-16", "2026-03-18", "2026-03-23", "2026-03-25"}
	for index, task := range result.Tasks {
		if task.Title != "高数" {
			t.Fatalf("task title = %q, want 高数", task.Title)
		}
		if task.Note != "09:00 - 10:30" {
			t.Fatalf("task note = %q, want 09:00 - 10:30", task.Note)
		}
		if task.ScheduledFor == nil {
			t.Fatalf("task %d scheduled date is nil", index)
		}
		if task.ScheduledFor.Format("2006-01-02") != wantDates[index] {
			t.Fatalf("task %d date = %s, want %s", index, task.ScheduledFor.Format("2006-01-02"), wantDates[index])
		}
	}
}
