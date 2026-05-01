package loop

// Option configures a Loop using the functional options pattern.
type Option func(*options)

type options struct {
	inboxSize     int
	workQueueSize int
	workPriority  bool
	onUnhandled   func(Message)
	onShutdown    func()
}

func defaultOptions() options {
	return options{
		inboxSize:     64,
		workQueueSize: 32,
		workPriority:  true,
	}
}

// WithInboxSize sets the inbox channel buffer size.
func WithInboxSize(size int) Option {
	return func(o *options) {
		if size > 0 {
			o.inboxSize = size
		}
	}
}

// WithWorkQueueSize sets the work queue channel buffer size.
func WithWorkQueueSize(size int) Option {
	return func(o *options) {
		if size > 0 {
			o.workQueueSize = size
		}
	}
}

// WithWorkPriority enables or disables work queue priority processing.
// When enabled, messages with Priority() > 0 are processed before regular messages.
func WithWorkPriority(enabled bool) Option {
	return func(o *options) {
		o.workPriority = enabled
	}
}

// WithUnhandledCallback sets a callback for messages with no registered handler.
func WithUnhandledCallback(fn func(Message)) Option {
	return func(o *options) {
		o.onUnhandled = fn
	}
}

// WithShutdownCallback sets a callback invoked when the loop shuts down.
func WithShutdownCallback(fn func()) Option {
	return func(o *options) {
		o.onShutdown = fn
	}
}
