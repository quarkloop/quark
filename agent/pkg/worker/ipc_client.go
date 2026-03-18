package worker

import (
	"encoding/json"
	"fmt"
	"net"
)

// ipcConn wraps a Unix socket connection for the worker side.
type ipcConn struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

func dialIPC(socketPath string) (*ipcConn, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", socketPath, err)
	}
	return &ipcConn{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(conn),
	}, nil
}

type taskMsg struct {
	Type    string `json:"type"`
	StepID  string `json:"step_id"`
	Agent   string `json:"agent"`
	Task    string `json:"description"`
}

type eventMsg struct {
	Type    string `json:"type"`
	StepID  string `json:"step_id"`
	Message string `json:"message"`
}

type resultMsg struct {
	Type   string `json:"type"`
	StepID string `json:"step_id"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (c *ipcConn) readTask() (*taskMsg, error) {
	var msg taskMsg
	if err := c.dec.Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (c *ipcConn) sendEvent(stepID, message string) error {
	return c.enc.Encode(eventMsg{Type: "event", StepID: stepID, Message: message})
}

func (c *ipcConn) sendResult(stepID, status, result, errMsg string) error {
	err := c.enc.Encode(resultMsg{
		Type:   "result",
		StepID: stepID,
		Status: status,
		Result: result,
		Error:  errMsg,
	})
	c.conn.Close()
	return err
}

func (c *ipcConn) close() { c.conn.Close() }
