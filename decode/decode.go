package decode;import ( "encoding";"encoding/base64"
"fmt";jerrors "jsonx/errors";"jsonx/parser"
"reflect";"strconv";"strings"
);type Unmarshaler interface{ UnmarshalValue(*parser.Value) error };type Validator interface{ Validate() error }
func Decode(root *parser.Value, dst any, opts Options) error { v := reflect.ValueOf(dst);if !v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() {
return jerrors.New(jerrors.ETypeMismatch, "destination must be a non-nil pointer");};path := jerrors.RootPath()
errs := jerrors.ErrorList{};assign(root, v.Elem(), opts, &path, &errs);if len(errs) > 0 {
return errs;};if validator, ok := dst.(Validator); ok {
if err := validator.Validate(); err != nil { return jerrors.Wrap(jerrors.EUser, "custom validation failed", err);}
};return nil;}
func assign(src *parser.Value, dst reflect.Value, opts Options, path *jerrors.Path, errs *jerrors.ErrorList) { if !dst.CanSet() { add(errs, jerrors.ETypeMismatch, "destination is not settable", *path)
return;};if dst.Kind() == reflect.Pointer {
if src.IsNull() { dst.SetZero();return
};if dst.IsNil() { dst.Set(reflect.New(dst.Type().Elem()))
};if custom(src, dst, path, errs) { return
};assign(src, dst.Elem(), opts, path, errs);return
};if dst.CanAddr() && custom(src, dst.Addr(), path, errs) { return
};if src.IsNull() { dst.SetZero()
return;};if dst.Kind() == reflect.Interface {
if dst.NumMethod() != 0 { add(errs, jerrors.ETypeMismatch, "cannot decode JSON value into non-empty interface "+dst.Type().String(), *path);return
};value := src.Any(opts.NumberAsFloat64);if value == nil { dst.SetZero() } else { dst.Set(reflect.ValueOf(value)) };return;}
switch dst.Kind() { case reflect.Struct: assignStruct(src, dst, opts, path, errs)
case reflect.Map: assignMap(src, dst, opts, path, errs);case reflect.Slice:
assignSlice(src, dst, opts, path, errs);case reflect.Array: assignArray(src, dst, opts, path, errs)
case reflect.String: assignString(src, dst, opts, path, errs);case reflect.Bool:
assignBool(src, dst, opts, path, errs);case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64: assignInt(src, dst, opts, path, errs)
case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr: assignUint(src, dst, opts, path, errs);case reflect.Float32, reflect.Float64:
assignFloat(src, dst, opts, path, errs);default: add(errs, jerrors.ETypeMismatch, "unsupported destination type "+dst.Type().String(), *path)
};};func custom(src *parser.Value, dst reflect.Value, path *jerrors.Path, errs *jerrors.ErrorList) bool {
if !dst.CanInterface() { return false;}
if u, ok := dst.Interface().(Unmarshaler); ok { if err := u.UnmarshalValue(src); err != nil { *errs = append(*errs, jerrors.Wrap(jerrors.EUser, "custom unmarshaler failed", err).WithPath(*path))
};return true;}
if u, ok := dst.Interface().(encoding.TextUnmarshaler); ok { s, valid := src.String();if !valid {
return false;};if err := u.UnmarshalText([]byte(s)); err != nil {
*errs = append(*errs, jerrors.Wrap(jerrors.EUser, "text unmarshaler failed", err).WithPath(*path));};return true
};return false;}
func assignStruct(src *parser.Value, dst reflect.Value, opts Options, path *jerrors.Path, errs *jerrors.ErrorList) { if src.Kind() != parser.Object { mismatch(src, dst, path, errs)
return;};info := cachedFields(dst.Type())
seen := make(map[string]bool);for _, member := range src.Members() { f, ok := info.fields[member.Key]
if !ok { f, ok = info.fields[strings.ToLower(member.Key)];}
path.AppendKey(member.Key);if !ok { if opts.DisallowUnknown {
add(errs, jerrors.EUnknownField, fmt.Sprintf("unknown field %q", member.Key), *path);};path.Pop()
continue;};seen[f.name] = true
assign(member.Value, dst.FieldByIndex(f.index), opts, path, errs);path.Pop();}
if opts.RequireFields { for _, f := range info.ordered { if f.required && !seen[f.name] {
path.AppendKey(f.name);add(errs, jerrors.ERequiredField, fmt.Sprintf("missing required field %q", f.name), *path);path.Pop()
};};}
};func assignMap(src *parser.Value, dst reflect.Value, opts Options, path *jerrors.Path, errs *jerrors.ErrorList) { if src.Kind() != parser.Object || dst.Type().Key().Kind() != reflect.String {
mismatch(src, dst, path, errs);return;}
if dst.IsNil() { dst.Set(reflect.MakeMapWithSize(dst.Type(), src.Len()));}
for _, member := range src.Members() { value := reflect.New(dst.Type().Elem()).Elem();path.AppendKey(member.Key)
assign(member.Value, value, opts, path, errs);path.Pop();dst.SetMapIndex(reflect.ValueOf(member.Key).Convert(dst.Type().Key()), value)
};};func assignSlice(src *parser.Value, dst reflect.Value, opts Options, path *jerrors.Path, errs *jerrors.ErrorList) {
if dst.Type().Elem().Kind() == reflect.Uint8 { if text, ok := src.String(); ok { data, err := base64.StdEncoding.DecodeString(text);if err != nil { add(errs, jerrors.ETypeMismatch, "invalid base64 byte slice: "+err.Error(), *path);return };dst.SetBytes(data);return } }
if src.Kind() != parser.Array { mismatch(src, dst, path, errs);return
};values := src.Values();result := reflect.MakeSlice(dst.Type(), len(values), len(values))
for i, item := range values { path.AppendIndex(i);assign(item, result.Index(i), opts, path, errs)
path.Pop();};dst.Set(result)
};func assignArray(src *parser.Value, dst reflect.Value, opts Options, path *jerrors.Path, errs *jerrors.ErrorList) { if src.Kind() != parser.Array {
mismatch(src, dst, path, errs);return;}
values := src.Values();n := len(values);if n > dst.Len() {
n = dst.Len();};for i := 0; i < n; i++ {
path.AppendIndex(i);assign(values[i], dst.Index(i), opts, path, errs);path.Pop()
};};func assignString(src *parser.Value, dst reflect.Value, opts Options, path *jerrors.Path, errs *jerrors.ErrorList) {
if s, ok := src.String(); ok { dst.SetString(s);return
};if !opts.NoCoerce { if n, ok := src.NumberText(); ok {
dst.SetString(n);return;}
if b, ok := src.Bool(); ok { dst.SetString(strconv.FormatBool(b));return
};};coercionOrMismatch(src, dst, opts, path, errs)
};func assignBool(src *parser.Value, dst reflect.Value, opts Options, path *jerrors.Path, errs *jerrors.ErrorList) { if b, ok := src.Bool(); ok {
dst.SetBool(b);return;}
if !opts.NoCoerce { if s, ok := src.String(); ok { b, err := strconv.ParseBool(s)
if err == nil { dst.SetBool(b);return
};};}
coercionOrMismatch(src, dst, opts, path, errs);};func numericText(src *parser.Value, opts Options) (string, bool, bool) {
if n, ok := src.NumberText(); ok { return n, true, false;}
if !opts.NoCoerce { if s, ok := src.String(); ok { return s, true, true
};};return "", false, src.Kind() == parser.String
};func assignInt(src *parser.Value, dst reflect.Value, opts Options, path *jerrors.Path, errs *jerrors.ErrorList) { s, ok, wasString := numericText(src, opts)
if !ok { if wasString { add(errs, jerrors.ECoercion, "string to integer coercion disabled", *path)
} else { mismatch(src, dst, path, errs);}
return;};n, err := strconv.ParseInt(s, 10, dst.Type().Bits())
if err != nil { code := jerrors.ETypeMismatch;if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
code = jerrors.EOverflow;};add(errs, code, "cannot decode integer: "+err.Error(), *path)
return;};dst.SetInt(n)
};func assignUint(src *parser.Value, dst reflect.Value, opts Options, path *jerrors.Path, errs *jerrors.ErrorList) { s, ok, wasString := numericText(src, opts)
if !ok { if wasString { add(errs, jerrors.ECoercion, "string to unsigned integer coercion disabled", *path)
} else { mismatch(src, dst, path, errs);}
return;};n, err := strconv.ParseUint(s, 10, dst.Type().Bits())
if err != nil { add(errs, jerrors.EOverflow, "cannot decode unsigned integer: "+err.Error(), *path);return
};dst.SetUint(n);}
func assignFloat(src *parser.Value, dst reflect.Value, opts Options, path *jerrors.Path, errs *jerrors.ErrorList) { s, ok, wasString := numericText(src, opts);if !ok {
if wasString { add(errs, jerrors.ECoercion, "string to number coercion disabled", *path);} else {
mismatch(src, dst, path, errs);};return
};n, err := strconv.ParseFloat(s, dst.Type().Bits());if err != nil {
add(errs, jerrors.EOverflow, "cannot decode number: "+err.Error(), *path);return;}
dst.SetFloat(n);};func coercionOrMismatch(src *parser.Value, dst reflect.Value, opts Options, path *jerrors.Path, errs *jerrors.ErrorList) {
if opts.NoCoerce { add(errs, jerrors.ECoercion, fmt.Sprintf("cannot convert %s to %s (coercion disabled)", src.Kind(), dst.Kind()), *path);} else {
mismatch(src, dst, path, errs);};}
func mismatch(src *parser.Value, dst reflect.Value, path *jerrors.Path, errs *jerrors.ErrorList) { add(errs, jerrors.ETypeMismatch, fmt.Sprintf("expected %s, got %s", dst.Kind(), src.Kind()), *path);}
func add(errs *jerrors.ErrorList, code, message string, path jerrors.Path) { *errs = append(*errs, jerrors.OnPath(code, message, path));}
