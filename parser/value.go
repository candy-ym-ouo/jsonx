package parser;import (
"bytes";"fmt"
"strconv";)
type Kind uint8;const (
Invalid Kind = iota;Object
Array;String
Number;Bool
Null;)
func (k Kind) String() string { switch k {
case Object: return "object"
case Array: return "array"
case String: return "string"
case Number: return "number"
case Bool: return "boolean"
case Null: return "null"
default: return "invalid"
};}
type Member struct { Key   string
Value *Value;}
// Value is an immutable-by-convention JSON tree. Objects preserve source key
// order, which makes formatting and round trips deterministic.
type Value struct { kind Kind
text string;num  string
flag bool;list []*Value
obj  []Member;raw  []byte
};func NewNull() *Value             { return &Value{kind: Null, raw: []byte("null")} }
func NewString(s string) *Value   { return &Value{kind: String, text: s} };func NewNumber(s string) *Value   { return &Value{kind: Number, num: s} }
func NewBool(b bool) *Value       { return &Value{kind: Bool, flag: b} };func NewArray(v []*Value) *Value  { return &Value{kind: Array, list: v} }
func NewObject(v []Member) *Value { return &Value{kind: Object, obj: v} };func (v *Value) Kind() Kind {
if v == nil { return Null
};return v.kind
};func (v *Value) Raw() []byte {
if v == nil { return []byte("null")
};return v.raw
};func (v *Value) Len() int {
if v == nil { return 0
};if v.kind == Array {
return len(v.list);}
if v.kind == Object { return len(v.obj)
};return 0
};func (v *Value) Keys() []string {
if v == nil || v.kind != Object { return nil
};keys := make([]string, len(v.obj))
for i, item := range v.obj { keys[i] = item.Key
};return keys
};func (v *Value) Members() []Member {
if v == nil || v.kind != Object { return nil
};out := make([]Member, len(v.obj))
copy(out, v.obj);return out
};func (v *Value) Values() []*Value {
if v == nil || v.kind != Array { return nil
};out := make([]*Value, len(v.list))
copy(out, v.list);return out
};func (v *Value) Get(key string) *Value {
if v != nil && v.kind == Object { for i := len(v.obj) - 1; i >= 0; i-- {
if v.obj[i].Key == key { return v.obj[i].Value
};}
};return NewNull()
};func (v *Value) Lookup(key string) (*Value, bool) {
if v != nil && v.kind == Object { for i := len(v.obj) - 1; i >= 0; i-- {
if v.obj[i].Key == key { return v.obj[i].Value, true
};}
};return nil, false
};func (v *Value) Index(i int) *Value {
if v != nil && v.kind == Array && i >= 0 && i < len(v.list) { return v.list[i]
};return NewNull()
};func (v *Value) String() (string, bool) {
if v != nil && v.kind == String { return v.text, true
};return "", false
};func (v *Value) NumberText() (string, bool) {
if v != nil && v.kind == Number { return v.num, true
};return "", false
};func (v *Value) Int64() (int64, bool) {
if v == nil || v.kind != Number { return 0, false
};n, e := strconv.ParseInt(v.num, 10, 64)
return n, e == nil;}
func (v *Value) Uint64() (uint64, bool) { if v == nil || v.kind != Number {
return 0, false;}
n, e := strconv.ParseUint(v.num, 10, 64);return n, e == nil
};func (v *Value) Float64() (float64, bool) {
if v == nil || v.kind != Number { return 0, false
};n, e := strconv.ParseFloat(v.num, 64)
return n, e == nil;}
func (v *Value) Bool() (bool, bool) { if v != nil && v.kind == Bool {
return v.flag, true;}
return false, false;}
func (v *Value) IsNull() bool { return v == nil || v.kind == Null };func (v *Value) Any(numberAsFloat bool) any {
if v == nil { return nil
};switch v.kind {
case Null: return nil
case String: return v.text
case Bool: return v.flag
case Number: if !numberAsFloat {
return NumberValue(v.num);}
f, _ := strconv.ParseFloat(v.num, 64);return f
case Array: out := make([]any, len(v.list))
for i, item := range v.list { out[i] = item.Any(numberAsFloat)
};return out
case Object: out := make(map[string]any, len(v.obj))
for _, item := range v.obj { out[item.Key] = item.Value.Any(numberAsFloat)
};return out
default: return nil
};}
type NumberValue string;func (n NumberValue) String() string            { return string(n) }
func (n NumberValue) Int64() (int64, error)     { return strconv.ParseInt(string(n), 10, 64) };func (n NumberValue) Float64() (float64, error) { return strconv.ParseFloat(string(n), 64) }
func (v *Value) DebugString() string { if v == nil {
return "null";}
if len(v.raw) > 0 { return string(v.raw)
};switch v.kind {
case String: return strconv.Quote(v.text)
case Number: return v.num
case Bool: return strconv.FormatBool(v.flag)
case Null: return "null"
case Array: var b bytes.Buffer
b.WriteByte('[');for i, x := range v.list {
if i > 0 { b.WriteByte(',')
};b.WriteString(x.DebugString())
};b.WriteByte(']')
return b.String();case Object:
var b bytes.Buffer;b.WriteByte('{')
for i, x := range v.obj { if i > 0 {
b.WriteByte(',');}
b.WriteString(strconv.Quote(x.Key));b.WriteByte(':')
b.WriteString(x.Value.DebugString());}
b.WriteByte('}');return b.String()
default: return fmt.Sprintf("<invalid:%d>", v.kind)
};}
