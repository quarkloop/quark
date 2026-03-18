package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// StreamChunk is a single token or content delta received from a streaming
// LLM response.
type StreamChunk struct {
	// Delta is the incremental text content for this chunk.
	Delta string
	// Done is true on the final chunk (no more content follows).
	Done bool
	// Err is non-nil if the stream encountered a provider error mid-stream.
	Err error
}

// StreamingGateway extends Gateway with token-by-token streaming.
// Not all gateway implementations are required to satisfy this interface;
// callers should type-assert before using:
//
//	if sg, ok := gw.(model.StreamingGateway); ok {
//	    ch, err := sg.InferStream(ctx, payload)
//	}
type StreamingGateway interface {
	Gateway
	// InferStream sends payload to the provider and returns a channel of
	// StreamChunks. The channel is closed after the final chunk (Done==true)
	// or on error. The caller must drain the channel.
	InferStream(ctx context.Context, payload []byte) (<-chan StreamChunk, error)
}

// CollectStream drains a StreamChunk channel and returns the concatenated
// content plus approximate token counts from the final stats chunk (if
// provided by the gateway). It is a convenience wrapper for callers that
// do not need incremental processing.
func CollectStream(ch <-chan StreamChunk) (string, error) {
	var sb strings.Builder
	for chunk := range ch {
		if chunk.Err != nil {
			return sb.String(), chunk.Err
		}
		sb.WriteString(chunk.Delta)
		if chunk.Done {
			break
		}
	}
	// Drain any remaining chunks after Done (there should be none, but be safe).
	for range ch {
	}
	return sb.String(), nil
}

// ─── OpenAI / Zhipu SSE streaming ────────────────────────────────────────────
//
// Both providers use the identical Server-Sent Events format:
//
//	data: {"id":"...","choices":[{"delta":{"content":"token"},"finish_reason":null}]}
//	data: [DONE]

type openAIStreamDelta struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// streamOpenAI performs an SSE streaming request to an OpenAI-compatible
// endpoint (used by both openAIGateway and zhipuGateway).
func streamOpenAI(ctx context.Context, client *http.Client, url, authHeader string, payload []byte) (<-chan StreamChunk, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stream http: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("stream http %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan StreamChunk, 32)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- StreamChunk{Done: true}
				return
			}
			var event openAIStreamDelta
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue // skip malformed lines
			}
			if event.Error != nil {
				ch <- StreamChunk{Err: fmt.Errorf("provider stream error %s: %s", event.Error.Type, event.Error.Message)}
				return
			}
			for _, choice := range event.Choices {
				if choice.Delta.Content != "" {
					ch <- StreamChunk{Delta: choice.Delta.Content}
				}
				if choice.FinishReason != nil && *choice.FinishReason != "" {
					ch <- StreamChunk{Done: true}
					return
				}
			}
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			ch <- StreamChunk{Err: fmt.Errorf("stream scan: %w", err)}
		}
	}()
	return ch, nil
}

// InferStream implements StreamingGateway for openAIGateway.
func (g *openAIGateway) InferStream(ctx context.Context, payload []byte) (<-chan StreamChunk, error) {
	// Inject stream:true into the payload.
	payload, err := injectStreamTrue(payload)
	if err != nil {
		return nil, err
	}
	return streamOpenAI(ctx, g.http,
		"https://api.openai.com/v1/chat/completions",
		"Bearer "+g.apiKey,
		payload)
}

// InferStream implements StreamingGateway for zhipuGateway.
// GLM uses an OpenAI-compatible SSE format.
func (g *zhipuGateway) InferStream(ctx context.Context, payload []byte) (<-chan StreamChunk, error) {
	payload, err := injectStreamTrue(payload)
	if err != nil {
		return nil, err
	}
	return streamOpenAI(ctx, g.http,
		"https://open.bigmodel.cn/api/paas/v4/chat/completions",
		"Bearer "+g.apiKey,
		payload)
}

// ─── Anthropic SSE streaming ──────────────────────────────────────────────────
//
// Anthropic uses a different SSE event vocabulary:
//
//	event: content_block_delta
//	data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"token"}}
//
//	event: message_stop
//	data: {"type":"message_stop"}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// InferStream implements StreamingGateway for anthropicGateway.
func (g *anthropicGateway) InferStream(ctx context.Context, payload []byte) (<-chan StreamChunk, error) {
	payload, err := injectStreamTrue(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", g.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic stream http: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("anthropic stream http %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan StreamChunk, 32)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			switch event.Type {
			case "content_block_delta":
				if event.Delta != nil && event.Delta.Type == "text_delta" && event.Delta.Text != "" {
					ch <- StreamChunk{Delta: event.Delta.Text}
				}
			case "message_stop":
				ch <- StreamChunk{Done: true}
				return
			case "error":
				if event.Error != nil {
					ch <- StreamChunk{Err: fmt.Errorf("anthropic stream error %s: %s",
						event.Error.Type, event.Error.Message)}
				}
				return
			}
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			ch <- StreamChunk{Err: fmt.Errorf("anthropic stream scan: %w", err)}
		}
	}()
	return ch, nil
}

// ─── Noop streaming ───────────────────────────────────────────────────────────

// InferStream implements StreamingGateway for noopGateway.
// Emits the canned response as a series of word-sized chunks.
func (g *noopGateway) InferStream(_ context.Context, payload []byte) (<-chan StreamChunk, error) {
	resp, _ := g.InferRaw(context.Background(), payload)
	words := strings.Fields(resp.Content)
	ch := make(chan StreamChunk, len(words)+1)
	go func() {
		defer close(ch)
		for i, w := range words {
			sep := ""
			if i > 0 {
				sep = " "
			}
			ch <- StreamChunk{Delta: sep + w}
		}
		ch <- StreamChunk{Done: true}
	}()
	return ch, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// injectStreamTrue unmarshals payload, sets stream:true, and re-marshals.
// This ensures the streaming flag is present regardless of what the adapter built.
func injectStreamTrue(payload []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, fmt.Errorf("inject stream: unmarshal: %w", err)
	}
	m["stream"] = true
	// GLM does not support stream_options.include_usage, so do not inject it.
	out, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("inject stream: marshal: %w", err)
	}
	return out, nil
}
