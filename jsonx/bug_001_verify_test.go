package jsonx_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"jsonx/jsonx"
)

func TestBug001ParseContextCancellation(t *testing.T) {
	data := []byte(strings.Repeat("[", 128) + "0" + strings.Repeat("]", 128))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := jsonx.ParseContext(ctx, data)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ParseContext error = %v, want context.Canceled", err)
	}
}
