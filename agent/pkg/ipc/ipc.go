// Package ipc implements the Unix domain socket IPC protocol between a
// supervisor agent and its worker agents.
//
// Protocol
//
// Messages are newline-delimited JSON frames. Each frame has a Type field
// that identifies the message kind:
//
//	supervisor → worker: TaskMessage   (task assignment on connect)
//	worker → supervisor: ResultMessage (step result on completion)
//	worker → supervisor: EventMessage  (progress event mid-execution)
//
// The supervisor listens on a Unix socket. Each worker connects, receives its
// task, sends zero or more progress events, then sends the final result and
// closes the connection.
package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// MessageType identifies the kind of IPC frame.
type MessageType string

const (
	TypeTask   MessageType = "task"
	TypeResult MessageType = "result"
	TypeEvent  MessageType = "event"
)

// TaskMessage is sent by the supervisor to a worker on connect.
type TaskMessage struct {
	Type      MessageType `json:"type"`
	SpaceID   string      `json:"space_id"`
	StepID    string      `json:"step_id"`
	Agent     string      `json:"agent"`
	Task      string      `json:"description"`
	Artifacts string      `json:"artifacts,omitempty"`
}

// ResultMessage is sent by the worker to the supervisor when the step is done.
type ResultMessage struct {
	Type    MessageType `json:"type"`
	StepID  string      `json:"step_id"`
	Status  string      `json:"status"` // "complete" | "failed"
	Result  string      `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// EventMessage is sent by the worker to report progress during execution.
type EventMessage struct {
	Type    MessageType `json:"type"`
	StepID  string      `json:"step_id"`
	Message string      `json:"message"`
}

// Envelope is used for initial frame type discrimination.
type Envelope struct {
	Type MessageType `json:"type"`
}

// Server is the supervisor-side IPC listener.
type Server struct {
	socketPath string
	listener   net.Listener
	handler    func(conn net.Conn)
}

// NewServer creates an IPC server on socketPath.
// handler is called in a goroutine for each incoming worker connection.
func NewServer(socketPath string, handler func(conn net.Conn)) (*Server, error) {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0700); err != nil {
		return nil, fmt.Errorf("ipc: mkdir %s: %w", filepath.Dir(socketPath), err)
	}
	// Remove stale socket from a previous run.
	os.Remove(socketPath)

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("ipc: listen %s: %w", socketPath, err)
	}
	return &Server{socketPath: socketPath, listener: l, handler: handler}, nil
}

// Serve accepts connections until Close is called.
func (s *Server) Serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}
		go s.handler(conn)
	}
}

// Close stops the server and removes the socket file.
func (s *Server) Close() {
	s.listener.Close()
	os.Remove(s.socketPath)
}

// Client is the worker-side IPC connection to the supervisor.
type Client struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

// Dial connects to the supervisor's IPC socket.
func Dial(socketPath string) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("ipc: dial %s: %w", socketPath, err)
	}
	return &Client{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(bufio.NewReader(conn)),
	}, nil
}

// ReadTask reads the TaskMessage sent by the supervisor on connect.
func (c *Client) ReadTask() (*TaskMessage, error) {
	var msg TaskMessage
	if err := c.dec.Decode(&msg); err != nil {
		return nil, fmt.Errorf("ipc: read task: %w", err)
	}
	return &msg, nil
}

// SendEvent sends a progress event to the supervisor.
func (c *Client) SendEvent(stepID, message string) error {
	return c.enc.Encode(EventMessage{
		Type:    TypeEvent,
		StepID:  stepID,
		Message: message,
	})
}

// SendResult sends the final step result to the supervisor and closes.
func (c *Client) SendResult(stepID, status, result, errMsg string) error {
	err := c.enc.Encode(ResultMessage{
		Type:   TypeResult,
		StepID: stepID,
		Status: status,
		Result: result,
		Error:  errMsg,
	})
	c.conn.Close()
	return err
}

// WriteTask sends a task to a worker over an existing server-side connection.
func WriteTask(conn net.Conn, task *TaskMessage) error {
	return json.NewEncoder(conn).Encode(task)
}

// ReadResult reads the final ResultMessage from a worker connection.
// Intermediate EventMessages are returned via onEvent (may be nil).
func ReadResult(conn net.Conn, onEvent func(*EventMessage)) (*ResultMessage, error) {
	dec := json.NewDecoder(bufio.NewReader(conn))
	for {
		var env Envelope
		// Peek type without consuming
		raw := json.RawMessage{}
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("ipc: read frame: %w", err)
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("ipc: decode envelope: %w", err)
		}
		switch env.Type {
		case TypeEvent:
			if onEvent != nil {
				var ev EventMessage
				if err := json.Unmarshal(raw, &ev); err == nil {
					onEvent(&ev)
				}
			}
		case TypeResult:
			var res ResultMessage
			if err := json.Unmarshal(raw, &res); err != nil {
				return nil, fmt.Errorf("ipc: decode result: %w", err)
			}
			return &res, nil
		}
	}
}
