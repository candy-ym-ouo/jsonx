// Package jsonx implements JSON parsing, validation, decoding and encoding
// without delegating its codec to encoding/json.
package jsonx

import (
	"io"
	"jsonx/decode"
	"jsonx/encode"
	jerrors "jsonx/errors"
	"jsonx/lexer"
	"jsonx/parser"
	"reflect"
)

func parserOptions(o Options) parser.Options {
	return parser.Options{MaxDepth: o.MaxDepth, AllowComments: o.AllowComments, RejectDuplicateKeys: o.RejectDuplicateKeys}
}
func decodeOptions(o Options) decode.Options {
	return decode.Options{DisallowUnknown: o.DisallowUnknown, RequireFields: o.RequireFields, NoCoerce: o.NoCoerce, NumberAsFloat64: o.NumberAsFloat64}
}
func encodeOptions(o Options) encode.Options {
	return encode.Options{EscapeHTML: o.EscapeHTML, SortKeys: o.SortKeys, MaxDepth: o.MaxDepth}
}
func Parse(data []byte, options ...Option) (*parser.Value, error) {
	o := resolve(options)
	return parser.Parse(data, parserOptions(o))
}
func ParseAny(data []byte, options ...Option) (any, error) {
	o := resolve(options)
	v, err := parser.Parse(data, parserOptions(o))
	if err != nil {
		return nil, err
	}
	return v.Any(o.NumberAsFloat64), nil
}
func Tokenize(r io.Reader, options ...Option) (*lexer.TokenStream, error) {
	o := resolve(options)
	return lexer.NewReader(r, lexer.Options{AllowComments: o.AllowComments})
}
func NewStreamReader(r io.Reader, options ...Option) *parser.StreamReader {
	o := resolve(options)
	return parser.NewStreamReader(r, parserOptions(o))
}
func Decode(data []byte, dst any, options ...Option) error {
	o := resolve(options)
	v, err := parser.Parse(data, parserOptions(o))
	if err != nil {
		return err
	}
	return decode.Decode(v, dst, decodeOptions(o))
}
func DecodeStrict(data []byte, dst any, options ...Option) error {
	base := []Option{DisallowUnknown(true), RequireFields(true), NoCoerce(true)}
	base = append(base, options...)
	return Decode(data, dst, base...)
}
func DecodeValue(value *parser.Value, dst any, options ...Option) error {
	o := resolve(options)
	return decode.Decode(value, dst, decodeOptions(o))
}
func Marshal(value any, options ...Option) ([]byte, error) {
	o := resolve(options)
	rv := reflect.ValueOf(value)
	if rv.IsValid() && rv.Kind() == reflect.Pointer && rv.IsNil() {
		if _, ok := value.(encode.Marshaler); ok {
			// Typed nil pointer implementing Marshaler: never invoke the
			// method on a nil receiver — it would dereference nil fields and
			// panic. Encode as null, mirroring encoding/json. Non-nil custom
			// Marshalers still flow through encode.Marshal below, preserving
			// their return-value and error-wrapping behavior unchanged.
			return []byte("null"), nil
		}
	}
	return encode.Marshal(value, encodeOptions(o))
}
func MarshalIndent(value any, prefix, indent string, options ...Option) ([]byte, error) {
	o := resolve(options)
	eo := encodeOptions(o)
	eo.Prefix, eo.Indent = prefix, indent
	return encode.Marshal(value, eo)
}
func Encode(w io.Writer, value any, options ...Option) error {
	o := resolve(options)
	return encode.Encode(w, value, encodeOptions(o))
}
func Validate(data []byte, options ...Option) error { _, err := Parse(data, options...); return err }

type CompiledSchema struct {
	schema  *decode.Schema
	options []Option
}

func CompileSchema(schemaData []byte, options ...Option) (*CompiledSchema, error) {
	v, err := Parse(schemaData, options...)
	if err != nil {
		return nil, err
	}
	s, err := decode.CompileSchema(v)
	if err != nil {
		return nil, err
	}
	return &CompiledSchema{schema: s, options: options}, nil
}
func (s *CompiledSchema) Validate(data []byte) error {
	v, err := Parse(data, s.options...)
	if err != nil {
		return err
	}
	return s.schema.Validate(v)
}
func ValidateSchema(data []byte, schema any, options ...Option) error {
	var compiled *decode.Schema
	switch x := schema.(type) {
	case []byte:
		v, err := Parse(x, options...)
		if err != nil {
			return err
		}
		compiled, err = decode.CompileSchema(v)
		if err != nil {
			return err
		}
	case string:
		v, err := Parse([]byte(x), options...)
		if err != nil {
			return err
		}
		compiled, err = decode.CompileSchema(v)
		if err != nil {
			return err
		}
	case *parser.Value:
		var err error
		compiled, err = decode.CompileSchema(x)
		if err != nil {
			return err
		}
	case *CompiledSchema:
		compiled = x.schema
	default:
		return jerrors.New(jerrors.ESchemaType, "schema must be []byte, string, Value, or CompiledSchema")
	}
	v, err := Parse(data, options...)
	if err != nil {
		return err
	}
	return compiled.Validate(v)
}
func Format(data []byte, indent string, options ...Option) ([]byte, error) {
	o := resolve(options)
	v, err := parser.Parse(data, parserOptions(o))
	if err != nil {
		return nil, err
	}
	eo := encodeOptions(o)
	eo.Indent = indent
	return encode.Marshal(v, eo)
}
func PathGet(data []byte, pathText string, options ...Option) (*parser.Value, error) {
	v, err := Parse(data, options...)
	if err != nil {
		return nil, err
	}
	path, err := jerrors.ParsePath(pathText)
	if err != nil {
		return nil, err
	}
	current := v
	for _, step := range path.Steps() {
		if step.IsKey {
			var ok bool
			current, ok = current.Lookup(step.Key)
			if !ok {
				return nil, jerrors.OnPath(jerrors.EInvalidPath, "object key not found", path)
			}
		} else {
			if current.Kind() != parser.Array || step.Index < 0 || step.Index >= current.Len() {
				return nil, jerrors.OnPath(jerrors.EInvalidPath, "array index out of range", path)
			}
			current = current.Index(step.Index)
		}
	}
	return current, nil
}
