package write

import (
	"fmt"
	"sort"
	"strings"
)

type lineIndex struct {
	lines              []lineInfo
	totalRunes         int
	hasTrailingNewline bool
}

type lineInfo struct {
	start int
	text  []rune
}

type resolvedEdit struct {
	Start   int
	End     int
	NewText string
}

func applyLineEdits(content string, edits []Edit) (string, int, error) {
	index := buildLineIndex(content)
	resolved := make([]resolvedEdit, 0, len(edits))

	for i, edit := range edits {
		start, err := positionToRuneOffset(index, edit.StartLine, edit.StartColumn)
		if err != nil {
			return "", 0, fmt.Errorf("edit %d start: %w", i+1, err)
		}
		end, err := positionToRuneOffset(index, edit.EndLine, edit.EndColumn)
		if err != nil {
			return "", 0, fmt.Errorf("edit %d end: %w", i+1, err)
		}
		if end < start {
			return "", 0, fmt.Errorf("edit %d end position must not be before start position", i+1)
		}
		resolved = append(resolved, resolvedEdit{
			Start:   start,
			End:     end,
			NewText: edit.NewText,
		})
	}

	sort.Slice(resolved, func(i, j int) bool {
		if resolved[i].Start == resolved[j].Start {
			return resolved[i].End < resolved[j].End
		}
		return resolved[i].Start < resolved[j].Start
	})

	for i := 1; i < len(resolved); i++ {
		prev := resolved[i-1]
		curr := resolved[i]
		if curr.Start < prev.End || curr.Start == prev.Start {
			return "", 0, fmt.Errorf("edit %d overlaps another edit", i+1)
		}
	}

	runes := []rune(content)
	for i := len(resolved) - 1; i >= 0; i-- {
		edit := resolved[i]
		replacement := []rune(edit.NewText)

		next := make([]rune, 0, len(runes)-(edit.End-edit.Start)+len(replacement))
		next = append(next, runes[:edit.Start]...)
		next = append(next, replacement...)
		next = append(next, runes[edit.End:]...)
		runes = next
	}

	return string(runes), len(resolved), nil
}

func buildLineIndex(content string) lineIndex {
	segments := strings.SplitAfter(content, "\n")
	if len(segments) == 0 {
		segments = []string{""}
	}

	index := lineIndex{
		lines:      make([]lineInfo, 0, len(segments)),
		totalRunes: len([]rune(content)),
	}

	start := 0
	for _, segment := range segments {
		hasNewline := strings.HasSuffix(segment, "\n")
		text := strings.TrimSuffix(segment, "\n")
		runes := []rune(text)

		index.lines = append(index.lines, lineInfo{
			start: start,
			text:  runes,
		})

		start += len(runes)
		if hasNewline {
			start++
			index.hasTrailingNewline = true
		} else {
			index.hasTrailingNewline = false
		}
	}

	return index
}

func positionToRuneOffset(index lineIndex, line, column int) (int, error) {
	if line < 1 {
		return 0, fmt.Errorf("line must be >= 1")
	}
	if column < 1 {
		return 0, fmt.Errorf("column must be >= 1")
	}

	if line <= len(index.lines) {
		info := index.lines[line-1]
		maxColumn := len(info.text) + 1
		if column > maxColumn {
			column = maxColumn
		}
		return info.start + column - 1, nil
	}

	if line == len(index.lines)+1 && index.hasTrailingNewline {
		if column != 1 {
			return 0, fmt.Errorf("column %d out of range for line %d", column, line)
		}
		return index.totalRunes, nil
	}

	return 0, fmt.Errorf("line %d out of range", line)
}
