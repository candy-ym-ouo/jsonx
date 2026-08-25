package encode
import "bytes"
type layout struct{ prefix, indent string }
func (l layout) newline(b *bytes.Buffer, depth int) {
	if l.indent == "" {
		return
	}
	b.WriteByte('\n')
	b.WriteString(l.prefix)
	for range depth {
		b.WriteString(l.indent)
	}
}
// Compact removes insignificant JSON whitespace without interpreting values.
// It is used only for already validated encoder output.
func Compact(src []byte) []byte {
	out := make([]byte, 0, len(src))
	inString, escaped := false, false
	for _, c := range src {
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			out = append(out, c)
		}
	}
	return out
}
