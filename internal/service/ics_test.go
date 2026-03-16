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
		if task.Importance != 3 {
			t.Fatalf("task importance = %d, want 3", task.Importance)
		}
		if task.Note != "" {
			t.Fatalf("task note = %q, want empty", task.Note)
		}
		if task.ScheduledFor == nil {
			t.Fatalf("task %d scheduled date is nil", index)
		}
		if task.ScheduledFor.Format("2006-01-02") != wantDates[index] {
			t.Fatalf("task %d date = %s, want %s", index, task.ScheduledFor.Format("2006-01-02"), wantDates[index])
		}
	}
}

func TestICSImporterParseUsesSummaryOnlyForCourseSchedule(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	importer := NewICSImporter(location, 30)
	now := time.Date(2026, 3, 1, 8, 0, 0, 0, location)

	body := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Schedule Export//EN\r\nCALSCALE:GREGORIAN\r\nMETHOD:PUBLISH\r\nBEGIN:VEVENT\r\nUID:aabdf9dd5010774bbef6268f715bd140@schedule\r\nDTSTAMP:20260316T135614Z\r\nDTSTART:20260304T154000\r\nDTEND:20260304T170500\r\nSUMMARY:大学体育Ⅳ (太极拳)\r\nLOCATION:科学城校区运动场\r\nDESCRIPTION:老师：申存生\\n周次：1周\\n节次：8-9\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")

	result, err := importer.Parse("schedule.ics", body, now)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("len(result.Tasks) = %d, want 1", len(result.Tasks))
	}

	task := result.Tasks[0]
	if task.Type != "schedule" {
		t.Fatalf("task type = %q, want schedule", task.Type)
	}
	if task.Title != "大学体育Ⅳ (太极拳)" {
		t.Fatalf("task title = %q, want %q", task.Title, "大学体育Ⅳ (太极拳)")
	}
	if task.Note != "" {
		t.Fatalf("task note = %q, want empty", task.Note)
	}
	if task.ScheduledFor == nil {
		t.Fatalf("task scheduled date is nil")
	}
	if got := task.ScheduledFor.Format("2006-01-02"); got != "2026-03-04" {
		t.Fatalf("task date = %s, want 2026-03-04", got)
	}
}
