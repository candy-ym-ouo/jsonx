package jsonx_test

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	jerrors "jsonx/errors"
	"jsonx/jsonx"
)

func TestParseFormatAndPath(t *testing.T) {
	data := []byte(`{"users":[{"name":"Ada","emoji":"\uD83D\uDE80"}],"ok":true}`)
	v, err := jsonx.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := v.Get("users").Index(0).Get("emoji").String(); got != "🚀" {
		t.Fatalf("unicode = %q", got)
	}
	name, err := jsonx.PathGet(data, "$.users[0].name")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := name.String(); got != "Ada" {
		t.Fatalf("name = %q", got)
	}
	if got := string(name.Raw()); got != `"Ada"` {
		t.Fatalf("name raw = %q", got)
	}
	if got := string(v.Get("users").Raw()); got != `[{"name":"Ada","emoji":"\uD83D\uDE80"}]` {
		t.Fatalf("users raw = %q", got)
	}
	formatted, err := jsonx.Format(data, "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(formatted, []byte("\n")) {
		t.Fatalf("not formatted: %s", formatted)
	}
}

func TestPathQuotedKeyRoundTrip(t *testing.T) {
	value, err := jsonx.PathGet([]byte(`{"a.b":{"x\\y'":7}}`), `$['a.b']['x\\y\'']`)
	if err != nil {
		t.Fatal(err)
	}
	if number, ok := value.Int64(); !ok || number != 7 {
		t.Fatalf("value = %s", value.DebugString())
	}
}

func TestInvalidJSONHasLocation(t *testing.T) {
	_, err := jsonx.Parse([]byte("{\n  \"a\": 1,\n}"))
	var located *jerrors.Error
	if !asError(err, &located) {
		t.Fatalf("expected located error, got %T %v", err, err)
	}
	if located.Code() != jerrors.EUnexpectedToken || located.Position().Line != 3 {
		t.Fatalf("bad location: %+v", located)
	}
}

func TestUTF8ErrorColumnCountsRunes(t *testing.T) {
	_, err := jsonx.Parse([]byte(`["你",?]`))
	var located *jerrors.Error
	if !asError(err, &located) {
		t.Fatalf("expected located error, got %T %v", err, err)
	}
	if located.Position().Column != 6 {
		t.Fatalf("column = %d, want 6", located.Position().Column)
	}
}

func asError(err error, target **jerrors.Error) bool {
	for err != nil {
		if value, ok := err.(*jerrors.Error); ok {
			*target = value
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func TestStrictDecodeAggregates(t *testing.T) {
	type config struct {
		Port int    `json:"port,required"`
		Host string `json:"host,required"`
	}
	var cfg config
	err := jsonx.DecodeStrict([]byte(`{"port":"8080","extra":1}`), &cfg)
	list, ok := err.(jerrors.ErrorList)
	if !ok || len(list) != 3 {
		t.Fatalf("expected three errors, got %T %v", err, err)
	}
}

func TestDecodeNonEmptyInterfaceReturnsError(t *testing.T) {
	var dst fmt.Stringer
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Decode panicked: %v", recovered)
		}
	}()
	if err := jsonx.Decode([]byte(`"value"`), &dst); err == nil {
		t.Fatal("expected type mismatch")
	}
}

func TestSchema(t *testing.T) {
	schema := []byte(`{"type":"object","required":["name"],"properties":{"name":{"type":"string","minLength":2},"age":{"type":"integer","minimum":0}},"additionalProperties":false}`)
	err := jsonx.ValidateSchema([]byte(`{"name":"x","age":-1,"extra":true}`), schema)
	list, ok := err.(jerrors.ErrorList)
	if !ok || len(list) != 3 {
		t.Fatalf("expected three schema errors, got %T %v", err, err)
	}
}

func TestMarshalAndCycle(t *testing.T) {
	type item struct {
		Name  string `json:"name"`
		Empty string `json:"empty,omitempty"`
	}
	out, err := jsonx.Marshal(item{Name: "<jsonx>"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"name":"\u003cjsonx\u003e"}` {
		t.Fatalf("marshal = %s", out)
	}
	m := map[string]any{}
	m["self"] = m
	if _, err := jsonx.Marshal(m); err == nil || !strings.Contains(err.Error(), jerrors.ECycle) {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestByteSliceUsesBase64(t *testing.T) {
	type payload struct {
		Data []byte `json:"data"`
	}
	out, err := jsonx.Marshal(payload{Data: []byte("jsonx")})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"data":"anNvbng="}` {
		t.Fatalf("marshal bytes = %s", out)
	}
	var decoded payload
	if err := jsonx.Decode(out, &decoded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.Data, []byte("jsonx")) {
		t.Fatalf("decoded bytes = %q", decoded.Data)
	}
}

func TestStreamAndTokens(t *testing.T) {
	s := jsonx.NewStreamReader(strings.NewReader(`[1,2,3]`))
	count := 0
	for s.Next() {
		count++
	}
	if count != 3 || s.Err() != io.EOF {
		t.Fatalf("count=%d err=%v", count, s.Err())
	}
	tokens, err := jsonx.Tokenize(strings.NewReader(`{"x":1}`))
	if err != nil {
		t.Fatal(err)
	}
	for tokens.Next().String() != "EOF" {
	}
	if tokens.Err() != nil {
		t.Fatal(tokens.Err())
	}
}

func TestStreamRejectsTrailingData(t *testing.T) {
	for _, input := range []string{`[1]x`, `1 2`} {
		s := jsonx.NewStreamReader(strings.NewReader(input))
		for s.Next() {
		}
		if s.Err() == nil || s.Err() == io.EOF {
			t.Fatalf("input %q accepted: %v", input, s.Err())
		}
	}
}

func TestStreamReadsNestedArrays(t *testing.T) {
	s := jsonx.NewStreamReader(strings.NewReader(`[{"a":1},[2,3],{"b":[4]}]`))
	want := []string{`{"a":1}`, `[2,3]`, `{"b":[4]}`}
	var got []string
	for s.Next() {
		encoded, err := jsonx.Marshal(s.Value())
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, string(encoded))
	}
	if s.Err() != io.EOF {
		t.Fatalf("stream error: %v", s.Err())
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("values = %v, want %v", got, want)
	}
}

func TestStreamAllowsCommentsBetweenElements(t *testing.T) {
	s := jsonx.NewStreamReader(strings.NewReader("[1, // line\n /* block */ 2]"), jsonx.AllowComments(true))
	var got []int64
	for s.Next() {
		value, ok := s.Value().Int64()
		if !ok {
			t.Fatalf("not an integer: %s", s.Value().DebugString())
		}
		got = append(got, value)
	}
	if s.Err() != io.EOF || !reflect.DeepEqual(got, []int64{1, 2}) {
		t.Fatalf("values=%v err=%v", got, s.Err())
	}
}

func TestSchemaRejectsInvalidPatternInArrayBranch(t *testing.T) {
	_, err := jsonx.CompileSchema([]byte(`{"allOf":[{"type":"string","pattern":"["}]}`))
	if err == nil || !strings.Contains(err.Error(), jerrors.ESchemaPattern) {
		t.Fatalf("invalid nested pattern accepted: %v", err)
	}
}

func TestParseBatchConcurrencyAndOrder(t *testing.T) {
	// Regression for "sync: negative WaitGroup counter": the public
	// jsonx.ParseBatch must run all goroutines concurrently, pair every Add
	// with exactly one Done, wait for all of them before returning, keep
	// results index-correspondent with inputs, surface per-task parse errors
	// (not swallow them), and accept empty batches without panicking.
	t.Run("valid", func(t *testing.T) {
		inputs := [][]byte{[]byte(`{"a":1}`), []byte(`[1,2,3]`), []byte(`"x"`)}
		for i := 0; i < 100; i++ {
			values, errs := jsonx.ParseBatch(inputs)
			if len(values) != len(inputs) || len(errs) != len(inputs) {
				t.Fatalf("length mismatch: %d %d", len(values), len(errs))
			}
			for _, err := range errs {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
			}
			if n, _ := values[0].Get("a").Int64(); n != 1 {
				t.Fatalf("a=%d", n)
			}
			if values[1].Len() != 3 {
				t.Fatalf("len=%d", values[1].Len())
			}
		}
	})
	t.Run("partial failure", func(t *testing.T) {
		inputs := [][]byte{[]byte(`{"ok":true}`), []byte(`{bad`), []byte(`123`)}
		for i := 0; i < 100; i++ {
			values, errs := jsonx.ParseBatch(inputs)
			if len(values) != 3 || len(errs) != 3 {
				t.Fatalf("length mismatch: %d %d", len(values), len(errs))
			}
			if errs[0] != nil || values[0] == nil {
				t.Fatalf("idx 0: err=%v val=%v", errs[0], values[0])
			}
			if errs[1] == nil || values[1] != nil || errs[1].Error() == "" {
				t.Fatalf("idx 1 should fail with non-empty error: err=%v val=%v", errs[1], values[1])
			}
			if errs[2] != nil || values[2] == nil {
				t.Fatalf("idx 2: err=%v val=%v", errs[2], values[2])
			}
		}
	})
	t.Run("empty", func(t *testing.T) {
		values, errs := jsonx.ParseBatch(nil)
		if len(values) != 0 || len(errs) != 0 {
			t.Fatalf("expected empty, got %d %d", len(values), len(errs))
		}
		values, errs = jsonx.ParseBatch([][]byte{})
		if len(values) != 0 || len(errs) != 0 {
			t.Fatalf("expected empty, got %d %d", len(values), len(errs))
		}
	})
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{`null`, `[]`, `{"a":1}`, `"x"`, `[true,false]`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) { _, _ = jsonx.Parse([]byte(input)) })
}
