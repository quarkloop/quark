// Package watcher provides a polling-based file change detector.
// It checks modification times at a configurable interval and calls
// an onChange callback when any watched file changes.
// Uses polling to avoid external dependencies (no fsnotify required).
package watcher

import (
	"context"
	"os"
	"sync"
	"time"
)

const defaultInterval = 500 * time.Millisecond

// Watcher polls a set of file paths and calls onChange when any file's
// modification time advances beyond what was last recorded.
type Watcher struct {
	paths    []string
	onChange func(path string)
	interval time.Duration

	mu       sync.Mutex
	modTimes map[string]time.Time
}

// New creates a Watcher that calls onChange whenever a watched file changes.
// interval controls how often the paths are polled; pass 0 to use the default (500ms).
func New(onChange func(path string), interval time.Duration, paths ...string) *Watcher {
	if interval <= 0 {
		interval = defaultInterval
	}
	w := &Watcher{
		paths:    paths,
		onChange: onChange,
		interval: interval,
		modTimes: make(map[string]time.Time, len(paths)),
	}
	// Capture initial modification times so we don't trigger on startup.
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil {
			w.modTimes[p] = fi.ModTime()
		}
	}
	return w
}

// Start begins polling until ctx is cancelled. Intended to run in a goroutine.
func (w *Watcher) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll()
		}
	}
}

// AddPath adds a new file path to watch at runtime.
func (w *Watcher) AddPath(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, p := range w.paths {
		if p == path {
			return // already watching
		}
	}
	w.paths = append(w.paths, path)
	if fi, err := os.Stat(path); err == nil {
		w.modTimes[path] = fi.ModTime()
	}
}

func (w *Watcher) poll() {
	for _, p := range w.paths {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		curr := fi.ModTime()
		w.mu.Lock()
		prev := w.modTimes[p]
		changed := curr.After(prev)
		if changed {
			w.modTimes[p] = curr
		}
		w.mu.Unlock()
		if changed {
			w.onChange(p)
		}
	}
}
