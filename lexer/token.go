package lexer
import "fmt"
// TokenKind identifies one lexical JSON item.
type TokenKind uint8
const (
	Invalid TokenKind = iota
	ObjectBegin
	ObjectEnd
	ArrayBegin
	ArrayEnd
	Colon
	Comma
	String
	Number
	True
	False
	Null
	EOF
)
var kindNames = [...]string{
	"invalid", "{", "}", "[", "]", ":", ",", "string", "number",
	"true", "false", "null", "EOF",
}
func (k TokenKind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("TokenKind(%d)", k)
}
// Token carries decoded data and the source range. For strings Data is the
// decoded content; Raw preserves the exact source spelling.
type Token struct {
	Kind TokenKind
	Data []byte
	Raw  []byte
	Pos  Position
}
func (t Token) String() string {
	if len(t.Raw) != 0 {
		return string(t.Raw)
	}
	return t.Kind.String()
}
