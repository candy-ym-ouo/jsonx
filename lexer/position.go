package lexer
import jerrors "jsonx/errors"
// Position is a location in the input. Line and Column are one-based while
// Offset is a zero-based byte offset.
type Position = jerrors.Position
func StartPosition() Position { return Position{Line: 1, Column: 1} }
func PositionAt(data []byte, offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(data) {
		offset = len(data)
	}
	p := StartPosition()
	for i := 0; i < offset; {
		b := data[i]
		if b == '\n' {
			p.Line++
			p.Column = 1
			i++
			continue
		}
		p.Column++
		if b < 0x80 {
			i++
		} else {
			n := runeLen(b)
			if n < 1 || i+n > offset {
				n = 1
			}
			i += n
		}
	}
	p.Offset = offset
	return p
}
func runeLen(b byte) int {
	switch {
	case b < 0x80:
		return 1
	case b&0xe0 == 0xc0:
		return 2
	case b&0xf0 == 0xe0:
		return 3
	case b&0xf8 == 0xf0:
		return 4
	default:
		return 1
	}
}
