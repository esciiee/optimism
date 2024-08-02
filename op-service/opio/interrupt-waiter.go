package opio

import (
	"context"
	"fmt"
)

type InterruptWaiter interface {
	waitForInterrupt(ctx context.Context) waitInterruptResult
}

// ctxErr should be the context.Cause of ctx when it is done. interrupt is only inspected if ctxErr
// is nil, and is not required to be set.
type InterruptWaiterFunc func(ctx context.Context) (interrupt, ctxErr error)

func (me InterruptWaiterFunc) waitForInterrupt(ctx context.Context) (res waitInterruptResult) {
	res.Interrupt, res.CtxError = me(ctx)
	return
}

type waitInterruptResult struct {
	// If CtxError is nil, interrupt occurred but this doesn't have to be any particular value.
	Interrupt error
	// If not nil, Context completion caused us to stop waiting.
	CtxError error
}

func (me waitInterruptResult) Cause() error {
	if me.CtxError != nil {
		return me.CtxError
	}
	if me.Interrupt != nil {
		// Do we really need to wrap the interrupt?
		return fmt.Errorf("interrupted: %w", me.Interrupt)
	}
	return nil
}
