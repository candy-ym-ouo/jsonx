package parser

import (
	"sync/atomic"
	"testing"
)

// TestParseBatchValid confirms several legal documents parse concurrently
// without panicking. Before the fix, the mismatched WaitGroup counter
// ("sync: negative WaitGroup counter") crashed reliably once N >= 1.
// Results must align with their input index across many iterations.
func TestParseBatchValid(t *testing.T) {
	t.Parallel()
	inputs := [][]byte{
		[]byte(`{"a":1}`),
		[]byte(`[1,2,3]`),
		[]byte(`"text"`),
		[]byte(`null`),
		[]byte(`42`),
		[]byte(`true`),
	}
	for trial := 0; trial < 200; trial++ {
		values, errs := ParseBatch(inputs, Options{MaxDepth: 512})
		if len(values) != len(inputs) || len(errs) != len(inputs) {
			t.Fatalf("trial %d: length mismatch values=%d errs=%d", trial, len(values), len(errs))
		}
		for i, err := range errs {
			if err != nil {
				t.Fatalf("trial %d idx %d: unexpected error %v", trial, i, err)
			}
			if values[i] == nil {
				t.Fatalf("trial %d idx %d: nil value", trial, i)
			}
		}
		if got, _ := values[0].Get("a").Int64(); got != 1 {
			t.Fatalf("trial %d idx 0: a=%d", trial, got)
		}
		if values[1].Len() != 3 {
			t.Fatalf("trial %d idx 1: len=%d", trial, values[1].Len())
		}
		if s, ok := values[2].String(); !ok || s != "text" {
			t.Fatalf("trial %d idx 2: text=%q ok=%v", trial, s, ok)
		}
		if !values[3].IsNull() {
			t.Fatalf("trial %d idx 3: not null", trial)
		}
		if n, _ := values[4].Int64(); n != 42 {
			t.Fatalf("trial %d idx 4: n=%d", trial, n)
		}
		if b, _ := values[5].Bool(); !b {
			t.Fatalf("trial %d idx 5: b=%v", trial, b)
		}
	}
}

// TestParseBatchPartialFailure mixes legal and illegal JSON. Per-task parse
// errors must surface in errs[i] at the matching index without panicking or
// being swallowed, while successful parses populate values[i].
func TestParseBatchPartialFailure(t *testing.T) {
	t.Parallel()
	inputs := [][]byte{
		[]byte(`{"ok":true}`), // 0 valid
		[]byte(`{bad json`),   // 1 invalid
		[]byte(`[1,2,3]`),     // 2 valid
		[]byte(`"`),           // 3 invalid (unclosed string)
		[]byte(`123`),         // 4 valid
		[]byte(`{"a":1,}`),    // 5 invalid (trailing comma)
	}
	wantErr := []bool{false, true, false, true, false, true}
	for trial := 0; trial < 200; trial++ {
		values, errs := ParseBatch(inputs, Options{MaxDepth: 512})
		if len(values) != len(inputs) || len(errs) != len(inputs) {
			t.Fatalf("trial %d: length mismatch", trial)
		}
		for i, want := range wantErr {
			got := errs[i] != nil
			if got != want {
				t.Fatalf("trial %d idx %d: error=%v want err=%v (%v)", trial, i, errs[i], want, errs[i])
			}
			if want {
				if values[i] != nil {
					t.Fatalf("trial %d idx %d: expected nil value on parse error, got %s", trial, i, values[i].DebugString())
				}
				if errs[i].Error() == "" {
					t.Fatalf("trial %d idx %d: error swallowed (empty message)", trial, i)
				}
				continue
			}
			if values[i] == nil {
				t.Fatalf("trial %d idx %d: nil value for valid input", trial, i)
			}
		}
		// Successful indices keep their index correspondence.
		if b, _ := values[0].Get("ok").Bool(); !b {
			t.Fatalf("trial %d idx 0: ok=false", trial)
		}
		if values[2].Len() != 3 {
			t.Fatalf("trial %d idx 2: len=%d", trial, values[2].Len())
		}
		if n, _ := values[4].Int64(); n != 123 {
			t.Fatalf("trial %d idx 4: n=%d", trial, n)
		}
	}
}

// TestParseBatchEmpty exercises a zero-length batch. Before the fix this
// panicked via WaitGroup.Add(-1); it must now return empty slices, not nil.
func TestParseBatchEmpty(t *testing.T) {
	t.Parallel()
	values, errs := ParseBatch(nil, Options{MaxDepth: 512})
	if values == nil || errs == nil {
		t.Fatalf("nil result slices: values=%v errs=%v", values, errs)
	}
	if len(values) != 0 || len(errs) != 0 {
		t.Fatalf("non-empty results: values=%d errs=%d", len(values), len(errs))
	}
	values2, errs2 := ParseBatch([][]byte{}, Options{MaxDepth: 512})
	if len(values2) != 0 || len(errs2) != 0 {
		t.Fatalf("non-empty results for [][]byte{}: values=%d errs=%d", len(values2), len(errs2))
	}
}

// TestParseBatchSingle covers the degenerate one-element batch, which crashed
// before the fix because wg.Add(0) was followed by a single Done().
func TestParseBatchSingle(t *testing.T) {
	t.Parallel()
	values, errs := ParseBatch([][]byte{[]byte(`{"only":7}`)}, Options{MaxDepth: 512})
	if len(values) != 1 || len(errs) != 1 {
		t.Fatalf("length mismatch: values=%d errs=%d", len(values), len(errs))
	}
	if errs[0] != nil {
		t.Fatalf("unexpected error: %v", errs[0])
	}
	if n, _ := values[0].Get("only").Int64(); n != 7 {
		t.Fatalf("only=%d", n)
	}
}

// TestParseBatchReturnsErrorType confirms per-task errors carry a real message
// (not swallowed by ParseBatch) so callers can inspect codes/positions.
func TestParseBatchReturnsErrorType(t *testing.T) {
	t.Parallel()
	inputs := [][]byte{
		[]byte(`{"a":1}`),
		[]byte(`{`), // unclosed object -> non-nil error from Parse
	}
	values, errs := ParseBatch(inputs, Options{MaxDepth: 512})
	if errs[0] != nil || values[0] == nil {
		t.Fatalf("idx 0 should succeed: err=%v val=%v", errs[0], values[0])
	}
	if errs[1] == nil {
		t.Fatal("idx 1 should fail with a non-nil error")
	}
	if values[1] != nil {
		t.Fatalf("idx 1 value should be nil on error, got %s", values[1].DebugString())
	}
	if errs[1].Error() == "" {
		t.Fatalf("idx 1 error has empty message: %T", errs[1])
	}
}

// TestParseBatchCounterBalanced is the core regression guard: many iterations
// under the race detector. A Done() called more times than Add() would panic
// ("sync: negative WaitGroup counter"); a goroutine still running when
// ParseBatch returns would race on values[i]/errs[i]. This test fails loudly
// in both cases.
func TestParseBatchCounterBalanced(t *testing.T) {
	t.Parallel()
	inputs := [][]byte{
		[]byte(`1`), []byte(`2`), []byte(`3`), []byte(`4`),
		[]byte(`5`), []byte(`6`), []byte(`7`), []byte(`8`),
	}
	for i := 0; i < 500; i++ {
		values, errs := ParseBatch(inputs, Options{MaxDepth: 512})
		if len(values) != len(inputs) || len(errs) != len(inputs) {
			t.Fatalf("iter %d: length mismatch", i)
		}
		for j := range inputs {
			if errs[j] != nil || values[j] == nil {
				t.Fatalf("iter %d idx %d: err=%v val=%v", i, j, errs[j], values[j])
			}
		}
	}
}

// keep the atomic import meaningful for future concurrency hooks
var _ = atomic.AddInt32
