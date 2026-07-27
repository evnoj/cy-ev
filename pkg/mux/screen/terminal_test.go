package screen

import (
	"context"
	"testing"
	"time"

	"github.com/cfoust/cy/pkg/emu"

	"github.com/stretchr/testify/require"
)

func TestTerminalBell(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	term := NewStaticTerminal(ctx, emu.New())
	sub := term.Subscribe(ctx)

	_, err := term.Write([]byte("\a"))
	require.NoError(t, err)

	select {
	case event := <-sub.Recv():
		_, ok := event.(BellEvent)
		require.True(t, ok, "expected a BellEvent, got %T", event)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for BellEvent")
	}
}
