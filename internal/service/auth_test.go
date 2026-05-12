package service

import "testing"

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{name: "normalize", input: "  Alice.Test  ", want: "alice.test"},
		{name: "too short", input: "ab", wantError: true},
		{name: "invalid chars", input: "alice test", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateUsername(tt.input)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("validateUsername() = %q, want %q", got, tt.want)
			}
		})
	}
}
