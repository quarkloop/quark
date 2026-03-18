// Package worker implements the short-lived worker agent process.
//
// A worker process is spawned by a supervisor for each plan step. It:
//  1. Connects to the supervisor's IPC socket.
//  2. Reads the task assignment.
//  3. Executes the step (LLM call + optional tool invocations).
//  4. Sends the result back over IPC.
//  5. Exits.
package worker

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// Config holds parameters for a worker process.
type Config struct {
	SpaceID   string
	StepID    string
	IPCSocket string
}

// Worker is a short-lived agent that executes a single plan step.
type Worker struct {
	cfg *Config
}

// New creates a Worker from cfg.
func New(cfg *Config) (*Worker, error) {
	if cfg.SpaceID == "" || cfg.StepID == "" || cfg.IPCSocket == "" {
		return nil, fmt.Errorf("worker: space-id, step-id and ipc-socket are required")
	}
	return &Worker{cfg: cfg}, nil
}

// Run connects to the supervisor IPC socket, reads the task, executes it,
// and sends the result. Blocks until done or ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	// Import here to avoid circular package init issues at compile time.
	ipcPkg, err := dialIPC(w.cfg.IPCSocket)
	if err != nil {
		return fmt.Errorf("worker: ipc dial: %w", err)
	}
	defer ipcPkg.close()

	task, err := ipcPkg.readTask()
	if err != nil {
		return fmt.Errorf("worker: read task: %w", err)
	}

	log.Printf("worker[%s]: starting task: %s", task.StepID, task.Task)

	result, execErr := executeTask(ctx, task.Task, func(msg string) {
		ipcPkg.sendEvent(task.StepID, msg)
	})

	status := "complete"
	errMsg := ""
	if execErr != nil {
		status = "failed"
		errMsg = execErr.Error()
		log.Printf("worker[%s]: failed: %v", task.StepID, execErr)
	} else {
		log.Printf("worker[%s]: complete", task.StepID)
	}

	return ipcPkg.sendResult(task.StepID, status, result, errMsg)
}

// executeTask runs the task description using the available tool CLIs.
// For now it calls the bash tool to execute shell commands found in the task.
func executeTask(ctx context.Context, task string, onEvent func(string)) (string, error) {
	// Simple heuristic: if the task contains a shell command, run it.
	// A full implementation would call the LLM with available tools.
	if strings.Contains(task, "```bash") || strings.Contains(task, "$ ") {
		cmd := extractShellCommand(task)
		if cmd != "" {
			onEvent(fmt.Sprintf("executing: %s", cmd))
			out, err := exec.CommandContext(ctx, "bash", "-c", cmd).CombinedOutput()
			return string(out), err
		}
	}
	// Default: return the task description as the result.
	return fmt.Sprintf("Task completed: %s", task), nil
}

func extractShellCommand(task string) string {
	lines := strings.Split(task, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "$ ") {
			return strings.TrimPrefix(line, "$ ")
		}
	}
	return ""
}
