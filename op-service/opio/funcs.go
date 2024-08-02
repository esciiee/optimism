package opio

import (
	"context"
)

// WaitForInterrupt blocks until an interrupt is received, defaulting to interrupting on the default
// signals if no interrupt blocker is present in the Context. Returns nil if an interrupt occurs,
// else the Context error when it's done.
func WaitForInterrupt(ctx context.Context) error {
	iw := contextInterruptWaiter(ctx)
	if iw == nil {
		catcher := newSignalInterrupter()
		defer catcher.Stop()
		iw = catcher
	}
	return iw.waitForInterrupt(ctx).CtxError
}

// WithSignalInterruptWaiter attaches an interrupt signal handler to the context which continues to receive
// signals after every block, and also prevents the interrupt signals being handled before we're
// ready to wait for them. This helps functions block on individual consecutive interrupts.
func WithSignalInterruptWaiter(ctx context.Context) (_ context.Context, stop func()) {
	if ctx.Value(interruptWaiterContextKey) != nil { // already has an interrupt waiter
		return ctx, func() {}
	}
	catcher := newSignalInterrupter()
	return withInterruptWaiter(ctx, catcher), catcher.Stop
}

// Returns a Context with a signal interrupt blocker and leaks the destructor. Intended for use in
// main functions where we exit right after using the returned context anyway.
func WithSignalInterruptMain(ctx context.Context) context.Context {
	catcher := newSignalInterrupter()
	return withInterruptWaiter(ctx, catcher)
}

// WithCancelOnInterrupt returns a Context that is cancelled when WaitForInterrupt returns on the
// InterruptWaiter in ctx. If there's no InterruptWaiter, the default interrupt signals are used: In
// this case the signal hooking is not stopped until the original ctx is cancelled.
func WithCancelOnInterrupt(ctx context.Context) context.Context {
	interruptWaiter := contextInterruptWaiter(ctx)
	ctx, cancel := context.WithCancelCause(ctx)
	stop := func() {}
	if interruptWaiter == nil {
		catcher := newSignalInterrupter()
		stop = catcher.Stop
		interruptWaiter = catcher
	}
	go func() {
		defer stop()
		cancel(interruptWaiter.waitForInterrupt(ctx).Cause())
	}()
	return ctx
}
