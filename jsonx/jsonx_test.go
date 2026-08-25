package jsonx_test

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

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

func TestStreamValuesClosesOnEnd(t *testing.T) {
	ch, stop := jsonx.StreamValues(strings.NewReader(`[1,2,3]`))
	defer stop()
	var got []int64
	for v := range ch {
		n, ok := v.Int64()
		if !ok {
			t.Fatalf("not an integer: %s", v.DebugString())
		}
		got = append(got, n)
	}
	// Ranging to completion must terminate: the channel is closed after the
	// last element, so this is a hard failure (deadlock) if it is not.
	if !reflect.DeepEqual(got, []int64{1, 2, 3}) {
		t.Fatalf("values = %v, want [1 2 3]", got)
	}
}

func TestStreamValuesClosesOnSingleValue(t *testing.T) {
	ch, stop := jsonx.StreamValues(strings.NewReader(`{"a":1}`))
	defer stop()
	count := 0
	for range ch {
		count++
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestStreamValuesEarlyStopUnblocksProducer(t *testing.T) {
	// Consume only the first element of a long stream, then stop. The producer
	// goroutine must exit promptly instead of blocking forever on its next send.
	ch, stop := jsonx.StreamValues(strings.NewReader(`[1,2,3,4,5,6,7,8,9,10]`))

	v, ok := <-ch
	if !ok {
		t.Fatal("expected at least one value")
	}
	if n, _ := v.Int64(); n != 1 {
		t.Fatalf("first value = %d, want 1", n)
	}
	stop()

	// The channel is closed once stop is honored (or the stream finishes). Give
	// the goroutine a bounded window to drain; a leak hangs this deadline.
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("producer goroutine did not exit after stop")
	}
}

func TestStreamValuesGoroutineStability(t *testing.T) {
	// Repeatedly start a stream, consume one element, and stop. If the producer
	// leaked per invocation the live goroutine count would climb monotonically.
	// With a correct lifecycle it must stay bounded and stable across rounds.
	runtime.GC()
	baseline := runtime.NumGoroutine()
	for i := 0; i < 100; i++ {
		ch, stop := jsonx.StreamValues(strings.NewReader(`[1,2,3,4,5]`))
		<-ch
		stop()
		// Drain any in-flight value so the producer's send completes before the
		// next round. This is for measurement stability, not correctness.
		for range ch {
		}
	}
	// Allow the final goroutine to schedule its exit, then assert no growth.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= baseline+1 {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("goroutine leak: baseline=%d now=%d", baseline, runtime.NumGoroutine())
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{`null`, `[]`, `{"a":1}`, `"x"`, `[true,false]`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) { _, _ = jsonx.Parse([]byte(input)) })
}
