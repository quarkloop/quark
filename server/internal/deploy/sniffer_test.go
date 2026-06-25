// Package deploy — additional edge-case tests for the source sniffer.
package deploy

import "testing"

func TestSniffSystemMeta_NameWithSpaces(t *testing.T) {
        // Names with spaces are syntactically valid JS string literals
        src := `export default {
    name: "my system",
    namespace: "alice",
    nodes: {}
};`
        meta, err := sniffSystemMeta(src)
        if err != nil {
                t.Fatalf("sniffSystemMeta error: %v", err)
        }
        if meta.Name != "my system" {
                t.Errorf("Name = %q, want 'my system'", meta.Name)
        }
}

func TestSniffSystemMeta_QuarkTsComment(t *testing.T) {
        // Comments should not interfere with the sniffer
        src := `// This is a comment with name: "fake" and runtime: "isolated"
export default {
    name: "real",
    namespace: "alice",
    runtime: "shared", // inline comment
    nodes: {}
};`
        meta, err := sniffSystemMeta(src)
        if err != nil {
                t.Fatalf("sniffSystemMeta error: %v", err)
        }
        if meta.Name != "real" {
                t.Errorf("Name = %q, want 'real' (comment should be ignored)", meta.Name)
        }
        if meta.IsIsolated {
                t.Errorf("IsIsolated = true, want false (comment should be ignored)")
        }
}

func TestSniffSystemMeta_OnlyName(t *testing.T) {
        // Minimum required field is `name`. The sniffer expects `name:` to
        // appear at the start of a line (with optional leading whitespace),
        // matching the multi-line .quark.ts convention. A one-liner like
        // `export default { name: "x" };` is NOT supported — that's a
        // known limitation, documented here.
        src := `export default {
    name: "lonely"
};`
        meta, err := sniffSystemMeta(src)
        if err != nil {
                t.Fatalf("sniffSystemMeta error: %v", err)
        }
        if meta.Name != "lonely" {
                t.Errorf("Name = %q, want 'lonely'", meta.Name)
        }
        if meta.Namespace != "" {
                t.Errorf("Namespace = %q, want empty", meta.Namespace)
        }
}

func TestSniffSystemMeta_RuntimeWithMixedCase(t *testing.T) {
        // runtime: "Isolated" should NOT match (case-sensitive comparison)
        src := `export default {
    name: "monitor",
    namespace: "alice",
    runtime: "Isolated",
    nodes: {}
};`
        meta, err := sniffSystemMeta(src)
        if err != nil {
                t.Fatalf("sniffSystemMeta error: %v", err)
        }
        if meta.IsIsolated {
                t.Errorf("IsIsolated = true, want false (case-sensitive: 'Isolated' != 'isolated')")
        }
}

func TestSniffSystemMeta_NestedObjectWithName(t *testing.T) {
        // A nested object that also has a `name:` field should NOT confuse
        // the sniffer (the regex is anchored to start-of-line via ^\s*).
        src := `export default {
    name: "outer",
    namespace: "alice",
    nodes: {
        timer: {
            name: "inner",
            uses: "quark/time/schedule/timer:v1"
        }
    }
};`
        meta, err := sniffSystemMeta(src)
        if err != nil {
                t.Fatalf("sniffSystemMeta error: %v", err)
        }
        if meta.Name != "outer" {
                t.Errorf("Name = %q, want 'outer' (should pick the top-level name)", meta.Name)
        }
}
