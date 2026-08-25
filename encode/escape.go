package encode
import (
	"bytes"
	"fmt"
	"unicode/utf8"
)
var hex = "0123456789abcdef"
func appendEscapedString(b *bytes.Buffer, s string, escapeHTML bool) {
	b.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c < utf8.RuneSelf && c != '\\' && c != '"' && (!escapeHTML || (c != '<' && c != '>' && c != '&')) {
			i++
			continue
		}
		if start < i {
			b.WriteString(s[start:i])
		}
		switch c {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteByte(c)
			i++
		case '\n':
			b.WriteString("\\n")
			i++
		case '\r':
			b.WriteString("\\r")
			i++
		case '\t':
			b.WriteString("\\t")
			i++
		case '\b':
			b.WriteString("\\b")
			i++
		case '\f':
			b.WriteString("\\f")
			i++
		default:
			if c < 0x20 || (escapeHTML && (c == '<' || c == '>' || c == '&')) {
				b.WriteString("\\u00")
				b.WriteByte(hex[c>>4])
				b.WriteByte(hex[c&0xf])
				i++
			} else {
				r, n := utf8.DecodeRuneInString(s[i:])
				if r == utf8.RuneError && n == 1 {
					b.WriteString("\\ufffd")
					i++
				} else if r == '\u2028' || r == '\u2029' {
					fmt.Fprintf(b, "\\u%04x", r)
					i += n
				} else {
					b.WriteString(s[i : i+n])
					i += n
				}
			}
		}
		start = i
	}
	if start < len(s) {
		b.WriteString(s[start:])
	}
	b.WriteByte('"')
}
func EscapeString(s string, escapeHTML bool) string {
	var b bytes.Buffer
	appendEscapedString(&b, s, escapeHTML)
	return b.String()
}
