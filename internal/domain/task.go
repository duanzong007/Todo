package domain

import (
	"time"

	"github.com/google/uuid"
)

type TaskType string

const (
	TaskTypeTodo     TaskType = "todo"
	TaskTypeSchedule TaskType = "schedule"
	TaskTypeDDL      TaskType = "ddl"
)

type TaskStatus string

const (
	TaskStatusActive TaskStatus = "active"
	TaskStatusDone   TaskStatus = "done"
)

type SourceType string

const (
	SourceTypeManualText SourceType = "manual_text"
	SourceTypeSMSPaste   SourceType = "sms_paste"
	SourceTypeICSImport  SourceType = "ics_import"
)

type Task struct {
	ID             uuid.UUID
	SourceID       *uuid.UUID
	Title          string
	Note           string
	Type           TaskType
	Status         TaskStatus
	ScheduledFor   *time.Time
	Deadline       *time.Time
	CompletedAt    *time.Time
	PostponedCount int
	Metadata       map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (t Task) SupportsCompletion() bool {
	return t.Type == TaskTypeTodo || t.Type == TaskTypeDDL
}

func (t Task) SupportsPostpone() bool {
	return t.Type == TaskTypeSchedule || t.Type == TaskTypeDDL
}
