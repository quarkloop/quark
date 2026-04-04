package agentcore

import (
	"fmt"
	"time"

	"github.com/quarkloop/agent/pkg/eventbus"
)

// EmitActivity emits a typed event on the given event bus.
func EmitActivity(bus *eventbus.Bus, sessionKey string, kind eventbus.EventKind, data any) {
	if bus == nil {
		return
	}
	bus.Emit(eventbus.Event{
		ID:        fmt.Sprintf("%s-%d", kind, time.Now().UnixNano()),
		SessionID: sessionKey,
		Kind:      kind,
		Timestamp: time.Now().UTC(),
		Data:      data,
	})
}
