package message

import (
	"context"
	"testing"
)

func TestEmitStopsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch := make(chan StreamMessage)
	if Emit(ctx, ch, StreamMessage{Type: "token", Data: "hello"}) {
		t.Fatal("Emit reported success after context cancellation")
	}
}

func TestEmitSendsWhenReceiverReady(t *testing.T) {
	ch := make(chan StreamMessage)
	received := make(chan StreamMessage, 1)
	go func() {
		received <- <-ch
	}()

	if !Emit(context.Background(), ch, StreamMessage{Type: "token", Data: "hello"}) {
		t.Fatal("Emit reported cancellation with active context")
	}
	msg := <-received
	if msg.Type != "token" || msg.Data != "hello" {
		t.Fatalf("unexpected message: %+v", msg)
	}
}
