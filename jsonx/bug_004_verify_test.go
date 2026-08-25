package jsonx_test

import (
	"errors"
	"testing"

	"jsonx/jsonx"
	"jsonx/parser"
)

type bug004Unmarshaler struct{}

var bug004Sentinel = errors.New("bug004 sentinel")

func (*bug004Unmarshaler) UnmarshalValue(*parser.Value) error { return bug004Sentinel }

func TestBug004CustomUnmarshalPreservesCause(t *testing.T) {
	var dst bug004Unmarshaler
	err := jsonx.Decode([]byte(`"value"`), &dst)
	if !errors.Is(err, bug004Sentinel) {
		t.Fatalf("error = %v, errors.Is did not find sentinel", err)
	}
}
