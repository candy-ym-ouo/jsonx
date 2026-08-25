package errors
import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)
type pathStep struct {
	key   string
	index int
	isKey bool
}
// Path is a small JSONPath subset: root, object keys, and array indexes.
type Path struct{ steps []pathStep }
func RootPath() Path { return Path{steps: []pathStep{}} }
func (p Path) Clone() Path {
	out := Path{steps: make([]pathStep, len(p.steps))}
	copy(out.steps, p.steps)
	return out
}
func (p *Path) AppendKey(key string)  { p.steps = append(p.steps, pathStep{key: key, isKey: true}) }
func (p *Path) AppendIndex(index int) { p.steps = append(p.steps, pathStep{index: index}) }
func (p *Path) Pop() {
	if len(p.steps) > 0 {
		p.steps = p.steps[:len(p.steps)-1]
	}
}
func (p Path) Len() int { return len(p.steps) }
func (p Path) String() string {
	var b strings.Builder
	b.WriteByte('$')
	for _, s := range p.steps {
		if !s.isKey {
			fmt.Fprintf(&b, "[%d]", s.index)
			continue
		}
		if simpleKey(s.key) {
			b.WriteByte('.')
			b.WriteString(s.key)
		} else {
			b.WriteString("['")
			b.WriteString(strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(s.key))
			b.WriteString("']")
		}
	}
	return b.String()
}
func simpleKey(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(unicode.IsLetter(r) || r == '_' || (i > 0 && unicode.IsDigit(r))) {
			return false
		}
	}
	return true
}
type Step struct {
	Key   string
	Index int
	IsKey bool
}
func (p Path) Steps() []Step {
	out := make([]Step, len(p.steps))
	for i, s := range p.steps {
		out[i] = Step{Key: s.key, Index: s.index, IsKey: s.isKey}
	}
	return out
}
func ParsePath(text string) (Path, error) {
	p := RootPath()
	if text == "" || text[0] != '$' {
		return p, New(EInvalidPath, "path must start with $")
	}
	for i := 1; i < len(text); {
		switch text[i] {
		case '.':
			i++
			start := i
			for i < len(text) && text[i] != '.' && text[i] != '[' {
				i++
			}
			if start == i {
				return p, New(EInvalidPath, "empty path key")
			}
			p.AppendKey(text[start:i])
		case '[':
			i++
			if i < len(text) && text[i] == '\'' {
				i++
				var key strings.Builder
				for i < len(text) && text[i] != '\'' {
					if text[i] == '\\' {
						i++
						if i >= len(text) || (text[i] != '\\' && text[i] != '\'') {
							return p, New(EInvalidPath, "invalid quoted key escape")
						}
					}
					key.WriteByte(text[i])
					i++
				}
				if i+1 >= len(text) || text[i] != '\'' || text[i+1] != ']' {
					return p, New(EInvalidPath, "unclosed quoted key")
				}
				p.AppendKey(key.String())
				i += 2
				continue
			}
			start := i
			for i < len(text) && text[i] >= '0' && text[i] <= '9' {
				i++
			}
			if start == i || i >= len(text) || text[i] != ']' {
				return p, New(EInvalidPath, "invalid array index")
			}
			n, err := strconv.Atoi(text[start:i])
			if err != nil {
				return p, Wrap(EInvalidPath, "invalid array index", err)
			}
			p.AppendIndex(n)
			i++
		default:
			return p, New(EInvalidPath, "expected . or [")
		}
	}
	return p, nil
}
