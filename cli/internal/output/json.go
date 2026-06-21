package output

import (
        "encoding/json"
        "fmt"
        "io"
)

// JSONPrinter emits the raw API response as indented JSON.
// Used when --json flag is set (for AI agents and scripting).
type JSONPrinter struct {
        w io.Writer
}

func (p *JSONPrinter) print(v interface{}) error {
        bs, err := json.MarshalIndent(v, "", "  ")
        if err != nil {
                return fmt.Errorf("marshal json: %w", err)
        }
        _, err = p.w.Write(append(bs, '\n'))
        return err
}

func (p *JSONPrinter) PrintSystemList(systems interface{}) error { return p.print(systems) }
func (p *JSONPrinter) PrintSystemDetail(system interface{}) error { return p.print(system) }
func (p *JSONPrinter) PrintNodeList(nodes interface{}) error  { return p.print(nodes) }
func (p *JSONPrinter) PrintNodeDetail(node interface{}) error { return p.print(node) }
func (p *JSONPrinter) PrintRegistryList(entries interface{}) error    { return p.print(entries) }
func (p *JSONPrinter) PrintRegistryEntry(entry interface{}) error     { return p.print(entry) }
func (p *JSONPrinter) PrintEventList(events interface{}) error        { return p.print(events) }
func (p *JSONPrinter) PrintHealthSummary(health interface{}) error    { return p.print(health) }
func (p *JSONPrinter) PrintSystemHealth(health interface{}) error   { return p.print(health) }
func (p *JSONPrinter) PrintNodeHealth(health interface{}) error   { return p.print(health) }
func (p *JSONPrinter) PrintDeployResult(result interface{}) error     { return p.print(result) }
func (p *JSONPrinter) PrintRaw(value interface{}) error               { return p.print(value) }
func (p *JSONPrinter) PrintSuccess(message string) error {
        // For success messages, emit a simple JSON object so scripts can detect it.
        return p.print(map[string]string{"status": "ok", "message": message})
}
func (p *JSONPrinter) PrintError(err error) error {
        // Try to extract a structured API error first — either our local
        // *APIError or anything that looks like one (duck-typed: has Code +
        // Message + optional Details fields). This avoids an import cycle
        // with the client package.
        if apiErr, ok := err.(*APIError); ok {
                return p.print(apiErr)
        }
        if coder, ok := err.(interface{ GetCode() string }); ok {
                return p.print(map[string]interface{}{
                        "code":    coder.GetCode(),
                        "message": err.Error(),
                })
        }
        // Fall back to the raw error message. The client.APIError string
        // form already includes the status code and message.
        return p.print(map[string]string{"error": err.Error()})
}

// APIError mirrors the client.APIError type for JSON output.
// We define a local type to avoid an import cycle (client imports nothing
// from output, but output shouldn't import client).
type APIError struct {
        Code    string                 `json:"code"`
        Message string                 `json:"message"`
        Details map[string]interface{} `json:"details,omitempty"`
}

func (e *APIError) Error() string {
        return e.Code + ": " + e.Message
}
