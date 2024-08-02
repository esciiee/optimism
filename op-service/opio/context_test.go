package opio

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestInterruptSignalIsUnique(t *testing.T) {
	ass := require.New(t)
	ctx := context.Background()
	ass.Nil(ctx.Value(interruptWaiterContextKey))
	ctx = context.WithValue(ctx, interruptWaiterContextKey, 1)
	ass.Equal(ctx.Value(interruptWaiterContextKey), 1)
	ctx = context.WithValue(ctx, interruptWaiterContextKey, 2)
	ass.Equal(ctx.Value(interruptWaiterContextKey), 2)
	ass.Nil(ctx.Value(struct{}{}))
}
