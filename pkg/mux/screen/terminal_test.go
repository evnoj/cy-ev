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

func TestTerminalTitle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	term := NewStaticTerminal(ctx, emu.New())
	sub := term.Subscribe(ctx)

	// OSC 2 sets the window title, terminated by ST (ESC \).
	_, err := term.Write([]byte("\x1b]2;some-title\x1b\\"))
	require.NoError(t, err)

	select {
	case event := <-sub.Recv():
		title, ok := event.(TitleEvent)
		require.True(t, ok, "expected a TitleEvent, got %T", event)
		require.Equal(t, "some-title", title.Title)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for TitleEvent")
	}

	// Writing the same title again must not publish another TitleEvent.
	_, err = term.Write([]byte("\x1b]2;some-title\x1b\\"))
	require.NoError(t, err)

	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case event := <-sub.Recv():
			_, ok := event.(TitleEvent)
			require.False(
				t,
				ok,
				"unexpected TitleEvent for unchanged title",
			)
		case <-timeout:
			return
		}
	}
}
