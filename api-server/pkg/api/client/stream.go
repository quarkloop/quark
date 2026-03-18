package client

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
)

// StreamSSE handles the long-lived connection and line-by-line parsing.
func (c *Client) StreamSSE(ctx context.Context, path string, fn func(string)) error {
	fullURL := fmt.Sprintf("%s/%s", c.baseURL, strings.TrimLeft(path, "/"))

	// 1. Create request with context to allow caller to stop the stream
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("stream request creation failed: %w", err)
	}

	// 2. Set headers appropriate for SSE
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream connection failed: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return err
	}

	// 3. Use a Scanner to read line-by-line.
	// This prevents splitting a single log message across two buffer reads.
	scanner := bufio.NewScanner(resp.Body)
	for {
		select {
		case <-ctx.Done():
			// Return nil or ctx.Err() depending on if you consider
			// manual cancellation an "error"
			return ctx.Err()
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("stream read error: %w", err)
				}
				return nil // Normal EOF
			}

			line := scanner.Text()
			if line == "" {
				continue // Skip empty keep-alive lines
			}

			// SSE protocol usually prefixes data with "data: "
			// You can decide whether to trim this here or in the callback
			cleanLine := strings.TrimPrefix(line, "data: ")
			fn(cleanLine)
		}
	}
}
