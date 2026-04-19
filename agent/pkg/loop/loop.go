package loop

import (
	"context"
	"fmt"
	"sync"
)

// Loop is the core message processing engine.
// It receives messages via Send/Submit, dispatches them to registered handlers,
// and applies middleware for cross-cutting concerns.
type Loop struct {
	inbox      chan Message
	workQueue  chan Message
	handlers   map[string]HandlerFunc
	middleware []Middleware
	opts       options
	mu         sync.RWMutex
	running    bool
}

// New creates a new Loop with the given options.
func New(opts ...Option) *Loop {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	return &Loop{
		inbox:     make(chan Message, o.inboxSize),
		workQueue: make(chan Message, o.workQueueSize),
		handlers:  make(map[string]HandlerFunc),
		opts:      o,
	}
}

// Register adds a handler for the given message type.
// Only one handler per type is allowed; later registrations replace earlier ones.
func (l *Loop) Register(msgType string, handler HandlerFunc) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handlers[msgType] = handler
}

// RegisterHandler adds a Handler implementation.
func (l *Loop) RegisterHandler(h Handler) {
	l.Register(h.Type(), h.Handle)
}

// Use adds middleware to the chain. Middleware is applied in registration order.
func (l *Loop) Use(mw ...Middleware) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.middleware = append(l.middleware, mw...)
}

// Send submits a message to the loop. High-priority messages (Priority > 0)
// go to the work queue; others go to the inbox.
// This method is safe to call from any goroutine.
func (l *Loop) Send(msg Message) {
	if l.opts.workPriority && msg.Priority() > 0 {
		select {
		case l.workQueue <- msg:
		default:
			// Work queue full, fall back to inbox
			l.inbox <- msg
		}
	} else {
		l.inbox <- msg
	}
}

// Submit is an alias for Send.
func (l *Loop) Submit(msg Message) {
	l.Send(msg)
}

// Run starts the message loop. It blocks until the context is cancelled.
// Work queue messages are always processed before inbox messages.
func (l *Loop) Run(ctx context.Context) error {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return fmt.Errorf("loop is already running")
	}
	l.running = true
	l.mu.Unlock()

	defer func() {
		l.mu.Lock()
		l.running = false
		l.mu.Unlock()
		if l.opts.onShutdown != nil {
			l.opts.onShutdown()
		}
	}()

	for {
		// Priority: always try work queue first
		if l.opts.workPriority {
			select {
			case msg := <-l.workQueue:
				l.dispatch(ctx, msg)
				continue
			default:
			}
		}

		// Work queue empty or disabled — process inbox or work queue
		select {
		case msg := <-l.workQueue:
			l.dispatch(ctx, msg)
		case msg := <-l.inbox:
			l.dispatch(ctx, msg)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// dispatch routes a message to its handler, applying middleware.
func (l *Loop) dispatch(ctx context.Context, msg Message) {
	l.mu.RLock()
	handler, ok := l.handlers[msg.Type()]
	middleware := l.middleware
	l.mu.RUnlock()

	if !ok {
		if l.opts.onUnhandled != nil {
			l.opts.onUnhandled(msg)
		}
		return
	}

	// Build the middleware chain
	wrapped := chain(handler, middleware...)

	// Execute the handler
	if err := wrapped(ctx, msg); err != nil {
		// Errors are silently ignored at this level.
		// Use middleware for error handling/logging.
		_ = err
	}
}

// Running returns true if the loop is currently running.
func (l *Loop) Running() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.running
}

// Handlers returns the registered message types.
func (l *Loop) Handlers() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	types := make([]string, 0, len(l.handlers))
	for t := range l.handlers {
		types = append(types, t)
	}
	return types
}
