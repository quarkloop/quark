package dgraph

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/dgo/v250"
	"github.com/dgraph-io/dgo/v250/protos/api"
)

const mutationRetryAttempts = 5

func (d *Driver) doMutation(ctx context.Context, operation string, request func() *api.Request) error {
	var err error
	for attempt := 1; attempt <= mutationRetryAttempts; attempt++ {
		_, err = d.client.NewTxn().Do(ctx, request())
		if err == nil {
			return nil
		}
		if !errors.Is(err, dgo.ErrAborted) {
			return fmt.Errorf("%s: %w", operation, err)
		}
		if attempt == mutationRetryAttempts {
			break
		}
		d.logger.Warn("retrying aborted dgraph mutation", "operation", operation, "attempt", attempt, "error", err)
		if waitErr := waitBeforeRetry(ctx, attempt); waitErr != nil {
			return fmt.Errorf("%s: %w", operation, waitErr)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func waitBeforeRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(attempt*attempt) * 50 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
