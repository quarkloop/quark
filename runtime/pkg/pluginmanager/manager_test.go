package pluginmanager

import "testing"

func TestNormalizeToolResponseUnwrapsToolkitOutput(t *testing.T) {
	got := normalizeToolResponse([]byte(`{"data":{"output":"quark-ok\n","exit_code":0},"error":""}`))
	want := `{"output":"quark-ok\n","exit_code":0}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestNormalizeToolResponseKeepsCustomPayload(t *testing.T) {
	got := normalizeToolResponse([]byte(`{"results":[{"title":"ok"}]}`))
	want := `{"results":[{"title":"ok"}]}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestNormalizeToolResponseConvertsToolkitError(t *testing.T) {
	got := normalizeToolResponse([]byte(`{"data":null,"error":"nope"}`))
	want := `{"error":"nope","is_error":true}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
