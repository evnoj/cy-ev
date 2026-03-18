package tty

import (
	"bytes"
	"strconv"
	"sync"

	"github.com/cfoust/cy/pkg/emu"
	"github.com/cfoust/cy/pkg/geom"
	"github.com/cfoust/cy/pkg/geom/image"
)

// Pre-computed escape sequences for common operations
var (
	csiPrefix         = []byte("\033[")
	cursorHide        = []byte("\033[?25l")
	cursorShow        = []byte("\033[?12l\033[?25h")
	cursorVVisible    = []byte("\033[?12;25h")
	sgrReset          = []byte("\033(B\033[m")
	boldMode          = []byte("\033[1m")
	underlineMode     = []byte("\033[4m")
	underline4Colon   = []byte("\033[4:")
	strikeMode        = []byte("\033[9m")
	italicMode        = []byte("\033[3m")
	blinkMode         = []byte("\033[5m")
	strikeOff         = []byte("\033[29m")
	syncBegin         = []byte("\033[?2026h")
	syncEnd           = []byte("\033[?2026l")
	cursorStyleSuffix = []byte(" q")
	rgb48Prefix       = []byte("48;2;")
	rgb38Prefix       = []byte("38;2;")
	x256bgPrefix      = []byte("48;5;")
	x256fgPrefix      = []byte("38;5;")
	brightBgPrefix    = []byte("10")
	uc58Colon2        = []byte("58:2:")
	uc58Colon5        = []byte("58:5:")
)

var swapBufPool = sync.Pool{
	New: func() any {
		b := new(bytes.Buffer)
		b.Grow(4096)
		return b
	},
}

// writeInt writes a non-negative integer to the buffer without allocations.
func writeInt(buf *bytes.Buffer, n int) {
	buf.Write(strconv.AppendInt(buf.AvailableBuffer(), int64(n), 10))
}

// writeCursorAddress writes \033[row;colH directly.
func writeCursorAddress(buf *bytes.Buffer, row, col int) {
	buf.Write(csiPrefix)
	writeInt(buf, row+1)
	buf.WriteByte(';')
	writeInt(buf, col+1)
	buf.WriteByte('H')
}

// setColor writes the escape sequence for the given color directly
// to buf, avoiding allocations.
func setColor(
	buf *bytes.Buffer,
	color emu.Color,
	isBg bool,
) {
	if (!isBg && color == emu.DefaultFG) ||
		(isBg && color == emu.DefaultBG) {
		return
	}

	// Special case for reversed text when still set to default
	if isBg && color == emu.DefaultFG {
		color = emu.ANSIColor(15)
	} else if !isBg && color == emu.DefaultBG {
		color = emu.ANSIColor(0)
	}

	if r, g, b, ok := color.RGB(); ok {
		// \033[38;2;R;G;Bm or \033[48;2;R;G;Bm
		buf.Write(csiPrefix)
		if isBg {
			buf.Write(rgb48Prefix)
		} else {
			buf.Write(rgb38Prefix)
		}
		writeInt(buf, r)
		buf.WriteByte(';')
		writeInt(buf, g)
		buf.WriteByte(';')
		writeInt(buf, b)
		buf.WriteByte('m')
	} else if xterm, ok := color.XTerm(); ok {
		if xterm < 8 {
			// Standard ANSI: \033[3Xm / \033[4Xm
			buf.Write(csiPrefix)
			if isBg {
				buf.WriteByte('4')
			} else {
				buf.WriteByte('3')
			}
			buf.WriteByte(byte('0' + xterm))
			buf.WriteByte('m')
		} else if xterm < 16 {
			// Bright ANSI: \033[9Xm / \033[10Xm
			buf.Write(csiPrefix)
			if isBg {
				buf.Write(brightBgPrefix)
			} else {
				buf.WriteByte('9')
			}
			buf.WriteByte(byte('0' + xterm - 8))
			buf.WriteByte('m')
		} else {
			// 256-color: \033[38;5;Xm / \033[48;5;Xm
			buf.Write(csiPrefix)
			if isBg {
				buf.Write(x256bgPrefix)
			} else {
				buf.Write(x256fgPrefix)
			}
			writeInt(buf, xterm)
			buf.WriteByte('m')
		}
	}
}

// writeUnderlineColor writes the escape sequence for the given underline color.
func writeUnderlineColor(buf *bytes.Buffer, color emu.Color) {
	if r, g, b, ok := color.RGB(); ok {
		buf.Write(csiPrefix)
		buf.Write(uc58Colon2)
		writeInt(buf, r)
		buf.WriteByte(':')
		writeInt(buf, g)
		buf.WriteByte(':')
		writeInt(buf, b)
		buf.WriteByte('m')
	} else if xterm, ok := color.XTerm(); ok {
		buf.Write(csiPrefix)
		buf.Write(uc58Colon5)
		writeInt(buf, xterm)
		buf.WriteByte('m')
	}
}

// writeSwapImage writes the minimum escape sequence to transform dst into src
// into buf.
func writeSwapImage(buf *bytes.Buffer, dst, src image.Image) {
	buf.Write(cursorHide)

	max := geom.GetMaximum(dst.Size(), src.Size())

	for row := 0; row < max.R; row++ {
		for col := 0; col < max.C; col++ {
			dstCell := dst.Cell(col, row)
			srcCell := src.Cell(col, row)

			if dstCell.Equal(srcCell) {
				continue
			}

			writeCursorAddress(buf, row, col)

			mode := srcCell.Mode

			// note: emu.AttrReverse is handled virtually, since
			// it's just a color change

			if mode&emu.AttrBold != 0 {
				buf.Write(boldMode)
			}

			if mode&emu.AttrUnderline != 0 {
				style := srcCell.UnderlineStyle()
				if style <= 1 {
					buf.Write(underlineMode)
				} else {
					buf.Write(underline4Colon)
					buf.WriteByte('0' + style)
					buf.WriteByte('m')
				}
				if !srcCell.UnderlineColor.Default() {
					writeUnderlineColor(buf, srcCell.UnderlineColor)
				}
			}

			if mode&emu.AttrStrikethrough != 0 {
				buf.Write(strikeMode)
			}

			if mode&emu.AttrItalic != 0 {
				buf.Write(italicMode)
			}

			if mode&emu.AttrBlink != 0 {
				buf.Write(blinkMode)
			}

			setColor(buf, srcCell.FG, false)
			setColor(buf, srcCell.BG, true)

			buf.WriteRune(srcCell.Char)

			// TODO(cfoust): 08/07/24 why does ExitAttributeMode not cover this in alacritty?
			if mode&emu.AttrStrikethrough != 0 {
				buf.Write(strikeOff)
			}

			buf.Write(sgrReset)

			// CJK characters
			col += srcCell.Width() - 1
		}
	}

	buf.Write(cursorShow)
}

// swapImage calculates the minimum string to transform `src` into `dst`.
func swapImage(
	dst, src image.Image,
) []byte {
	buf := swapBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	writeSwapImage(buf, dst, src)
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	swapBufPool.Put(buf)
	return result
}

func Swap(
	dst, src *State,
) []byte {
	buf := swapBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	buf.Write(syncBegin)
	writeSwapImage(buf, dst.Image, src.Image)

	dstCursor := dst.Cursor
	srcCursor := src.Cursor

	// TODO(cfoust): 08/09/23 debug
	//if dstCursor.X != srcCursor.X || dstCursor.Y != srcCursor.Y {
	writeCursorAddress(buf, srcCursor.R, srcCursor.C)

	if dstCursor.Style != srcCursor.Style {
		buf.Write(csiPrefix)
		writeInt(buf, int(srcCursor.Style))
		buf.Write(cursorStyleSuffix)
	}

	// This is wasteful, we shouldn't have to include this on every frame
	if src.CursorVisible {
		buf.Write(cursorVVisible)
	} else {
		buf.Write(cursorHide)
	}

	buf.Write(syncEnd)

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	swapBufPool.Put(buf)
	return result
}
