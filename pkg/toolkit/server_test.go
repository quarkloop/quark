package toolkit

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/quarkloop/pkg/plugin"
)

func TestBuildServerSchemaEndpointDispatchesSingleCommand(t *testing.T) {
	tool := testTool{
		name:       "bash",
		schemaName: "bash",
		commands: []Command{{
			Name: "run",
			Args: []Arg{{Name: "cmd", Required: true}},
			Handler: func(_ context.Context, input Input) (Output, error) {
				return Output{Data: map[string]any{"cmd": input.Args["cmd"]}}, nil
			},
		}},
	}
	app := BuildServer(tool)

	resp, err := app.Test(httptestRequest(http.MethodPost, "/bash", `{"cmd":"echo ok"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var out Output
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Data["cmd"] != "echo ok" {
		t.Fatalf("cmd = %#v", out.Data["cmd"])
	}
}

func TestBuildServerSchemaEndpointAcceptsCommandAliasForCmd(t *testing.T) {
	tool := testTool{
		name:       "bash",
		schemaName: "bash",
		commands: []Command{{
			Name: "run",
			Args: []Arg{{Name: "cmd", Required: true}},
			Handler: func(_ context.Context, input Input) (Output, error) {
				return Output{Data: map[string]any{"cmd": input.Args["cmd"]}}, nil
			},
		}},
	}
	app := BuildServer(tool)

	resp, err := app.Test(httptestRequest(http.MethodPost, "/bash", `{"command":"echo ok"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var out Output
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Data["cmd"] != "echo ok" {
		t.Fatalf("cmd = %#v", out.Data["cmd"])
	}
}

func TestBuildServerSchemaEndpointDispatchesCommandField(t *testing.T) {
	tool := testTool{
		name:       "fs",
		schemaName: "fs",
		commands: []Command{{
			Name: "read",
			Args: []Arg{{Name: "path", Required: true}},
			Flags: []Flag{{
				Name:    "start-line",
				Type:    "int",
				Default: 0,
			}},
			Handler: func(_ context.Context, input Input) (Output, error) {
				return Output{Data: map[string]any{
					"path":       input.Args["path"],
					"start_line": input.Flags["start-line"],
				}}, nil
			},
		}},
	}
	app := BuildServer(tool)

	resp, err := app.Test(httptestRequest(http.MethodPost, "/fs", `{"command":"read","path":"notes.txt","start_line":2}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out Output
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Data["path"] != "notes.txt" {
		t.Fatalf("path = %#v", out.Data["path"])
	}
	if out.Data["start_line"] != float64(2) {
		t.Fatalf("start_line = %#v", out.Data["start_line"])
	}
}

type testTool struct {
	name       string
	schemaName string
	commands   []Command
}

func (t testTool) Name() string        { return t.name }
func (t testTool) Version() string     { return "test" }
func (t testTool) Description() string { return "test tool" }
func (t testTool) Commands() []Command { return t.commands }
func (t testTool) Schema() plugin.ToolSchema {
	return plugin.ToolSchema{Name: t.schemaName}
}

func httptestRequest(method, path, body string) *http.Request {
	req, _ := http.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}
