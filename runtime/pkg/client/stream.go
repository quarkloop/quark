package agentclient

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
)

func (t *Transport) StreamSSE(ctx context.Context, path string, fn func(string)) error {
	fullURL := fmt.Sprintf("%s/%s", t.baseURL, strings.TrimLeft(path, "/"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("build stream request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return err
	}

	scanner := bufio.NewScanner(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("read stream: %w", err)
				}
				return nil
			}

			line := scanner.Text()
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, ":") {
				continue
			}
			fn(strings.TrimPrefix(line, "data: "))
		}
	}
}
