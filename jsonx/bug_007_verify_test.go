package jsonx_test

import (
	"jsonx/jsonx"
	"testing"
)

func TestBug007OptionsDoNotLeakBetweenCalls(t *testing.T) {
	if _, err := jsonx.Parse([]byte(`{/* comment */"ok":true}`), jsonx.AllowComments(true)); err != nil {
		t.Fatal(err)
	}
	if _, err := jsonx.Parse([]byte(`{/* comment */"ok":true}`)); err == nil {
		t.Fatal("AllowComments leaked into next call")
	}
}
