package loop_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quarkloop/agent/pkg/loop"
)

type testMessage struct {
	loop.BaseMessage
	Value string
}

func TestLoopBasic(t *testing.T) {
	l := loop.New(loop.WithInboxSize(8))

	var handled atomic.Int32

	l.Register("test", func(ctx context.Context, msg loop.Message) error {
		handled.Add(1)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go l.Run(ctx)

	l.Send(testMessage{BaseMessage: loop.NewMessage("test"), Value: "hello"})
	l.Send(testMessage{BaseMessage: loop.NewMessage("test"), Value: "world"})

	time.Sleep(100 * time.Millisecond)

	if got := handled.Load(); got != 2 {
		t.Errorf("expected 2 messages handled, got %d", got)
	}
}

func TestLoopPriority(t *testing.T) {
	l := loop.New(loop.WithWorkPriority(true))

	var order []string

	l.Register("low", func(ctx context.Context, msg loop.Message) error {
		order = append(order, "low")
		return nil
	})
	l.Register("high", func(ctx context.Context, msg loop.Message) error {
		order = append(order, "high")
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Send low priority first, high priority second
	l.Send(testMessage{BaseMessage: loop.NewMessage("low"), Value: "1"})
	l.Send(testMessage{BaseMessage: loop.NewPriorityMessage("high", 10), Value: "2"})

	go l.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// High priority should be processed first because work queue is checked first
	if len(order) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(order))
	}
	if order[0] != "high" {
		t.Errorf("expected high priority first, got order: %v", order)
	}
}

func TestLoopMiddleware(t *testing.T) {
	l := loop.New()

	var middlewareCalled bool
	var handlerCalled bool

	l.Use(func(next loop.HandlerFunc) loop.HandlerFunc {
		return func(ctx context.Context, msg loop.Message) error {
			middlewareCalled = true
			return next(ctx, msg)
		}
	})

	l.Register("test", func(ctx context.Context, msg loop.Message) error {
		handlerCalled = true
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go l.Run(ctx)

	l.Send(testMessage{BaseMessage: loop.NewMessage("test")})
	time.Sleep(100 * time.Millisecond)

	if !middlewareCalled {
		t.Error("middleware was not called")
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
}

func TestLoopUnhandled(t *testing.T) {
	var unhandledCalled bool

	l := loop.New(loop.WithUnhandledCallback(func(msg loop.Message) {
		unhandledCalled = true
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go l.Run(ctx)

	l.Send(testMessage{BaseMessage: loop.NewMessage("unknown")})
	time.Sleep(100 * time.Millisecond)

	if !unhandledCalled {
		t.Error("unhandled callback was not called")
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	l := loop.New()

	var recovered bool

	// Observer must be registered first so it wraps Recovery and sees the error
	l.Use(loop.ObserverMiddleware(func(msgType string, err error) {
		if err != nil {
			recovered = true
		}
	}))
	l.Use(loop.RecoveryMiddleware)

	l.Register("panic", func(ctx context.Context, msg loop.Message) error {
		panic("test panic")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go l.Run(ctx)

	l.Send(testMessage{BaseMessage: loop.NewMessage("panic")})
	time.Sleep(100 * time.Millisecond)

	if !recovered {
		t.Error("panic was not recovered")
	}
}
