package statistics

import (
	"testing"
	"time"
)

func TestClipSession(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		start, end time.Time
		since      time.Time
		want       time.Duration
	}{
		{"inside window", base, base.Add(2 * time.Hour), base.Add(-time.Hour), 2 * time.Hour},
		{"starts before window", base, base.Add(2 * time.Hour), base.Add(time.Hour), time.Hour},
		{"ends before window", base, base.Add(time.Hour), base.Add(2 * time.Hour), 0},
		{"capped at max session", base, base.Add(30 * time.Hour), time.Time{}, maxSession},
		{"all time", base, base.Add(90 * time.Minute), time.Time{}, 90 * time.Minute},
	}
	for _, tt := range tests {
		if got := clipSession(tt.start, tt.end, tt.since); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	if got := formatDuration(42 * time.Minute); got != "42 min" {
		t.Errorf("got %q", got)
	}
	if got := formatDuration(12*time.Hour + 5*time.Minute); got != "12 h 05 min" {
		t.Errorf("got %q", got)
	}
}

func TestProgressBar(t *testing.T) {
	if got := progressBar(10, 10); got != "▰▰▰▰▰▰▰▰▰▰" {
		t.Errorf("got %q", got)
	}
	if got := progressBar(1, 10); got != "▰▱▱▱▱▱▱▱▱▱" {
		t.Errorf("got %q", got)
	}
}
