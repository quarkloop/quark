package dag_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quarkloop/runtime/pkg/dag"
)

func TestDAGNew(t *testing.T) {
	steps := []dag.DAGStep{
		{ID: "step-1", Name: "First", Action: "do step 1"},
		{ID: "step-2", Name: "Second", Action: "do step 2", DependsOn: []string{"step-1"}},
		{ID: "step-3", Name: "Third", Action: "do step 3", DependsOn: []string{"step-1"}},
		{ID: "step-4", Name: "Fourth", Action: "do step 4", DependsOn: []string{"step-2", "step-3"}},
	}

	d, err := dag.New(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(d.Steps()) != 4 {
		t.Errorf("expected 4 steps, got %d", len(d.Steps()))
	}
}

func TestDAGMissingDependency(t *testing.T) {
	steps := []dag.DAGStep{
		{ID: "step-1", Name: "First", Action: "do step 1", DependsOn: []string{"nonexistent"}},
	}

	_, err := dag.New(steps)
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
}

func TestDAGCycleDetection(t *testing.T) {
	steps := []dag.DAGStep{
		{ID: "a", Name: "A", Action: "do a", DependsOn: []string{"c"}},
		{ID: "b", Name: "B", Action: "do b", DependsOn: []string{"a"}},
		{ID: "c", Name: "C", Action: "do c", DependsOn: []string{"b"}},
	}

	_, err := dag.New(steps)
	if err == nil {
		t.Fatal("expected error for cycle")
	}
}

func TestDAGReady(t *testing.T) {
	steps := []dag.DAGStep{
		{ID: "step-1", Name: "First", Action: "do step 1"},
		{ID: "step-2", Name: "Second", Action: "do step 2", DependsOn: []string{"step-1"}},
	}

	d, _ := dag.New(steps)
	d.UpdateReadySteps()

	ready := d.Ready()
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready step, got %d", len(ready))
	}
	if ready[0].ID != "step-1" {
		t.Errorf("expected step-1 to be ready, got %s", ready[0].ID)
	}
}

func TestDAGExecution(t *testing.T) {
	steps := []dag.DAGStep{
		{ID: "step-1", Name: "First", Action: "do step 1"},
		{ID: "step-2", Name: "Second", Action: "do step 2", DependsOn: []string{"step-1"}},
	}

	d, _ := dag.New(steps)

	var order []string

	runner := func(_ context.Context, stepID, _ string) (string, error) {
		order = append(order, stepID)
		return "done", nil
	}

	exec := dag.NewExecutor(d, runner, dag.ExecutorConfig{
		DefaultTimeout: 5 * time.Second,
	})

	err := exec.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 2 {
		t.Fatalf("expected 2 steps executed, got %d", len(order))
	}
	if order[0] != "step-1" {
		t.Errorf("expected step-1 first, got %s", order[0])
	}
	if order[1] != "step-2" {
		t.Errorf("expected step-2 second, got %s", order[1])
	}
}

func TestDAGParallelExecution(t *testing.T) {
	steps := []dag.DAGStep{
		{ID: "step-1", Name: "First", Action: "do step 1"},
		{ID: "step-2a", Name: "Second A", Action: "do step 2a", DependsOn: []string{"step-1"}},
		{ID: "step-2b", Name: "Second B", Action: "do step 2b", DependsOn: []string{"step-1"}},
		{ID: "step-3", Name: "Third", Action: "do step 3", DependsOn: []string{"step-2a", "step-2b"}},
	}

	d, _ := dag.New(steps)

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	runner := func(_ context.Context, stepID, _ string) (string, error) {
		c := concurrent.Add(1)
		// Track max concurrent
		for {
			old := maxConcurrent.Load()
			if c <= old || maxConcurrent.CompareAndSwap(old, c) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond) // Simulate work
		concurrent.Add(-1)
		return "done", nil
	}

	exec := dag.NewExecutor(d, runner, dag.ExecutorConfig{
		DefaultTimeout: 5 * time.Second,
	})

	err := exec.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// step-2a and step-2b should run in parallel
	if maxConcurrent.Load() < 2 {
		t.Errorf("expected at least 2 concurrent steps, got %d", maxConcurrent.Load())
	}

	// All steps should be completed
	status := exec.GetStatus()
	if status.Completed != 4 {
		t.Errorf("expected 4 completed, got %d", status.Completed)
	}
}

func TestDAGFailureSkipsDownstream(t *testing.T) {
	steps := []dag.DAGStep{
		{ID: "step-1", Name: "First", Action: "do step 1"},
		{ID: "step-2", Name: "Second", Action: "do step 2", DependsOn: []string{"step-1"}},
	}

	d, _ := dag.New(steps)

	runner := func(_ context.Context, stepID, _ string) (string, error) {
		if stepID == "step-1" {
			return "", context.DeadlineExceeded
		}
		return "done", nil
	}

	exec := dag.NewExecutor(d, runner, dag.ExecutorConfig{
		DefaultTimeout: 5 * time.Second,
	})

	err := exec.Start(context.Background())
	if err == nil {
		t.Fatal("expected error from failed step")
	}

	// step-2 should be skipped
	s2, ok := d.Get("step-2")
	if !ok {
		t.Fatal("expected step-2 to exist")
	}
	if s2.Status != dag.StepSkipped {
		t.Errorf("expected step-2 to be skipped, got %s", s2.Status)
	}
}

func TestDAGProgress(t *testing.T) {
	steps := []dag.DAGStep{
		{ID: "step-1", Name: "First", Action: "do step 1"},
		{ID: "step-2", Name: "Second", Action: "do step 2"},
	}

	d, _ := dag.New(steps)

	completed, total := d.Progress()
	if completed != 0 || total != 2 {
		t.Errorf("expected 0/2, got %d/%d", completed, total)
	}

	d.MarkRunning("step-1")
	d.MarkCompleted("step-1", "done")

	completed, total = d.Progress()
	if completed != 1 || total != 2 {
		t.Errorf("expected 1/2, got %d/%d", completed, total)
	}
}
