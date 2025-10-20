package orchestrator

// OptionFunc defines a type for Orchestrator options
type OptionFunc[T any] func(*ctx[T])

// WithIdempotenceKey sets a stable, user-provided key to enable idempotent retries.
func WithIdempotenceKey(key string) OptionFunc[string] {
	return func(o *ctx[string]) {
		o.idempotencyKey = key
	}
}

// WithStore wires in a persistence layer for saga state.
// Pass nil to run entirely in-memory (e.g., for testing or fire-and-forget workflows).
// The store must be safe for concurrent use if the orchestrator is shared across goroutines.
func WithStore[T any](store Store) OptionFunc[T] {
	return func(o *ctx[T]) {
		o.store = store
	}
}

// WithLogger injects a structured logger.
// If omitted, a no-op logger is used (no output). Avoids forcing a logging framework on users.
func WithLogger[T any](logger Logger) OptionFunc[T] {
	return func(o *ctx[T]) {
		o.logger = logger
	}
}

// WithRetry configures retry behavior for the entire saga.
// Passing nil disables retries (default). Policy is applied per-step during forward execution.
func WithRetry[T any](policy *RetryPolicy) OptionFunc[T] {
	return func(o *ctx[T]) {
		if policy == nil {
			o.retryPolicy = nil
			return
		}
		o.retryPolicy = policy
	}
}
