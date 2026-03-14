package tty

import (
	"github.com/cfoust/cy/pkg/emu"
	"github.com/cfoust/cy/pkg/geom"
	"github.com/cfoust/cy/pkg/geom/image"
)

type State struct {
	Image         image.Image
	Cursor        emu.Cursor
	CursorVisible bool
}

func (s *State) Clone() *State {
	cloned := New(s.Image.Size())
	Copy(geom.Vec2{}, cloned, s)
	return cloned
}

func New(size geom.Vec2) *State {
	return &State{
		Image:         image.New(size),
		CursorVisible: true,
	}
}

// NewDirty creates a *State with all cells marked attrBlank so that the next
// Swap() call treats every cell as changed and emits a full redraw. Use this
// instead of New() after the physical terminal has been cleared (initial
// render or resize).
func NewDirty(size geom.Vec2) *State {
	state := New(size)
	for _, line := range state.Image {
		for i := range line {
			line[i].Mode |= emu.AttrBlank
		}
	}
	return state
}

func Copy(pos geom.Vec2, dst, src *State) {
	image.Copy(pos, dst.Image, src.Image)
	dst.Cursor = src.Cursor
	dst.Cursor.C += pos.C
	dst.Cursor.R += pos.R
	dst.CursorVisible = src.CursorVisible
}

func Capture(view emu.View) *State {
	cursor := view.Cursor()
	cursorVisible := view.CursorVisible()

	return &State{
		Image:         image.Capture(view),
		Cursor:        cursor,
		CursorVisible: cursorVisible,
	}
}
