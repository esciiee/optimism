package opio

import (
	"context"
)

// Newtyping empty struct prevents collision with other empty struct keys in the Context.
type interruptWaiterContextKeyType struct{}

var interruptWaiterContextKey = interruptWaiterContextKeyType{}

// WithInterruptWaiter overrides the interrupt waiter value, e.g. to insert a function that mocks
// interrupt signals for testing CLI shutdown without actual process signals.
func WithInterruptWaiterFunc(ctx context.Context, fn InterruptWaiterFunc) context.Context {
	return withInterruptWaiter(ctx, fn)
}

func withInterruptWaiter(ctx context.Context, value InterruptWaiter) context.Context {
	return context.WithValue(ctx, interruptWaiterContextKey, value)
}

// contextInterruptWaiter returns a interruptWaiter that blocks on interrupts when called.
func contextInterruptWaiter(ctx context.Context) InterruptWaiter {
	v := ctx.Value(interruptWaiterContextKey)
	if v == nil {
		return nil
	}
	return v.(InterruptWaiter)
}
