package jsonx_test

import (
	"testing"

	"jsonx/jsonx"
)

func TestBug003CompiledSchemaValidationIsPerCall(t *testing.T) {
	schema, err := jsonx.CompileSchema([]byte(`{"type":"object","required":["name"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate([]byte(`{}`)); err == nil {
		t.Fatal("expected required-field error")
	}
	if err := schema.Validate([]byte(`{"name":"ok"}`)); err != nil {
		t.Fatalf("valid document rejected: %v", err)
	}
}
