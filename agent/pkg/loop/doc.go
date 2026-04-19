// Package loop provides an isolated, flexible agent message loop.
//
// The loop package is designed as a standalone component with no dependencies
// on other agent packages. It provides:
//
//   - Typed message dispatch via the Message interface
//   - Handler registration by message type
//   - Middleware chain for cross-cutting concerns (logging, metrics, auth)
//   - Priority handling for work items over regular messages
//   - Graceful shutdown with inbox draining
//   - Functional options for configuration
//
// Basic usage:
//
//	l := loop.New(
//	    loop.WithInboxSize(64),
//	    loop.WithWorkPriority(true),
//	)
//	l.Register("user.message", handleUserMessage)
//	l.Register("work.step", handleWorkStep)
//	l.Use(loggingMiddleware)
//	go l.Run(ctx)
//	l.Send(myMessage)
//
// Messages must implement the Message interface:
//
//	type MyMessage struct { ... }
//	func (m MyMessage) Type() string    { return "my.message" }
//	func (m MyMessage) Priority() int   { return 0 }
//
// Handlers can be registered as functions:
//
//	l.Register("my.message", func(ctx context.Context, msg loop.Message) error {
//	    m := msg.(MyMessage)
//	    // handle message
//	    return nil
//	})
//
// Middleware wraps handlers for cross-cutting concerns:
//
//	func loggingMiddleware(next loop.HandlerFunc) loop.HandlerFunc {
//	    return func(ctx context.Context, msg loop.Message) error {
//	        log.Printf("handling %s", msg.Type())
//	        return next(ctx, msg)
//	    }
//	}
package loop
