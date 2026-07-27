package cy

import (
	"bytes"
	"testing"
	"time"

	"github.com/cfoust/cy/pkg/geom"
)

// watchForBell drains a client's output stream and closes the returned channel
// once a BEL byte (\a) is observed.
func watchForBell(client *Client) chan struct{} {
	found := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := client.Read(buf)
			if n > 0 && bytes.IndexByte(buf[:n], '\a') >= 0 {
				close(found)
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return found
}

// TestBellPropagation verifies that ringing the session propagates a BEL byte
// to the host terminal of every connected client.
func TestBellPropagation(t *testing.T) {
	server, create := setup(t)

	clientA := create(geom.DEFAULT_SIZE)
	clientB := create(geom.DEFAULT_SIZE)

	foundA := watchForBell(clientA)
	foundB := watchForBell(clientB)

	server.ringClients()

	for _, found := range []chan struct{}{foundA, foundB} {
		select {
		case <-found:
		case <-time.After(5 * time.Second):
			t.Fatal("did not observe BEL byte in client output")
		}
	}
}

// TestBellRingBellParam verifies that a client with :ring-bell disabled does
// not receive a BEL byte when the session is rung.
func TestBellRingBellParam(t *testing.T) {
	server, create := setup(t)

	client := create(geom.DEFAULT_SIZE)
	client.Params().SetRingBell(false)

	found := watchForBell(client)

	server.ringClients()

	select {
	case <-found:
		t.Fatal("client rang despite :ring-bell being false")
	case <-time.After(500 * time.Millisecond):
		// expected: no bell was propagated
	}
}
