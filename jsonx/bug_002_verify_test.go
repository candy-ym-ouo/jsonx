package jsonx_test

import (
	"bytes"
	"testing"

	"jsonx/jsonx"
)

func TestBug002MarshalResultOwnsBytes(t *testing.T) {
	type payload struct {
		Value string `json:"value"`
	}
	first, err := jsonx.Marshal(payload{Value: "first"})
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte(nil), first...)
	if _, err := jsonx.Marshal(payload{Value: "a much longer second result"}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, want) {
		t.Fatalf("first result changed: got %q want %q", first, want)
	}
}
