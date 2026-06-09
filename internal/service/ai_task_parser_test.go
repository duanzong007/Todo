package service

import (
	"testing"
	"time"

	"todo/internal/domain"
)

func TestNormalizeAITaskEndpoint(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "base api url",
			raw:  "https://chat.cqjtu.edu.cn/ds/api/v1",
			want: "https://chat.cqjtu.edu.cn/ds/api/v1/chat/completions",
		},
		{
			name: "full chat completions url",
			raw:  "https://chat.cqjtu.edu.cn/ds/api/v1/chat/completions",
			want: "https://chat.cqjtu.edu.cn/ds/api/v1/chat/completions",
		},
		{
			name: "root url",
			raw:  "https://example.com",
			want: "https://example.com/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeAITaskEndpoint(tt.raw)
			if err != nil {
				t.Fatalf("normalizeAITaskEndpoint() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeAITaskEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAITaskParserParseContent(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	parser := &AITaskParser{location: location}

	prefill, err := parser.parseContent(`{"type":"ddl","title":"交数据库作业","importance":4,"scheduled_for":null,"deadline":"2026-06-12T23:59","note":""}`)
	if err != nil {
		t.Fatalf("parseContent() error = %v", err)
	}
	task := prefill.Task
	if task.Type != domain.TaskTypeDDL {
		t.Fatalf("task.Type = %q, want ddl", task.Type)
	}
	if task.Title != "交数据库作业" {
		t.Fatalf("task.Title = %q", task.Title)
	}
	if task.Importance != 4 {
		t.Fatalf("task.Importance = %d, want 4", task.Importance)
	}
	if task.Deadline == nil || task.Deadline.Format("2006-01-02T15:04") != "2026-06-12T23:59" {
		t.Fatalf("task.Deadline = %v", task.Deadline)
	}
}

func TestAITaskParserParseContentScheduleBatch(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	parser := &AITaskParser{location: location}

	prefill, err := parser.parseContent(`{
		"type":"schedule",
		"title":"背单词",
		"importance":2,
		"schedule_mode":"batch",
		"scheduled_for":null,
		"batch_start":"2026-06-15",
		"batch_end":"2026-06-21",
		"batch_weekdays":["mon","tue","wed","thu","fri","sat","sun"],
		"deadline":null,
		"note":""
	}`)
	if err != nil {
		t.Fatalf("parseContent() error = %v", err)
	}
	if prefill.Task.Type != domain.TaskTypeSchedule {
		t.Fatalf("Task.Type = %q, want schedule", prefill.Task.Type)
	}
	if prefill.ScheduleMode != "batch" {
		t.Fatalf("ScheduleMode = %q, want batch", prefill.ScheduleMode)
	}
	if prefill.BatchStart == nil || prefill.BatchStart.Format("2006-01-02") != "2026-06-15" {
		t.Fatalf("BatchStart = %v", prefill.BatchStart)
	}
	if prefill.BatchEnd == nil || prefill.BatchEnd.Format("2006-01-02") != "2026-06-21" {
		t.Fatalf("BatchEnd = %v", prefill.BatchEnd)
	}
	if got, want := len(prefill.BatchWeekdays), 7; got != want {
		t.Fatalf("len(BatchWeekdays) = %d, want %d", got, want)
	}
}
