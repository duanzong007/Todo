package service

import (
	"strings"
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
			wantTitle:  "驿站：384923",
			wantNote:   "",
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
			wantImportance := domain.DefaultTaskImportance
			if tt.wantSource == domain.SourceTypeSMSPaste {
				wantImportance = 2
			}
			if got.Task.Importance != wantImportance {
				t.Fatalf("Task.Importance = %d, want %d", got.Task.Importance, wantImportance)
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

func TestTextParserParseSMSBatch(t *testing.T) {
	location := time.FixedZone("CST", 8*3600)
	parser := NewTextParser(location)
	now := time.Date(2026, 3, 16, 9, 0, 0, 0, location)

	input := strings.Join([]string{
		"【菜鸟驿站】您的包裹已到站，凭140-1-3005到重庆双福状元路41号店取件。",
		"【韵达超市】快递员提醒您，请凭取件码466412到建宇门口零食有鸣9号柜智能柜取您的快递",
		"【鸟箱】请凭取件码055151到零食有鸣5号柜鸟箱06副柜24格取件72小时内免费，之后每12小时收0.0元",
		"【熊猫柜】您的圆通包裹已到建宇新时区零食有鸣10号柜，取件码65676997",
		"【兔喜生活】您有包裹已到达零食有鸣3号柜兔喜快递柜，取件码为742605",
		"【申通快递】请凭A-88-2006到双福街道状元路41号取运单尾号8150包裹",
		"【多多代收点】请凭A-15-2096到双福街道状元路41号取运单尾号2507包裹",
		"【多多代收点】请凭A-33-3608到双福街道状元路41号取件，地址：双福街道状元路41号",
		"【圆通快递】请凭A-11-4608到双福街道状元路41号取件，地址：双福街道状元路41号",
		"【圆通快递】请凭A-22-2606到双福街道状元路41号取件，地址：双福街道状元路41号",
	}, "")

	got, err := parser.ParseSMSBatch(input, now)
	if err != nil {
		t.Fatalf("ParseSMSBatch() error = %v", err)
	}

	wantTitles := []string{
		"驿站：140-1-3005",
		"9号柜 466412",
		"5号柜 055151",
		"10号柜 65676997",
		"3号柜 742605",
		"驿站：A-88-2006",
		"驿站：A-15-2096",
		"驿站：A-33-3608",
		"驿站：A-11-4608",
		"驿站：A-22-2606",
	}
	if len(got) != len(wantTitles) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(wantTitles))
	}

	for index, parsed := range got {
		if parsed.SourceType != domain.SourceTypeSMSPaste {
			t.Fatalf("got[%d].SourceType = %s, want %s", index, parsed.SourceType, domain.SourceTypeSMSPaste)
		}
		if parsed.Task.Type != domain.TaskTypeTodo {
			t.Fatalf("got[%d].Task.Type = %s, want %s", index, parsed.Task.Type, domain.TaskTypeTodo)
		}
		if parsed.Task.Importance != 2 {
			t.Fatalf("got[%d].Task.Importance = %d, want 2", index, parsed.Task.Importance)
		}
		if parsed.Task.Title != wantTitles[index] {
			t.Fatalf("got[%d].Task.Title = %q, want %q", index, parsed.Task.Title, wantTitles[index])
		}
		if parsed.Task.Note != "" {
			t.Fatalf("got[%d].Task.Note = %q, want empty", index, parsed.Task.Note)
		}
	}
}
