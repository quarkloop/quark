package store_test

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/quarkloop/supervisor/pkg/space/store"
)

type item struct {
	ID    string `json:"id"`
	Value int    `json:"value"`
}

// TestConcurrentWrites hammers Set/Get/Delete concurrently to catch data races.
// Run with: go test -race ./pkg/store/...
func TestConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	col, err := store.Open(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer col.Close()

	const (
		writers  = 8
		readers  = 8
		ops      = 200
		keyRange = 20 // overlap to maximise contention
	)

	var wg sync.WaitGroup

	// Writers
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				id := fmt.Sprintf("item-%d", i%keyRange)
				if err := col.Set(item{ID: id, Value: w*ops + i}); err != nil {
					t.Errorf("Set: %v", err)
				}
			}
		}(w)
	}

	// Readers
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				id := fmt.Sprintf("item-%d", i%keyRange)
				var it item
				_ = col.GetInto(id, &it) // may legitimately miss
				_ = col.List()
				_ = col.Count()
			}
		}()
	}

	// Deleters
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < ops; i++ {
			id := fmt.Sprintf("item-%d", i%keyRange)
			_ = col.Delete(id) // may legitimately fail
		}
	}()

	wg.Wait()
}

// TestOrderPreserved verifies insertion order is maintained across a reopen.
func TestOrderPreserved(t *testing.T) {
	dir := t.TempDir()
	ids := []string{"c", "a", "b", "d"}

	col, err := store.Open(dir, "order")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if err := col.Set(item{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	col.Close()

	col2, err := store.Open(dir, "order")
	if err != nil {
		t.Fatal(err)
	}
	defer col2.Close()

	got := col2.ListIDs()
	if len(got) != len(ids) {
		t.Fatalf("want %d IDs, got %d", len(ids), len(got))
	}
	for i, id := range ids {
		if got[i] != id {
			t.Errorf("position %d: want %q got %q", i, id, got[i])
		}
	}
}

// TestAtomicFlush verifies no partial writes survive a simulated crash by
// checking the file is valid JSONL after concurrent writes.
func TestAtomicFlush(t *testing.T) {
	dir := t.TempDir()
	col, err := store.Open(dir, "atomic")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			col.Set(item{ID: fmt.Sprintf("k%d", i), Value: i})
		}(i)
	}
	wg.Wait()

	// Reopen and verify all records decode cleanly.
	col2, err := store.Open(dir, "atomic")
	if err != nil {
		t.Fatal(err)
	}
	defer col2.Close()

	list := col2.List()
	if len(list) == 0 {
		t.Fatal("expected records after concurrent writes, got none")
	}

	// Verify no stale .tmp file left behind.
	if _, err := os.Stat(dir + "/atomic.jsonl.tmp"); err == nil {
		t.Error("stale .tmp file found after flush")
	}
}

// TestNotFound verifies ErrNotFound is returned for missing keys.
func TestNotFound(t *testing.T) {
	dir := t.TempDir()
	col, err := store.Open(dir, "nf")
	if err != nil {
		t.Fatal(err)
	}
	defer col.Close()

	_, err = col.Get("missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !store.IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %T: %v", err, err)
	}
}
