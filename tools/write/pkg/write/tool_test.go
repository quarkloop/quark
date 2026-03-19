package write

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyWriteReplaceAppend(t *testing.T) {
	target := filepath.Join(t.TempDir(), "notes.txt")

	writeRes, err := Apply(Request{
		Path:      target,
		Operation: "write",
		Content:   "alpha draft\n",
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !writeRes.Created || !writeRes.Changed {
		t.Fatalf("expected write to create and change file, got %+v", writeRes)
	}

	replaceRes, err := Apply(Request{
		Path:        target,
		Operation:   "replace",
		Find:        "draft",
		ReplaceWith: "final",
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if replaceRes.Replacements != 1 {
		t.Fatalf("expected one replacement, got %+v", replaceRes)
	}

	appendRes, err := Apply(Request{
		Path:      target,
		Operation: "append",
		Content:   "tail\n",
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if appendRes.Operation != "append" {
		t.Fatalf("expected append result, got %+v", appendRes)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	if got := string(data); got != "alpha final\ntail\n" {
		t.Fatalf("unexpected file content %q", got)
	}
	if !strings.Contains(appendRes.ContentPreview, "alpha final") {
		t.Fatalf("expected preview to include final content, got %+v", appendRes)
	}
}

func TestApplyEditUsesLineAndColumnRanges(t *testing.T) {
	target := filepath.Join(t.TempDir(), "app.py")
	original := "def greet(name):\n    return f\"Hello, {name}!\"\n\nif __name__ == \"__main__\":\n    print(greet(\"World\"))\n"
	expected := "def greet(name, punctuation=\"!\"):\n    return f\"Hello, {name}{punctuation}\"\n\nif __name__ == \"__main__\":\n    print(greet(\"Quark\", \"?\"))\n"

	if err := os.WriteFile(target, []byte(original), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	res, err := Apply(Request{
		Path:      target,
		Operation: "edit",
		Edits: []Edit{
			{
				StartLine:   1,
				StartColumn: 1,
				EndLine:     1,
				EndColumn:   lineEndColumn(t, original, 1),
				NewText:     "def greet(name, punctuation=\"!\"):",
			},
			{
				StartLine:   2,
				StartColumn: 1,
				EndLine:     2,
				EndColumn:   lineEndColumn(t, original, 2),
				NewText:     "    return f\"Hello, {name}{punctuation}\"",
			},
			{
				StartLine:   5,
				StartColumn: 1,
				EndLine:     5,
				EndColumn:   lineEndColumn(t, original, 5),
				NewText:     "    print(greet(\"Quark\", \"?\"))",
			},
		},
	})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if res.Operation != "edit" || res.EditsApplied != 3 {
		t.Fatalf("expected edit result with 3 edits, got %+v", res)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	if got := string(data); got != expected {
		t.Fatalf("unexpected edited content %q", got)
	}
	if !strings.Contains(res.ContentPreview, "punctuation") {
		t.Fatalf("expected preview to include edited code, got %+v", res)
	}
}

func TestApplyRejectsOverlappingEdits(t *testing.T) {
	target := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(target, []byte("abcdef\n"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	_, err := Apply(Request{
		Path:      target,
		Operation: "edit",
		Edits: []Edit{
			{StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 4, NewText: "xy"},
			{StartLine: 1, StartColumn: 3, EndLine: 1, EndColumn: 5, NewText: "z"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "overlaps") {
		t.Fatalf("expected overlap error, got %v", err)
	}
}

func TestApplyRejectsNonRegularFile(t *testing.T) {
	target := t.TempDir()

	_, err := Apply(Request{
		Path:      target,
		Operation: "write",
		Content:   "hello",
	})
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("expected regular file error, got %v", err)
	}
}

func TestApplyRejectsBinaryFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "binary.txt")
	if err := os.WriteFile(target, []byte("a\x00b"), 0644); err != nil {
		t.Fatalf("seed binary file: %v", err)
	}

	_, err := Apply(Request{
		Path:      target,
		Operation: "append",
		Content:   "tail",
	})
	if err == nil || !strings.Contains(err.Error(), "text file") {
		t.Fatalf("expected text file error, got %v", err)
	}
}

func lineEndColumn(t *testing.T, content string, line int) int {
	t.Helper()

	lines := strings.Split(content, "\n")
	if line < 1 || line > len(lines) {
		t.Fatalf("line %d out of range", line)
	}
	return len([]rune(lines[line-1])) + 1
}
