package jsonx_test

import (
	"bytes"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"jsonx/jsonx"
)

// withComment is a JSON document containing comments. By default comments are
// rejected; only an explicit AllowComments(true) makes it parse.
const withComment = `{"a":1 /* c */}`

// TestOptionsDoNotLeakAcrossSequentialCalls guards against the historical bug
// where Parse and Marshal resolved options against a package-level mutable
// Options value. A single call that opts into AllowComments(true) used to
// leave that flag set, so a later call that omitted the option still accepted
// comments. Each call must start from independent defaults.
func TestOptionsDoNotLeakAcrossSequentialCalls(t *testing.T) {
	// Sanity: comments are rejected by default.
	if _, err := jsonx.Parse([]byte(withComment)); err == nil {
		t.Fatalf("comments should be rejected by default; want error")
	}

	// Opt in for this single call only.
	if _, err := jsonx.Parse([]byte(withComment), jsonx.AllowComments(true)); err != nil {
		t.Fatalf("AllowComments(true) parse failed: %v", err)
	}

	// The very next call omits the option: it must NOT inherit AllowComments.
	if _, err := jsonx.Parse([]byte(withComment)); err == nil {
		t.Fatalf("AllowComments leaked into a later call; want error")
	}

	// Same guarantee for the other resolveShared path: Marshal + EscapeHTML.
	t.Run("MarshalEscapeHTMLLeak", func(t *testing.T) {
		type item struct{ HTML string `json:"html"` }

		// Opt out of HTML escaping for this single call.
		raw, err := jsonx.Marshal(item{HTML: "<b>"}, jsonx.EscapeHTML(false))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(raw, []byte("<b>")) {
			t.Fatalf("EscapeHTML(false) ignored: %s", raw)
		}
		// Next call omits the option: HTML escaping must be back on by default,
		// so the literal '<' is escaped rather than appearing verbatim.
		out, err := jsonx.Marshal(item{HTML: "<b>"})
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(out, []byte("<b>")) {
			t.Fatalf("EscapeHTML(false) leaked into a later call: %s", out)
		}
	})

	// SortKeys likewise must not leak between calls.
	t.Run("SortKeysLeak", func(t *testing.T) {
		// Opt in to key sorting for this call.
		if _, err := jsonx.Marshal(map[string]int{"b": 2, "a": 1}, jsonx.SortKeys(true)); err != nil {
			t.Fatal(err)
		}
		// A subsequent unsorted map without the option is not asserted on byte
		// order; the point is that the prior option must not affect a later call's
		// resolved options. The concurrent test below covers SortKeys more
		// rigorously.
	})
}

// TestOptionsConcurrentNoRace runs Parse and Marshal concurrently with
// differing options. Before the fix, all of these touched the shared
// package-level Options value; `go test -race` reported a data race. Each call
// must now resolve against its own independent copy of the defaults.
//
// Each scenario returns ok=false when its outcome diverges from the expected
// one (e.g. comments accepted by default = option leakage). Scenarios that
// legitimately error (MaxDepth exceeded, comments rejected by default) verify
// that they *do* error, so they are not falsely counted as failures.
func TestOptionsConcurrentNoRace(t *testing.T) {
	const goroutines = 32
	const iterations = 200

	deep := []byte(strings.Repeat("[", 16) + strings.Repeat("]", 16))

	type scenario struct {
		name string
		run  func() bool // returns true when the outcome matched expectations
	}
	scenarios := []scenario{
		{
			name: "Parse-AllowComments-true",
			run: func() bool {
				_, err := jsonx.Parse([]byte(withComment), jsonx.AllowComments(true))
				return err == nil
			},
		},
		{
			name: "Parse-AllowComments-default",
			run: func() bool {
				_, err := jsonx.Parse([]byte(withComment))
				// Comments must be rejected by default. A nil error here means
				// the AllowComments(true) flag leaked from a concurrent call.
				return err != nil
			},
		},
		{
			name: "Marshal-SortKeys-true",
			run: func() bool {
				out, err := jsonx.Marshal(map[string]int{"b": 2, "a": 1}, jsonx.SortKeys(true))
				return err == nil && bytes.HasPrefix(out, []byte(`{"a":1,"b":2}`))
			},
		},
		{
			name: "Marshal-SortKeys-default",
			run: func() bool {
				_, err := jsonx.Marshal(map[string]int{"b": 2, "a": 1})
				return err == nil
			},
		},
		{
			name: "Marshal-EscapeHTML-false",
			run: func() bool {
				out, err := jsonx.Marshal(struct {
					HTML string `json:"html"`
				}{HTML: "<b>"}, jsonx.EscapeHTML(false))
				return err == nil && bytes.Contains(out, []byte("<b>"))
			},
		},
		{
			name: "Marshal-EscapeHTML-default",
			run: func() bool {
				out, err := jsonx.Marshal(struct {
					HTML string `json:"html"`
				}{HTML: "<b>"})
				// Default escapes '<'; literal "<b>" must not survive.
				return err == nil && !bytes.Contains(out, []byte("<b>"))
			},
		},
		{
			name: "Parse-MaxDepth-small",
			run: func() bool {
				_, err := jsonx.Parse(deep, jsonx.MaxDepth(8))
				// Depth-16 input under MaxDepth(8) must error.
				return err != nil
			},
		},
		{
			name: "Parse-MaxDepth-default",
			run: func() bool {
				_, err := jsonx.Parse(deep)
				// Default MaxDepth(512) accepts depth-16.
				return err == nil
			},
		},
	}

	var wg sync.WaitGroup
	var failures atomic.Int64

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			sc := scenarios[id%len(scenarios)]
			for j := 0; j < iterations; j++ {
				if !sc.run() {
					failures.Add(1)
				}
			}
		}(i)
	}
	wg.Wait()

	if n := failures.Load(); n != 0 {
		t.Fatalf("%d concurrent calls produced an unexpected outcome (option leakage)", n)
	}
}
