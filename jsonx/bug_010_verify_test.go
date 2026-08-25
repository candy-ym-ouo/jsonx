package jsonx_test

import (
	"jsonx/jsonx"
	"testing"
)

func TestBug010ParseBatchCompletesAndPreservesIndexes(t *testing.T) {
	inputs := [][]byte{[]byte(`{"id":1}`), []byte(`{"id":2}`), []byte(`{"id":3}`)}
	values, errs := jsonx.ParseBatch(inputs)
	for i := range inputs {
		if errs[i] != nil {
			t.Fatalf("errs[%d] = %v", i, errs[i])
		}
		if values[i] == nil || values[i].Get("id").DebugString() != string(rune('1'+i)) {
			t.Fatalf("values[%d] = %v", i, values[i])
		}
	}
	values, errs = jsonx.ParseBatch(nil)
	if len(values) != 0 || len(errs) != 0 {
		t.Fatalf("empty batch lengths = %d, %d", len(values), len(errs))
	}
}
