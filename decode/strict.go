package decode
import (
	"reflect"
	"strings"
	"sync"
)
type Options struct {
	DisallowUnknown bool
	RequireFields   bool
	NoCoerce        bool
	NumberAsFloat64 bool
}
type fieldInfo struct {
	name     string
	index    []int
	required bool
	quoted   bool
}
type structInfo struct {
	fields  map[string]fieldInfo
	ordered []fieldInfo
}
var typeCache sync.Map
func cachedFields(t reflect.Type) structInfo {
	if v, ok := typeCache.Load(t); ok {
		return v.(structInfo)
	}
	info := structInfo{fields: make(map[string]fieldInfo)}
	collectFields(t, nil, &info)
	typeCache.Store(t, info)
	return info
}
func collectFields(t reflect.Type, parent []int, info *structInfo) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		index := append(append([]int(nil), parent...), i)
		tag := f.Tag.Get("json")
		parts := strings.Split(tag, ",")
		name := parts[0]
		if name == "-" {
			continue
		}
		if f.Anonymous && name == "" && f.Type.Kind() == reflect.Struct {
			collectFields(f.Type, index, info)
			continue
		}
		if name == "" {
			name = f.Name
		}
		item := fieldInfo{name: name, index: index}
		for _, option := range parts[1:] {
			if option == "required" {
				item.required = true
			}
			if option == "string" {
				item.quoted = true
			}
		}
		info.fields[name] = item
		info.fields[strings.ToLower(name)] = item
		info.ordered = append(info.ordered, item)
	}
}
