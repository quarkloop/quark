package query

import (
	"testing"

	"github.com/quarkloop/quark/server/internal/domain"
)

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want bool
	}{
		{domain.NodeStateActive, domain.NodeStatePaused, true},
		{domain.NodeStatePaused, domain.NodeStateActive, true},
		{domain.NodeStateActive, domain.NodeStateDraining, true},
		{domain.NodeStateDraining, domain.NodeStateArchived, true},
		{domain.NodeStateError, domain.NodeStateRecovering, true},
		{domain.NodeStateArchived, domain.NodeStateActive, false}, // can't go directly to ACTIVE
		{domain.NodeStateDeleted, domain.NodeStateActive, false},  // terminal
		{domain.NodeStateDeleted, domain.NodeStateArchived, false},
		{domain.NodeStateActive, domain.NodeStateActive, true}, // same state is allowed (no-op)
	}
	for _, tt := range tests {
		got := isValidTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("isValidTransition(%q, %q) = %v, want %v",
				tt.from, tt.to, got, tt.want)
		}
	}
}
