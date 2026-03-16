package service

import (
	"testing"
	"time"

	"todo/internal/domain"
)

func TestTextParserParse(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	parser := NewTextParser(location)
	now := time.Date(2026, 3, 16, 9, 0, 0, 0, location)

	tests := []struct {
		name          string
		input         string
		wantType      domain.TaskType
		wantTitle     string
		wantNote      string
		wantDate      string
		wantClock     string
		wantSource    domain.SourceType
		checkDeadline bool
	}{
		{
			name:       "persistent todo",
			input:      "买电池",
			wantType:   domain.TaskTypeTodo,
			wantTitle:  "买电池",
			wantSource: domain.SourceTypeManualText,
		},
		{
			name:       "schedule with relative date",
			input:      "明天上课",
			wantType:   domain.TaskTypeSchedule,
			wantTitle:  "上课",
			wantDate:   "2026-03-17",
			wantSource: domain.SourceTypeManualText,
		},
		{
			name:          "ddl with weekday",
			input:         "周五交作业",
			wantType:      domain.TaskTypeDDL,
			wantTitle:     "交作业",
			wantDate:      "2026-03-20",
			wantClock:     "23:59",
			wantSource:    domain.SourceTypeManualText,
			checkDeadline: true,
		},
		{
			name:          "ddl with explicit clock",
			input:         "3月20日20:00交报告",
			wantType:      domain.TaskTypeDDL,
			wantTitle:     "交报告",
			wantDate:      "2026-03-20",
			wantClock:     "20:00",
			wantSource:    domain.SourceTypeManualText,
			checkDeadline: true,
		},
		{
			name:       "pickup sms",
			input:      "【菜鸟驿站】取件码 384923",
			wantType:   domain.TaskTypeTodo,
			wantTitle:  "取快递",
			wantNote:   "取件码 384923",
			wantSource: domain.SourceTypeSMSPaste,
		},
		{
			name:       "day period schedule",
			input:      "下午签到",
			wantType:   domain.TaskTypeSchedule,
			wantTitle:  "签到",
			wantDate:   "2026-03-16",
			wantSource: domain.SourceTypeManualText,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Parse(tt.input, now)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if got.SourceType != tt.wantSource {
				t.Fatalf("SourceType = %s, want %s", got.SourceType, tt.wantSource)
			}
			if got.Task.Type != tt.wantType {
				t.Fatalf("Task.Type = %s, want %s", got.Task.Type, tt.wantType)
			}
			if got.Task.Title != tt.wantTitle {
				t.Fatalf("Task.Title = %q, want %q", got.Task.Title, tt.wantTitle)
			}
			if got.Task.Note != tt.wantNote {
				t.Fatalf("Task.Note = %q, want %q", got.Task.Note, tt.wantNote)
			}
			if got.Task.Importance != domain.DefaultTaskImportance {
				t.Fatalf("Task.Importance = %d, want %d", got.Task.Importance, domain.DefaultTaskImportance)
			}

			var dateValue *time.Time
			if tt.checkDeadline {
				dateValue = got.Task.Deadline
			} else {
				dateValue = got.Task.ScheduledFor
			}

			if tt.wantDate == "" {
				if dateValue != nil {
					t.Fatalf("expected no parsed date, got %s", dateValue.Format("2006-01-02"))
				}
				return
			}

			if dateValue == nil {
				t.Fatalf("expected parsed date %s, got nil", tt.wantDate)
			}
			if dateValue.Format("2006-01-02") != tt.wantDate {
				t.Fatalf("parsed date = %s, want %s", dateValue.Format("2006-01-02"), tt.wantDate)
			}
			if tt.wantClock != "" && dateValue.In(location).Format("15:04") != tt.wantClock {
				t.Fatalf("parsed time = %s, want %s", dateValue.In(location).Format("15:04"), tt.wantClock)
			}
		})
	}
}
