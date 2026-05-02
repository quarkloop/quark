package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	api "github.com/quarkloop/supervisor/pkg/api"
	event "github.com/quarkloop/pkg/event"
)

// SessionType is the type of a session.
type SessionType = api.SessionType

const (
	SessionTypeMain     SessionType = api.SessionTypeMain
	SessionTypeChat     SessionType = api.SessionTypeChat
	SessionTypeSubAgent SessionType = api.SessionTypeSubAgent
	SessionTypeCron     SessionType = api.SessionTypeCron
)

// CreateSessionRequest is the body for POST /v1/spaces/{name}/sessions.
type CreateSessionRequest = api.CreateSessionRequest

// Session is the supervisor-owned record for a conversation.
type Session = api.Session

// ActivityRecord represents a single activity log entry.
type ActivityRecord = api.ActivityRecord

// ListSessions returns all sessions stored for the named space.
func (c *Client) ListSessions(ctx context.Context, space string) ([]api.Session, error) {
	var out []api.Session
	if err := c.do(ctx, http.MethodGet, c.route.SpaceSessions(space), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateSession registers a new session in the supervisor. The supervisor
// publishes a session.created event on the space stream so any listening
// agent is notified.
func (c *Client) CreateSession(ctx context.Context, space string, req api.CreateSessionRequest) (api.Session, error) {
	var out api.Session
	err := c.do(ctx, http.MethodPost, c.route.SpaceSessions(space), req, &out)
	return out, err
}

// GetSession fetches a single session by id.
func (c *Client) GetSession(ctx context.Context, space, id string) (api.Session, error) {
	var out api.Session
	err := c.do(ctx, http.MethodGet, c.route.SpaceSession(space, id), nil, &out)
	return out, err
}

// DeleteSession removes a session. The supervisor publishes session.deleted
// on the space stream.
func (c *Client) DeleteSession(ctx context.Context, space, id string) error {
	return c.do(ctx, http.MethodDelete, c.route.SpaceSession(space, id), nil, nil)
}

// StreamEvents subscribes to the supervisor's space-scoped SSE event stream
// and invokes fn for every event received. The call blocks until ctx is
// cancelled, the server closes the connection, or an unrecoverable error
// occurs.
//
// Because StreamEvents holds the HTTP connection open for the lifetime of
// ctx, the call ignores the Client's configured timeout by using a
// dedicated no-timeout HTTP client.
func (c *Client) StreamEvents(ctx context.Context, space string, fn func(event.Event)) error {
	return c.StreamEventsWithReady(ctx, space, nil, fn)
}

// StreamEventsWithReady is like StreamEvents but invokes onReady (if non-nil)
// once the HTTP response headers have arrived successfully — i.e., the
// supervisor has registered this subscriber and no further events for this
// space can be missed.
func (c *Client) StreamEventsWithReady(ctx context.Context, space string, onReady func(), fn func(event.Event)) error {
	url := c.baseURL + c.route.SpaceEventStream(space)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build events request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Clone the transport but drop the timeout — SSE holds the connection
	// open indefinitely.
	httpc := *c.http
	httpc.Timeout = 0
	resp, err := httpc.Do(req)
	if err != nil {
		return fmt.Errorf("supervisor events %s: %w", space, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return &HTTPError{
			Method:     http.MethodGet,
			Path:       c.route.SpaceEventStream(space),
			StatusCode: resp.StatusCode,
			Body:       string(data),
		}
	}
	if onReady != nil {
		onReady()
	}

	reader := bufio.NewReaderSize(resp.Body, 32*1024)
	var dataBuf bytes.Buffer
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF || ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("read events: %w", err)
		}
		// Normalise CRLF.
		line = bytes.TrimRight(line, "\r\n")

		// Blank line terminates an event frame.
		if len(line) == 0 {
			if dataBuf.Len() == 0 {
				continue
			}
			var ev event.Event
			if err := json.Unmarshal(dataBuf.Bytes(), &ev); err == nil {
				fn(ev)
			}
			dataBuf.Reset()
			continue
		}

		// Ignore comments (`: keepalive`).
		if bytes.HasPrefix(line, []byte(":")) {
			continue
		}

		// We only care about the `data:` field; `event:` is redundant with
		// the Kind field inside the JSON payload.
		if rest, ok := bytes.CutPrefix(line, []byte("data:")); ok {
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.Write(bytes.TrimPrefix(rest, []byte(" ")))
			continue
		}
		// Other fields (event:, id:, retry:) are ignored.
	}
}
