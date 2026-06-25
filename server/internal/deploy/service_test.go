package deploy

import "testing"

func TestSniffSystemMeta_ValidSimple(t *testing.T) {
	src := `export default {
    name: "monitor",
    namespace: "alice",
    nodes: {
        timer: { uses: "quark/time/schedule/timer:v1", interval: "1s" }
    }
};`
	meta, err := sniffSystemMeta(src)
	if err != nil {
		t.Fatalf("sniffSystemMeta error: %v", err)
	}
	if meta.Name != "monitor" {
		t.Errorf("Name = %q, want monitor", meta.Name)
	}
	if meta.Namespace != "alice" {
		t.Errorf("Namespace = %q, want alice", meta.Namespace)
	}
	if meta.IsIsolated {
		t.Errorf("IsIsolated = true, want false (default shared)")
	}
}

func TestSniffSystemMeta_IsolatedRuntime(t *testing.T) {
	src := `export default {
    name: "monitor",
    namespace: "alice",
    runtime: "isolated",
    nodes: {}
};`
	meta, err := sniffSystemMeta(src)
	if err != nil {
		t.Fatalf("sniffSystemMeta error: %v", err)
	}
	if !meta.IsIsolated {
		t.Errorf("IsIsolated = false, want true")
	}
}

func TestSniffSystemMeta_SharedRuntime(t *testing.T) {
	src := `export default {
    name: "monitor",
    namespace: "alice",
    runtime: "shared",
    nodes: {}
};`
	meta, err := sniffSystemMeta(src)
	if err != nil {
		t.Fatalf("sniffSystemMeta error: %v", err)
	}
	if meta.IsIsolated {
		t.Errorf("IsIsolated = true, want false (explicit shared)")
	}
}

func TestSniffSystemMeta_EmptySource(t *testing.T) {
	_, err := sniffSystemMeta("")
	if err == nil {
		t.Error("sniffSystemMeta(\"\") should return error")
	}
}

func TestSniffSystemMeta_MissingName(t *testing.T) {
	src := `export default {
    namespace: "alice",
    nodes: {}
};`
	_, err := sniffSystemMeta(src)
	if err == nil {
		t.Error("sniffSystemMeta with missing name should return error")
	}
}

func TestSniffSystemMeta_SingleQuotedStrings(t *testing.T) {
	src := `export default {
    name: 'monitor',
    namespace: 'alice',
    nodes: {}
};`
	meta, err := sniffSystemMeta(src)
	if err != nil {
		t.Fatalf("sniffSystemMeta error: %v", err)
	}
	if meta.Name != "monitor" {
		t.Errorf("Name = %q, want monitor", meta.Name)
	}
	if meta.Namespace != "alice" {
		t.Errorf("Namespace = %q, want alice", meta.Namespace)
	}
}

func TestSniffSystemMeta_RealExample(t *testing.T) {
	// Verifies the sniffer works on the actual simple-streaming example
	src := `/**
 * Simple Streaming Monitor — Multi-Tenant Example
 */
export default {
    name: "monitor",
    namespace: "alice",

    nodes: {
        // A 1-second timer that publishes "tick" events.
        timer: {
            uses: "quark/time/schedule/timer:v1",
            interval: "1s",
            events: ["tick"],
        },
        cpu: {
            uses: "quark/system/cpu/profile:v1",
            timeout: "200ms",
            listens: ["timer.tick"],
            events: ["data"],
            onFailure: { retry: 3, routeTo: "writer" },
        },
    },
};`
	meta, err := sniffSystemMeta(src)
	if err != nil {
		t.Fatalf("sniffSystemMeta error: %v", err)
	}
	if meta.Name != "monitor" {
		t.Errorf("Name = %q, want monitor", meta.Name)
	}
	if meta.Namespace != "alice" {
		t.Errorf("Namespace = %q, want alice", meta.Namespace)
	}
	if meta.IsIsolated {
		t.Errorf("IsIsolated = true, want false (no runtime field)")
	}
}
