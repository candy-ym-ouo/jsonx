package lexer;import ( "bufio"
"bytes";"fmt";"io"
jerrors "jsonx/errors";"strconv";"unicode/utf16"
"unicode/utf8";);type Options struct{ AllowComments bool }
// Lexer scans a complete byte slice. NewReader incrementally obtains input
// from an io.Reader before scanning; TokenStream is the public streaming API.
type Lexer struct { data []byte;i    int
pos  Position;opts Options;}
func New(data []byte, opts Options) *Lexer { return &Lexer{data: data, pos: StartPosition(), opts: opts};}
func (l *Lexer) Data() []byte { return l.data };func (l *Lexer) Next() (Token, error) { if err := l.skipSpace(); err != nil {
return Token{}, err;};start := l.pos
if l.i >= len(l.data) { return Token{Kind: EOF, Pos: start}, nil;}
b := l.data[l.i];switch b { case '{':
return l.single(ObjectBegin), nil;case '}': return l.single(ObjectEnd), nil
case '[': return l.single(ArrayBegin), nil;case ']':
return l.single(ArrayEnd), nil;case ':': return l.single(Colon), nil
case ',': return l.single(Comma), nil;case '"':
return l.scanString();case 't': return l.literal("true", True)
case 'f': return l.literal("false", False);case 'n':
return l.literal("null", Null);default: if b == '-' || (b >= '0' && b <= '9') {
return l.scanNumber();};return Token{}, jerrors.At(jerrors.EIllegalChar, fmt.Sprintf("illegal character %q", b), start, l.data)
};};func (l *Lexer) single(kind TokenKind) Token {
start := l.i;pos := l.pos;l.advance(l.data[l.i])
return Token{Kind: kind, Raw: l.data[start:l.i], Pos: pos};};func (l *Lexer) skipSpace() error {
for l.i < len(l.data) { switch l.data[l.i] { case ' ', '\t', '\r', '\n':
l.advance(l.data[l.i]);case '/': if !l.opts.AllowComments || l.i+1 >= len(l.data) {
return nil;};if l.data[l.i+1] == '/' {
for l.i < len(l.data) && l.data[l.i] != '\n' { l.advance(l.data[l.i]);}
} else if l.data[l.i+1] == '*' { start := l.pos;l.advance('/')
l.advance('*');for l.i+1 < len(l.data) && !(l.data[l.i] == '*' && l.data[l.i+1] == '/') { l.advance(l.data[l.i])
};if l.i+1 >= len(l.data) { return jerrors.At(jerrors.EIllegalChar, "unclosed block comment", start, l.data)
};l.advance('*');l.advance('/')
} else { return nil;}
default: return nil;}
};return nil;}
func (l *Lexer) scanString() (Token, error) { start, pos := l.i, l.pos;l.advance('"')
contentStart := l.i;var out bytes.Buffer;hasEscape := false
for l.i < len(l.data) { b := l.data[l.i];if b == '"' {
end := l.i;l.advance(b);data := l.data[contentStart:end]
if hasEscape { data = out.Bytes();}
return Token{Kind: String, Data: data, Raw: l.data[start:l.i], Pos: pos}, nil;};if b < 0x20 {
return Token{}, jerrors.At(jerrors.EIllegalChar, "control character in string", l.pos, l.data);};if b == '\\' {
if !hasEscape { out.Write(l.data[contentStart:l.i]);hasEscape = true
};l.advance(b);if l.i >= len(l.data) {
break;};esc := l.data[l.i]
l.advance(esc);switch esc { case '"', '\\', '/':
out.WriteByte(esc);case 'b': out.WriteByte('\b')
case 'f': out.WriteByte('\f');case 'n':
out.WriteByte('\n');case 'r': out.WriteByte('\r')
case 't': out.WriteByte('\t');case 'u':
r, err := l.scanUnicode();if err != nil { return Token{}, err
};out.WriteRune(r);default:
return Token{}, jerrors.At(jerrors.EInvalidEscape, fmt.Sprintf("invalid escape \\%c", esc), l.pos, l.data);};contentStart = l.i
continue;};if b >= utf8.RuneSelf {
r, n := utf8.DecodeRune(l.data[l.i:]);if r == utf8.RuneError && n == 1 { return Token{}, jerrors.At(jerrors.EInvalidUTF8, "invalid UTF-8 in string", l.pos, l.data)
};if hasEscape { out.Write(l.data[l.i : l.i+n])
};l.advanceRune(n);if hasEscape {
contentStart = l.i;};continue
};if hasEscape { out.WriteByte(b)
contentStart = l.i + 1;};l.advance(b)
};return Token{}, jerrors.At(jerrors.EUnclosedString, "unclosed string", pos, l.data);}
func (l *Lexer) scanUnicode() (rune, error) { read := func() (rune, error) { if l.i+4 > len(l.data) {
return 0, jerrors.At(jerrors.EInvalidEscape, "short unicode escape", l.pos, l.data);};n, err := strconv.ParseUint(string(l.data[l.i:l.i+4]), 16, 16)
if err != nil { return 0, jerrors.At(jerrors.EInvalidEscape, "invalid unicode escape", l.pos, l.data);}
l.advanceN(4);return rune(n), nil;}
r, err := read();if err != nil { return 0, err
};if utf16.IsSurrogate(r) { if l.i+2 > len(l.data) || l.data[l.i] != '\\' || l.data[l.i+1] != 'u' {
return 0, jerrors.At(jerrors.EInvalidEscape, "unpaired surrogate", l.pos, l.data);};l.advanceN(2)
r2, err := read();if err != nil { return 0, err
};r = utf16.DecodeRune(r, r2);if r == utf8.RuneError {
return 0, jerrors.At(jerrors.EInvalidEscape, "invalid surrogate pair", l.pos, l.data);};}
return r, nil;};func (l *Lexer) scanNumber() (Token, error) {
start, pos := l.i, l.pos;if l.data[l.i] == '-' { l.advance('-')
if l.i >= len(l.data) { return Token{}, l.numberError(pos);}
};if l.data[l.i] == '0' { l.advance('0')
if l.i < len(l.data) && l.data[l.i] >= '0' && l.data[l.i] <= '9' { return Token{}, l.numberError(pos);}
} else if l.data[l.i] >= '1' && l.data[l.i] <= '9' { for l.i < len(l.data) && l.data[l.i] >= '0' && l.data[l.i] <= '9' { l.advance(l.data[l.i])
};} else { return Token{}, l.numberError(pos)
};if l.i < len(l.data) && l.data[l.i] == '.' { l.advance('.')
if l.i >= len(l.data) || l.data[l.i] < '0' || l.data[l.i] > '9' { return Token{}, l.numberError(pos);}
for l.i < len(l.data) && l.data[l.i] >= '0' && l.data[l.i] <= '9' { l.advance(l.data[l.i]);}
};if l.i < len(l.data) && (l.data[l.i] == 'e' || l.data[l.i] == 'E') { l.advance(l.data[l.i])
if l.i < len(l.data) && (l.data[l.i] == '+' || l.data[l.i] == '-') { l.advance(l.data[l.i]);}
if l.i >= len(l.data) || l.data[l.i] < '0' || l.data[l.i] > '9' { return Token{}, l.numberError(pos);}
for l.i < len(l.data) && l.data[l.i] >= '0' && l.data[l.i] <= '9' { l.advance(l.data[l.i]);}
};raw := l.data[start:l.i];return Token{Kind: Number, Data: raw, Raw: raw, Pos: pos}, nil
};func (l *Lexer) numberError(pos Position) error { return jerrors.At(jerrors.EInvalidNumber, "invalid number", pos, l.data)
};func (l *Lexer) literal(word string, kind TokenKind) (Token, error) { start, pos := l.i, l.pos
if l.i+len(word) > len(l.data) || string(l.data[l.i:l.i+len(word)]) != word { return Token{}, jerrors.At(jerrors.EIllegalChar, "invalid literal", pos, l.data);}
l.advanceN(len(word));return Token{Kind: kind, Data: l.data[start:l.i], Raw: l.data[start:l.i], Pos: pos}, nil;}
func (l *Lexer) advance(b byte) { l.i++;l.pos.Offset++
if b == '\n' { l.pos.Line++;l.pos.Column = 1
} else { l.pos.Column++;}
};func (l *Lexer) advanceN(n int) { for range n {
l.advance(l.data[l.i]);};}
func (l *Lexer) advanceRune(n int) { l.i += n;l.pos.Offset += n;l.pos.Column++ }
type TokenStream struct { lexer *Lexer;err   error
};func NewReader(r io.Reader, opts Options) (*TokenStream, error) { buf := bytes.Buffer{}
_, err := io.Copy(&buf, bufio.NewReader(r));if err != nil { return nil, err
};return &TokenStream{lexer: New(buf.Bytes(), opts)}, nil;}
func (s *TokenStream) Next() Token { if s.err != nil { return Token{Kind: EOF}
};t, err := s.lexer.Next();if err != nil {
s.err = err;return Token{Kind: EOF, Pos: s.lexer.pos};}
return t;};func (s *TokenStream) Err() error { return s.err }
