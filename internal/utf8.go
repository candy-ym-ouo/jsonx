package internal
import (
	"strings"
	"unicode/utf8"
)
func ValidUTF8(data []byte) bool { return utf8.Valid(data) }
func ValidString(s string) bool  { return utf8.ValidString(s) }
// RepairUTF8 mirrors the usual JSON encoder policy: invalid input bytes are
// replaced with U+FFFD, never emitted verbatim into a JSON string.
func RepairUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for len(s) > 0 {
		r, n := utf8.DecodeRuneInString(s)
		if n == 0 {
			break
		}
		b.WriteRune(r)
		s = s[n:]
	}
	return b.String()
}
func RuneCount(data []byte) int { return utf8.RuneCount(data) }
func TruncateRunes(s string, count int) string {
	if count <= 0 {
		return ""
	}
	for i := range s {
		if count == 0 {
			return s[:i]
		}
		count--
	}
	return s
}
