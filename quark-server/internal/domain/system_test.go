package domain

import "testing"

func TestNodeEvent_TimestampString_StringInput(t *testing.T) {
	ev := &NodeEvent{Timestamp: "2026-06-25T21:14:05.511421593Z"}
	got := ev.TimestampString()
	if got != "2026-06-25T21:14:05.511421593Z" {
		t.Errorf("got %q, want passthrough", got)
	}
}

func TestNodeEvent_TimestampString_EpochSeconds(t *testing.T) {
	// 1782000000 epoch-seconds = 2026-06-21T00:00:00Z (approx)
	ev := &NodeEvent{Timestamp: float64(1782000000)}
	got := ev.TimestampString()
	if got == "" {
		t.Error("got empty, want RFC3339 timestamp")
	}
	// Should be in 2026
	if len(got) < 4 || got[:4] != "2026" {
		t.Errorf("got %q, want 2026-... timestamp", got)
	}
}

func TestNodeEvent_TimestampString_EpochMillis(t *testing.T) {
	// 1782000000000 epoch-millis = 2026-06-21T00:00:00Z (approx)
	ev := &NodeEvent{Timestamp: float64(1782000000000)}
	got := ev.TimestampString()
	if got == "" {
		t.Error("got empty, want RFC3339 timestamp")
	}
	if len(got) < 4 || got[:4] != "2026" {
		t.Errorf("got %q, want 2026-... timestamp", got)
	}
}

func TestNodeEvent_TimestampString_IntSeconds(t *testing.T) {
	ev := &NodeEvent{Timestamp: int64(1782000000)}
	got := ev.TimestampString()
	if got == "" {
		t.Error("got empty, want RFC3339 timestamp")
	}
}

func TestNodeEvent_TimestampString_IntMillis(t *testing.T) {
	ev := &NodeEvent{Timestamp: int64(1782000000000)}
	got := ev.TimestampString()
	if got == "" {
		t.Error("got empty, want RFC3339 timestamp")
	}
}

func TestNodeEvent_TimestampString_InvalidType(t *testing.T) {
	ev := &NodeEvent{Timestamp: []string{"not", "a", "timestamp"}}
	got := ev.TimestampString()
	if got != "" {
		t.Errorf("got %q, want empty string for invalid type", got)
	}
}

func TestNow(t *testing.T) {
	got := Now()
	if got == "" {
		t.Error("Now() returned empty string")
	}
	// Should start with "20" (year 20XX)
	if len(got) < 4 || got[:2] != "20" {
		t.Errorf("Now() = %q, want 20XX-... RFC3339 timestamp", got)
	}
}
