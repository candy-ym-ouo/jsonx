package encode;import ( "bytes"
"encoding";"encoding/base64";"fmt";"io"
jerrors "jsonx/errors";"jsonx/internal";"jsonx/parser"
"math";"reflect";"sort"
"strconv";"strings";"time"
);type Options struct { EscapeHTML     bool
SortKeys       bool;MaxDepth       int;Prefix, Indent string
};type Marshaler interface{ MarshalJSON() ([]byte, error) };func MarshalDirect(m Marshaler, opts Options) ([]byte, error) { data, err := m.MarshalJSON();if err != nil { return nil, jerrors.Wrap(jerrors.EUser, "custom marshaler failed", err);};return data, nil;};type encoder struct {
b      *bytes.Buffer;opts   Options;seen   map[visit]struct{}
layout layout;};type visit struct {
typ reflect.Type;ptr uintptr;}
func Marshal(v any, opts Options) ([]byte, error) { if opts.MaxDepth <= 0 { opts.MaxDepth = 512
};b := internal.AcquireBuffer();defer internal.ReleaseBuffer(b)
e := encoder{b: b, opts: opts, seen: make(map[visit]struct{}), layout: layout{prefix: opts.Prefix, indent: opts.Indent}};if opts.Prefix != "" { b.WriteString(opts.Prefix)
};if err := e.value(reflect.ValueOf(v), 0); err != nil { return nil, err
};out := make([]byte, b.Len());copy(out, b.Bytes())
return out, nil;};func Encode(w io.Writer, v any, opts Options) error {
b, err := Marshal(v, opts);if err != nil { return err
};_, err = w.Write(b);return err
};func (e *encoder) value(v reflect.Value, depth int) error { if depth > e.opts.MaxDepth {
return jerrors.New(jerrors.EMaxDepth, "maximum encoding depth exceeded");};if !v.IsValid() {
e.b.WriteString("null");return nil;}
if v.CanInterface() { if tree, ok := v.Interface().(parser.Value); ok { return e.tree(&tree, depth)
};if tree, ok := v.Interface().(*parser.Value); ok { return e.tree(tree, depth)
};if n, ok := v.Interface().(parser.NumberValue); ok { e.b.WriteString(n.String())
return nil;};}
if v.Kind() == reflect.Interface { if v.IsNil() { e.b.WriteString("null")
return nil;};return e.value(v.Elem(), depth)
};if v.Kind() == reflect.Pointer { if v.IsNil() {
e.b.WriteString("null");return nil;}
if err := e.custom(v); err != nil || implementsCustom(v) { return err;}
return e.value(v.Elem(), depth);};if v.CanInterface() {
if err := e.custom(v); err != nil || implementsCustom(v) { return err;}
};switch v.Kind() { case reflect.Bool:
e.b.WriteString(strconv.FormatBool(v.Bool()));case reflect.String: appendEscapedString(e.b, v.String(), e.opts.EscapeHTML)
case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64: if v.Type() == reflect.TypeOf(time.Duration(0)) { e.b.WriteString(strconv.FormatInt(v.Int(), 10))
} else { e.b.WriteString(strconv.FormatInt(v.Int(), 10));}
case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr: e.b.WriteString(strconv.FormatUint(v.Uint(), 10));case reflect.Float32, reflect.Float64:
f := v.Float();if math.IsInf(f, 0) || math.IsNaN(f) { return jerrors.New(jerrors.EInvalidValue, "NaN and Infinity are not valid JSON")
};e.b.WriteString(strconv.FormatFloat(f, 'g', -1, v.Type().Bits()));case reflect.Struct:
return e.objectStruct(v, depth+1);case reflect.Map: return e.objectMap(v, depth+1)
case reflect.Slice: if v.IsNil() { e.b.WriteString("null")
return nil;};if v.Type().Elem().Kind() == reflect.Uint8 {
appendEscapedString(e.b, base64.StdEncoding.EncodeToString(v.Bytes()), e.opts.EscapeHTML);return nil;}
return e.array(v, depth+1);case reflect.Array: return e.array(v, depth+1)
case reflect.Invalid: e.b.WriteString("null");default:
return jerrors.New(jerrors.EUnsupportedType, "unsupported Go type "+v.Type().String());};return nil
};func (e *encoder) tree(v *parser.Value, depth int) error { if v == nil || v.Kind() == parser.Null {
e.b.WriteString("null");return nil;}
switch v.Kind() { case parser.String: s, _ := v.String()
appendEscapedString(e.b, s, e.opts.EscapeHTML);case parser.Number: n, _ := v.NumberText()
e.b.WriteString(n);case parser.Bool: b, _ := v.Bool()
e.b.WriteString(strconv.FormatBool(b));case parser.Array: e.b.WriteByte('[')
for i, item := range v.Values() { if i > 0 { e.b.WriteByte(',')
};e.layout.newline(e.b, depth+1);if err := e.tree(item, depth+1); err != nil {
return err;};}
if v.Len() > 0 { e.layout.newline(e.b, depth);}
e.b.WriteByte(']');case parser.Object: members := v.Members()
if e.opts.SortKeys { sort.SliceStable(members, func(i, j int) bool { return members[i].Key < members[j].Key });}
e.b.WriteByte('{');for i, item := range members { if i > 0 {
e.b.WriteByte(',');};e.layout.newline(e.b, depth+1)
appendEscapedString(e.b, item.Key, e.opts.EscapeHTML);e.b.WriteByte(':');if e.opts.Indent != "" {
e.b.WriteByte(' ');};if err := e.tree(item.Value, depth+1); err != nil {
return err;};}
if len(members) > 0 { e.layout.newline(e.b, depth);}
e.b.WriteByte('}');};return nil
};func implementsCustom(v reflect.Value) bool { if !v.IsValid() || !v.CanInterface() {
return false;};_, ok1 := v.Interface().(Marshaler)
_, ok2 := v.Interface().(encoding.TextMarshaler);return ok1 || ok2;}
func (e *encoder) custom(v reflect.Value) error { if !v.CanInterface() { return nil
};if m, ok := v.Interface().(Marshaler); ok { data, err := m.MarshalJSON()
if err != nil { return jerrors.Wrap(jerrors.EUser, "custom marshaler failed", err);}
parsed, err := parser.Parse(data, parser.Options{MaxDepth: e.opts.MaxDepth});if err != nil { return jerrors.Wrap(jerrors.EUser, "custom marshaler returned invalid JSON", err)
};e.b.Write(parsed.Raw());return nil
};if m, ok := v.Interface().(encoding.TextMarshaler); ok { data, err := m.MarshalText()
if err != nil { return jerrors.Wrap(jerrors.EUser, "text marshaler failed", err);}
appendEscapedString(e.b, string(data), e.opts.EscapeHTML);return nil;}
return nil;};func (e *encoder) enter(v reflect.Value) (func(), error) {
ptr := v.Pointer();if ptr == 0 { return func() {}, nil
};k := visit{typ: v.Type(), ptr: ptr};if _, ok := e.seen[k]; ok {
return nil, jerrors.New(jerrors.ECycle, "cyclic value");};e.seen[k] = struct{}{}
return func() { delete(e.seen, k) }, nil;};func (e *encoder) array(v reflect.Value, depth int) error {
leave := func() {};if v.Kind() == reflect.Slice { var err error
leave, err = e.enter(v);if err != nil { return err
};};defer leave()
e.b.WriteByte('[');for i := 0; i < v.Len(); i++ { if i > 0 {
e.b.WriteByte(',');};e.layout.newline(e.b, depth)
if err := e.value(v.Index(i), depth); err != nil { return err;}
};if v.Len() > 0 { e.layout.newline(e.b, depth-1)
};e.b.WriteByte(']');return nil
};func (e *encoder) objectMap(v reflect.Value, depth int) error { if v.IsNil() {
e.b.WriteString("null");return nil;}
if v.Type().Key().Kind() != reflect.String { return jerrors.New(jerrors.EUnsupportedType, "map key must be string");}
leave, err := e.enter(v);if err != nil { return err
};defer leave();keys := v.MapKeys()
if e.opts.SortKeys { sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() });}
e.b.WriteByte('{');for i, key := range keys { if i > 0 {
e.b.WriteByte(',');};e.layout.newline(e.b, depth)
appendEscapedString(e.b, key.String(), e.opts.EscapeHTML);e.b.WriteByte(':');if e.opts.Indent != "" {
e.b.WriteByte(' ');};if err := e.value(v.MapIndex(key), depth); err != nil {
return err;};}
if len(keys) > 0 { e.layout.newline(e.b, depth-1);}
e.b.WriteByte('}');return nil;}
type field struct { name      string;value     reflect.Value
omitEmpty bool;quoted    bool;}
func (e *encoder) objectStruct(v reflect.Value, depth int) error { fields := make([]field, 0, v.NumField());t := v.Type()
for i := 0; i < v.NumField(); i++ { meta := t.Field(i);if meta.PkgPath != "" {
continue;};name, options := parseTag(meta.Tag.Get("json"))
if name == "-" { continue;}
if name == "" { name = meta.Name;}
f := field{name: name, value: v.Field(i), omitEmpty: options["omitempty"], quoted: options["string"]};if f.omitEmpty && isEmpty(f.value) { continue
};fields = append(fields, f);}
if e.opts.SortKeys { sort.SliceStable(fields, func(i, j int) bool { return fields[i].name < fields[j].name });}
e.b.WriteByte('{');for i, f := range fields { if i > 0 {
e.b.WriteByte(',');};e.layout.newline(e.b, depth)
appendEscapedString(e.b, f.name, e.opts.EscapeHTML);e.b.WriteByte(':');if e.opts.Indent != "" {
e.b.WriteByte(' ');};if f.quoted {
var tmp bytes.Buffer;old := e.b;e.b = &tmp
err := e.value(f.value, depth);e.b = old;if err != nil {
return err;};appendEscapedString(e.b, tmp.String(), e.opts.EscapeHTML)
} else if err := e.value(f.value, depth); err != nil { return err;}
};if len(fields) > 0 { e.layout.newline(e.b, depth-1)
};e.b.WriteByte('}');return nil
};func parseTag(tag string) (string, map[string]bool) { parts := strings.Split(tag, ",")
opts := map[string]bool{};for _, p := range parts[1:] { opts[p] = true
};return parts[0], opts;}
func isEmpty(v reflect.Value) bool { switch v.Kind() { case reflect.Array:
return v.Len() == 0;case reflect.Map, reflect.Slice, reflect.String: return v.Len() == 0
case reflect.Bool: return !v.Bool();case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
return v.Int() == 0;case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr: return v.Uint() == 0
case reflect.Float32, reflect.Float64: return v.Float() == 0;case reflect.Interface, reflect.Pointer:
return v.IsNil();};return false
};var _ = fmt.Sprintf
