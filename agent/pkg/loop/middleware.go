package loop

import "context"

// Middleware wraps a HandlerFunc to add cross-cutting behavior.
// Middleware is applied in the order registered, forming a chain.
type Middleware func(next HandlerFunc) HandlerFunc

// chain builds a handler chain from middleware and a final handler.
func chain(handler HandlerFunc, middleware ...Middleware) HandlerFunc {
	// Apply middleware in reverse order so the first registered runs first
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

// RecoveryMiddleware catches panics and converts them to errors.
func RecoveryMiddleware(next HandlerFunc) HandlerFunc {
	return func(ctx context.Context, msg Message) (err error) {
		defer func() {
			if r := recover(); r != nil {
				switch v := r.(type) {
				case error:
					err = v
				default:
					err = &PanicError{Value: r, MsgType: msg.Type()}
				}
			}
		}()
		return next(ctx, msg)
	}
}

// PanicError wraps a recovered panic value.
type PanicError struct {
	Value   any
	MsgType string
}

func (e *PanicError) Error() string {
	return "panic in handler for " + e.MsgType
}

// ObserverFunc is called when a message is processed.
type ObserverFunc func(msgType string, err error)

// ObserverMiddleware calls an observer function after each message is handled.
func ObserverMiddleware(observe ObserverFunc) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, msg Message) error {
			err := next(ctx, msg)
			observe(msg.Type(), err)
			return err
		}
	}
}
