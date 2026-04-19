package llmcontext

import "testing"

func TestCompactIndex(t *testing.T) {
	tests := []struct {
		name          string
		contents      []int
		contextWindow int
		wantStart     int
	}{
		{
			name:          "empty history",
			contents:      []int{},
			contextWindow: 8192,
			wantStart:     0,
		},
		{
			name:          "within both limits — no compaction",
			contents:      []int{100, 200, 300},
			contextWindow: 8192, // budget = 8192 * 4 * 0.8 = 26214 chars
			wantStart:     0,
		},
		{
			name:          "zero context window disables char compaction — only maxMessages applies",
			contents:      []int{100, 200, 300},
			contextWindow: 0,
			wantStart:     0,
		},
		{
			name: "exceeds char budget — drops oldest messages",
			// budget = 1000 * 4 * 0.8 = 3200 chars
			// total = 1000 + 1000 + 1000 + 1000 = 4000 > 3200
			// drop msg[0] (1000) → 3000 ≤ 3200 and 3 msgs ≤ 200 → start=1
			contents:      []int{1000, 1000, 1000, 1000},
			contextWindow: 1000,
			wantStart:     1,
		},
		{
			name: "well over budget — drops several messages",
			// budget = 500 * 4 * 0.8 = 1600 chars
			// total = 5 * 1000 = 5000
			// drop until ≤ 1600: need to drop 4 msgs (leaving 1000 ≤ 1600)
			contents:      []int{1000, 1000, 1000, 1000, 1000},
			contextWindow: 500,
			wantStart:     4,
		},
		{
			name: "exceeds maxMessages — drops oldest regardless of char budget",
			// 210 messages of 1 char each — well within char budget but > maxMessages(200)
			// need to drop 10, keep last 200 → start=10
			contents:      makeContents(210, 1),
			contextWindow: 999999,
			wantStart:     10,
		},
		{
			name: "exactly at char budget — no compaction",
			// budget = 1000 * 4 * 0.8 = 3200; total = 3200
			contents:      []int{1600, 1600},
			contextWindow: 1000,
			wantStart:     0,
		},
		{
			name: "single oversized message — keep it (last message fallback)",
			// budget = 100 * 4 * 0.8 = 320 chars; single msg = 10000 > budget
			// loop exhausts, fallback keeps last message → start = n-1 = 0
			contents:      []int{10000},
			contextWindow: 100,
			wantStart:     0,
		},
		{
			name: "multiple oversized messages — keep only the last",
			// budget = 100 * 4 * 0.8 = 320; all messages > budget
			// loop exhausts, fallback → start = n-1 = 2
			contents:      []int{5000, 5000, 5000},
			contextWindow: 100,
			wantStart:     2,
		},
		{
			name: "exactly one message fits after dropping",
			// budget = 200 * 4 * 0.8 = 640; msgs: 500, 500, 600
			// drop 500 → 1100 still > 640; drop 500 → 600 ≤ 640 → start=2
			contents:      []int{500, 500, 600},
			contextWindow: 200,
			wantStart:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompactIndex(tt.contents, tt.contextWindow)
			if got != tt.wantStart {
				t.Errorf("CompactIndex(%v, %d) = %d, want %d",
					tt.contents, tt.contextWindow, got, tt.wantStart)
			}
		})
	}
}

// makeContents returns a slice of n entries each with the given char count.
func makeContents(n, charCount int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = charCount
	}
	return s
}

func TestContext(t *testing.T) {
	ctx := New(8192)

	ctx.Add("user", "Hello")
	ctx.Add("assistant", "Hi there")

	if ctx.Len() != 2 {
		t.Errorf("expected 2 messages, got %d", ctx.Len())
	}

	msgs := ctx.Messages()
	if msgs[0].Role != "user" || msgs[0].Content != "Hello" {
		t.Errorf("unexpected first message: %+v", msgs[0])
	}

	ctx.Clear()
	if ctx.Len() != 0 {
		t.Errorf("expected 0 messages after clear, got %d", ctx.Len())
	}
}

func TestWorkContext(t *testing.T) {
	work := NewWorkContext(8192)

	work.Add("system", "You are executing a task")
	work.Add("user", "Step 1: do X")
	work.Add("assistant", "Done X")

	msgs := work.Messages()
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages, got %d", len(msgs))
	}

	work.Clear()
	if work.Len() != 0 {
		t.Errorf("expected 0 after clear")
	}
}

func TestSessionContext(t *testing.T) {
	session := NewSessionContext("session-123", 8192)

	if session.SessionID() != "session-123" {
		t.Errorf("unexpected session ID: %s", session.SessionID())
	}

	session.Add("user", "What's the status?")
	session.Add("assistant", "Everything is fine")

	msgs := session.Messages()
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}

func TestSessionContextWithWorkSummary(t *testing.T) {
	session := NewSessionContext("session-123", 8192)
	session.Add("user", "How's the work going?")

	msgs := session.BuildWithWorkSummary("You are an AI assistant.", "Executing step 2/5")

	if len(msgs) != 2 {
		t.Errorf("expected 2 messages (system + user), got %d", len(msgs))
	}

	// First message should be system with work summary
	if msgs[0].Role != "system" {
		t.Errorf("expected system role, got %s", msgs[0].Role)
	}
	if msgs[0].Content == "You are an AI assistant." {
		t.Error("work summary should be included in system prompt")
	}
	if msgs[0].Content != "You are an AI assistant.\n\n## Current Work Status\nExecuting step 2/5" {
		t.Errorf("unexpected system content: %s", msgs[0].Content)
	}

	// Session messages should follow
	if msgs[1].Role != "user" || msgs[1].Content != "How's the work going?" {
		t.Errorf("unexpected user message: %+v", msgs[1])
	}
}

func TestContextIsolation(t *testing.T) {
	// Work and session contexts must be completely isolated
	work := NewWorkContext(8192)
	session := NewSessionContext("s1", 8192)

	work.Add("user", "Work task")
	session.Add("user", "Chat message")

	// Verify no cross-contamination
	workMsgs := work.Messages()
	sessionMsgs := session.Messages()

	if len(workMsgs) != 1 || workMsgs[0].Content != "Work task" {
		t.Error("work context contaminated")
	}
	if len(sessionMsgs) != 1 || sessionMsgs[0].Content != "Chat message" {
		t.Error("session context contaminated")
	}
}
