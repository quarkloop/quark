package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	agentapi "github.com/quarkloop/agent-api"
	"github.com/quarkloop/core/pkg/space"
)

// LocalStore implements Service using the local activity JSONL file.
type LocalStore struct {
	spaceDir string
}

// NewLocalService creates an activity service backed by the space's activity log.
func NewLocalService(spaceDir string) Service {
	return &LocalStore{spaceDir: spaceDir}
}

func (s *LocalStore) Append(_ context.Context, record agentapi.ActivityRecord) error {
	aDir := space.ActivityDir(s.spaceDir)
	if err := os.MkdirAll(aDir, 0755); err != nil {
		return err
	}
	path := space.ActivityLogPath(s.spaceDir)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open activity log: %w", err)
	}
	defer f.Close()

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write activity: %w", err)
	}
	return nil
}

func (s *LocalStore) Query(_ context.Context, opts QueryOptions) ([]agentapi.ActivityRecord, error) {
	path := space.ActivityLogPath(s.spaceDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	var sinceTime time.Time
	if opts.Since != "" {
		sinceTime, _ = time.Parse(time.RFC3339, opts.Since)
	}

	var out []agentapi.ActivityRecord
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var rec agentapi.ActivityRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if opts.Type != "" && rec.Type != opts.Type {
			continue
		}
		if !sinceTime.IsZero() && !rec.Timestamp.After(sinceTime) {
			continue
		}
		out = append(out, rec)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *LocalStore) Stream(_ context.Context, _ func(agentapi.ActivityRecord)) error {
	return ErrStreamNotSupported
}

func (s *LocalStore) Close() error {
	return nil
}
