//go:build linux || darwin || dragonfly || solaris || openbsd || netbsd || freebsd
// +build linux darwin dragonfly solaris openbsd netbsd freebsd

package emu

import (
	"bytes"

	"github.com/cfoust/cy/pkg/geom"
)

type terminal struct {
	*State
}

func newTerminal(info TerminalInfo) *terminal {
	t := &terminal{newState(info.w)}
	t.init(geom.Size{C: info.cols, R: info.rows})
	t.disableHistory = info.disableHistory
	t.historyLimit = info.historyLimit
	return t
}

func (t *terminal) init(size geom.Size) {
	t.cur.Attr.FG = DefaultFG
	t.cur.Attr.BG = DefaultBG
	t.Resize(size)
	t.reset()
}

// Write parses input and writes terminal changes to state.
func (t *terminal) Parse(p []byte) (written int) {
	t.dirty.writeId++

	// Fast path: no colons and not mid-sequence — forward directly.
	if t.subParamState == subParamGround &&
		bytes.IndexByte(p, ':') == -1 {
		for _, b := range p {
			t.parser.Advance(b)
			written++
		}
		return
	}

	// Slow path: strip colon sub-params before forwarding.
	for _, b := range p {
		if t.advanceSubParam(b) {
			t.parser.Advance(b)
		}
		written++
	}
	return
}

func (t *terminal) Write(p []byte) (int, error) {
	t.Lock()
	w := t.Parse(p)
	t.Unlock()
	return w, nil
}

func (t *terminal) WriteSync(p []byte) (int, bool, error) {
	t.Lock()
	w := t.Parse(p)
	syncing := t.mode&ModeSyncUpdate != 0
	t.Unlock()
	return w, syncing, nil
}

func (t *terminal) Resize(size geom.Vec2) {
	t.Lock()
	defer t.Unlock()
	t.resize(size)
}
