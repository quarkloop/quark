//go:build e2e

package utils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// PostMessage POSTs a user message to the agent's message SSE endpoint and
// returns the concatenated "text" payload received on the stream.
func PostMessage(t *testing.T, ctx context.Context, env *E2EEnv, sessionID, content string) string {
	t.Helper()
	return PostMessageTrace(t, ctx, env, sessionID, content).Text
}

// MessageTrace is the observable response stream produced by PostMessageTrace.
type MessageTrace struct {
	Text        string
	ToolStarts  []string
	ToolResults []string
}

// PostMessageTrace POSTs a user message and returns streamed text plus tool
// progress events emitted by the runtime.
func PostMessageTrace(t *testing.T, ctx context.Context, env *E2EEnv, sessionID, content string) MessageTrace {
	t.Helper()

	body, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	url := fmt.Sprintf("%s/v1/sessions/%s/messages", env.AgentURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	httpc := &http.Client{} // no timeout — SSE holds the connection open
	resp, err := httpc.Do(req)
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s: status=%d body=%s", url, resp.StatusCode, string(data))
	}

	var trace MessageTrace
	var full strings.Builder
	reader := bufio.NewReader(resp.Body)
	var dataBuf bytes.Buffer
	var currentEvent string
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			t.Fatalf("read stream: %v", err)
		}
		line = bytes.TrimRight(line, "\r\n")

		if len(line) == 0 {
			if dataBuf.Len() > 0 {
				if currentEvent == "text" || currentEvent == "token" {
					var payload string
					if err := json.Unmarshal(dataBuf.Bytes(), &payload); err == nil {
						full.WriteString(payload)
					}
				} else if currentEvent == "tool_start" {
					if name := toolEventName(dataBuf.Bytes()); name != "" {
						trace.ToolStarts = append(trace.ToolStarts, name)
					}
				} else if currentEvent == "tool_result" {
					if name := toolEventName(dataBuf.Bytes()); name != "" {
						trace.ToolResults = append(trace.ToolResults, name)
					}
				} else if currentEvent == "error" {
					if IsRateLimitText(dataBuf.String()) {
						t.Skipf("provider rate limited the e2e run: %s", dataBuf.String())
					}
					t.Fatalf("agent returned error event: %s", dataBuf.String())
				}
			}
			dataBuf.Reset()
			currentEvent = ""
			continue
		}
		if bytes.HasPrefix(line, []byte(":")) {
			continue
		}
		if rest, ok := bytes.CutPrefix(line, []byte("event:")); ok {
			currentEvent = strings.TrimSpace(string(rest))
			continue
		}
		if rest, ok := bytes.CutPrefix(line, []byte("data:")); ok {
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.Write(bytes.TrimPrefix(rest, []byte(" ")))
		}
	}
	trace.Text = full.String()
	return trace
}

func toolEventName(data []byte) string {
	var payload map[string]string
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return payload["name"]
}

// AgentSessionsCount reads GET /v1/info on the agent and returns the session
// count from the response.
func AgentSessionsCount(t *testing.T, env *E2EEnv) int {
	t.Helper()
	resp, err := env.HTTPC.Get(env.AgentURL + "/v1/info")
	if err != nil {
		t.Fatalf("get /v1/info: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /v1/info: status=%d body=%s", resp.StatusCode, string(body))
	}
	var payload struct {
		Sessions int `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode /v1/info: %v", err)
	}
	return payload.Sessions
}

// WaitForAgentSession polls the agent's /v1/info until the session count
// reflects that a session has been mirrored from the supervisor.
func WaitForAgentSession(t *testing.T, env *E2EEnv, id string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if AgentSessionsCount(t, env) > 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("agent never mirrored session %s", id)
}
