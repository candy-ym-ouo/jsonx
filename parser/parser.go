package parser

import (
	"fmt"
	jerrors "jsonx/errors"
	"jsonx/lexer"
	"sync"
)

type Options struct {
	MaxDepth            int
	AllowComments       bool
	RejectDuplicateKeys bool
}

func ParseBatch(inputs [][]byte, opts Options) ([]*Value, []error) {
	values := make([]*Value, len(inputs))
	errs := make([]error, len(inputs))
	// One WaitGroup guards all goroutines. Add exactly len(inputs) so each
	// task has a matching Done, then wait once for every result. This is also
	// correct for an empty batch: Add(0) and Wait() return immediately, so the
	// caller always receives length-zero slices instead of a panic.
	var wg sync.WaitGroup
	wg.Add(len(inputs))
	for i, input := range inputs {
		go func(i int, input []byte) {
			defer wg.Done()
			values[i], errs[i] = Parse(input, opts)
		}(i, input)
	}
	wg.Wait()
	return values, errs
}

type Parser struct {
	lex  *lexer.Lexer
	cur  lexer.Token
	err  error
	opts Options
}

func Parse(data []byte, opts Options) (*Value, error) {
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 512
	}
	p := &Parser{lex: lexer.New(data, lexer.Options{AllowComments: opts.AllowComments}), opts: opts}
	if err := p.advance(); err != nil {
		return nil, err
	}
	v, err := p.value(0)
	if err != nil {
		return nil, err
	}
	if p.cur.Kind != lexer.EOF {
		return nil, jerrors.At(jerrors.ETrailingData, "trailing data after root value", p.cur.Pos, data)
	}
	v.raw = data
	return v, nil
}
func (p *Parser) advance() error {
	t, err := p.lex.Next()
	if err != nil {
		p.err = err
		return err
	}
	p.cur = t
	return nil
}
func (p *Parser) value(depth int) (*Value, error) {
	if depth > p.opts.MaxDepth {
		return nil, jerrors.At(jerrors.EMaxDepth, "maximum nesting depth exceeded", p.cur.Pos, p.lex.Data())
	}
	t := p.cur
	switch t.Kind {
	case lexer.String:
		if err := p.advance(); err != nil {
			return nil, err
		}
		v := NewString(string(t.Data))
		v.raw = t.Raw
		return v, nil
	case lexer.Number:
		if err := p.advance(); err != nil {
			return nil, err
		}
		v := NewNumber(string(t.Data))
		v.raw = t.Raw
		return v, nil
	case lexer.True:
		if err := p.advance(); err != nil {
			return nil, err
		}
		v := NewBool(true)
		v.raw = t.Raw
		return v, nil
	case lexer.False:
		if err := p.advance(); err != nil {
			return nil, err
		}
		v := NewBool(false)
		v.raw = t.Raw
		return v, nil
	case lexer.Null:
		if err := p.advance(); err != nil {
			return nil, err
		}
		v := NewNull()
		v.raw = t.Raw
		return v, nil
	case lexer.ArrayBegin:
		return p.array(depth + 1)
	case lexer.ObjectBegin:
		return p.object(depth + 1)
	default:
		return nil, p.unexpected("JSON value")
	}
}
func (p *Parser) array(depth int) (*Value, error) {
	start := p.cur.Pos.Offset
	if err := p.advance(); err != nil {
		return nil, err
	}
	items := make([]*Value, 0)
	if p.cur.Kind == lexer.ArrayEnd {
		end := p.cur.Pos.Offset + len(p.cur.Raw)
		if err := p.advance(); err != nil {
			return nil, err
		}
		v := NewArray(items)
		v.raw = p.lex.Data()[start:end]
		return v, nil
	}
	for {
		item, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		if p.cur.Kind == lexer.ArrayEnd {
			end := p.cur.Pos.Offset + len(p.cur.Raw)
			if err := p.advance(); err != nil {
				return nil, err
			}
			v := NewArray(items)
			v.raw = p.lex.Data()[start:end]
			return v, nil
		}
		if p.cur.Kind != lexer.Comma {
			return nil, p.unexpected(", or ]")
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.Kind == lexer.ArrayEnd {
			return nil, p.unexpected("JSON value")
		}
	}
}
func (p *Parser) object(depth int) (*Value, error) {
	start := p.cur.Pos.Offset
	if err := p.advance(); err != nil {
		return nil, err
	}
	members := make([]Member, 0)
	seen := map[string]struct{}{}
	if p.cur.Kind == lexer.ObjectEnd {
		end := p.cur.Pos.Offset + len(p.cur.Raw)
		if err := p.advance(); err != nil {
			return nil, err
		}
		v := NewObject(members)
		v.raw = p.lex.Data()[start:end]
		return v, nil
	}
	for {
		if p.cur.Kind != lexer.String {
			return nil, jerrors.At(jerrors.EExpectedKey, "object key must be a string", p.cur.Pos, p.lex.Data())
		}
		key, keyPos := string(p.cur.Data), p.cur.Pos
		if p.opts.RejectDuplicateKeys {
			if _, ok := seen[key]; ok {
				return nil, jerrors.At(jerrors.EDuplicateKey, fmt.Sprintf("duplicate key %q", key), keyPos, p.lex.Data())
			}
		}
		seen[key] = struct{}{}
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.Kind != lexer.Colon {
			return nil, p.unexpected(":")
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		item, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		members = append(members, Member{Key: key, Value: item})
		if p.cur.Kind == lexer.ObjectEnd {
			end := p.cur.Pos.Offset + len(p.cur.Raw)
			if err := p.advance(); err != nil {
				return nil, err
			}
			v := NewObject(members)
			v.raw = p.lex.Data()[start:end]
			return v, nil
		}
		if p.cur.Kind != lexer.Comma {
			return nil, p.unexpected(", or }")
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.cur.Kind == lexer.ObjectEnd {
			return nil, p.unexpected("object key")
		}
	}
}
func (p *Parser) unexpected(expected string) error {
	err := jerrors.At(jerrors.EUnexpectedToken, "unexpected token", p.cur.Pos, p.lex.Data())
	err.Expected, err.Got = expected, p.cur.Kind.String()
	return err
}
func Stats(v *Value) (nodes, depth int) {
	var walk func(*Value, int)
	walk = func(x *Value, d int) {
		nodes++
		if d > depth {
			depth = d
		}
		if x == nil {
			return
		}
		for _, child := range x.list {
			walk(child, d+1)
		}
		for _, member := range x.obj {
			walk(member.Value, d+1)
		}
	}
	walk(v, 1)
	return
}
