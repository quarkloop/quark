package fs

import "testing"

func TestSchemaIncludesPDFExtraction(t *testing.T) {
	schema := (&Tool{}).Schema()
	props, ok := schema.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties missing: %#v", schema.Parameters)
	}
	command, ok := props["command"].(map[string]any)
	if !ok {
		t.Fatalf("command schema missing: %#v", props["command"])
	}
	enum, ok := command["enum"].([]string)
	if !ok {
		t.Fatalf("command enum missing: %#v", command["enum"])
	}
	for _, value := range enum {
		if value == "extract_pdf" {
			return
		}
	}
	t.Fatalf("command enum missing extract_pdf: %#v", enum)
}

func TestIntFlagAcceptsSchemaAlias(t *testing.T) {
	got, err := intFlag(map[string]any{"max_chars": float64(1200)}, "max-chars", defaultPDFMaxChars)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1200 {
		t.Fatalf("max chars = %d, want 1200", got)
	}
}
