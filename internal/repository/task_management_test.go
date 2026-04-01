package repository

import (
	"strings"
	"testing"
	"time"
)

func TestAppendTaskManagementDateBoundsUsesDateComparisons(t *testing.T) {
	from := time.Date(2026, time.April, 1, 9, 30, 0, 0, time.FixedZone("CST", 8*3600))
	to := time.Date(2026, time.April, 7, 21, 45, 0, 0, time.FixedZone("CST", 8*3600))

	args := []any{"user-id", "Asia/Shanghai"}
	var builder strings.Builder

	args = appendTaskManagementDateBounds(&builder, args, "((t.created_at AT TIME ZONE $2)::date)", &from, &to)

	if got := builder.String(); got != " AND ((t.created_at AT TIME ZONE $2)::date) >= $3::date AND ((t.created_at AT TIME ZONE $2)::date) <= $4::date" {
		t.Fatalf("unexpected where clause: %q", got)
	}

	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d", len(args))
	}

	if got, ok := args[2].(string); !ok || got != "2026-04-01" {
		t.Fatalf("unexpected from arg: %#v", args[2])
	}

	if got, ok := args[3].(string); !ok || got != "2026-04-07" {
		t.Fatalf("unexpected to arg: %#v", args[3])
	}
}
