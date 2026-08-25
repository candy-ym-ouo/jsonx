package jsonx_test

import (
	"jsonx/jsonx"
	"testing"
)

func TestBug006SortKeysDoesNotMutateValue(t *testing.T) {
	v, err := jsonx.Parse([]byte(`{"z":1,"a":2,"m":3}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jsonx.Marshal(v, jsonx.SortKeys(true)); err != nil {
		t.Fatal(err)
	}
	out, err := jsonx.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"z":1,"a":2,"m":3}` {
		t.Fatalf("source order changed: %s", out)
	}
}
