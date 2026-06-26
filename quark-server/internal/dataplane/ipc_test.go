package dataplane

import "testing"

func TestRuntimeID(t *testing.T) {
	tests := []struct {
		namespace  string
		isIsolated bool
		want       string
	}{
		{"alice", false, "shared"},
		{"alice", true, "ns-alice"},
		{"bob", false, "shared"},
		{"bob", true, "ns-bob"},
		{"", false, "shared"},
		{"", true, "ns-"},
	}
	for _, tt := range tests {
		got := RuntimeID(tt.namespace, tt.isIsolated)
		if got != tt.want {
			t.Errorf("RuntimeID(%q, %v) = %q, want %q",
				tt.namespace, tt.isIsolated, got, tt.want)
		}
	}
}

func TestDeploySubject(t *testing.T) {
	tests := []struct {
		runtimeId string
		want      string
	}{
		{"shared", "quark.control.shared.deploy"},
		{"ns-alice", "quark.control.ns-alice.deploy"},
	}
	for _, tt := range tests {
		got := DeploySubject(tt.runtimeId)
		if got != tt.want {
			t.Errorf("DeploySubject(%q) = %q, want %q",
				tt.runtimeId, got, tt.want)
		}
	}
}

func TestUndeploySubject(t *testing.T) {
	got := UndeploySubject("shared")
	want := "quark.control.shared.undeploy"
	if got != want {
		t.Errorf("UndeploySubject(%q) = %q, want %q", "shared", got, want)
	}
}

func TestEventSubject(t *testing.T) {
	got := EventSubject("shared")
	want := "quark.data.event.shared"
	if got != want {
		t.Errorf("EventSubject(%q) = %q, want %q", "shared", got, want)
	}
}

func TestHeartbeatSubject(t *testing.T) {
	got := HeartbeatSubject("shared")
	want := "quark.data.heartbeat.shared"
	if got != want {
		t.Errorf("HeartbeatSubject(%q) = %q, want %q", "shared", got, want)
	}
}

func TestSubjectConstants(t *testing.T) {
	if EventWildcard != "quark.data.event.>" {
		t.Errorf("EventWildcard = %q, want quark.data.event.>", EventWildcard)
	}
	if HeartbeatWildcard != "quark.data.heartbeat.>" {
		t.Errorf("HeartbeatWildcard = %q, want quark.data.heartbeat.>", HeartbeatWildcard)
	}
	if SharedRuntimeID != "shared" {
		t.Errorf("SharedRuntimeID = %q, want shared", SharedRuntimeID)
	}
}
