package jsonx_test

import (
	"errors"
	"jsonx/jsonx"
	"testing"
)

var bug009Sentinel = errors.New("bug009 validation failed")

type bug009Validator struct {
	Port int `json:"port"`
}

func (*bug009Validator) Validate() error { return bug009Sentinel }

func TestBug009ValidatorErrorPropagates(t *testing.T) {
	var dst bug009Validator
	err := jsonx.Decode([]byte(`{"port":8080}`), &dst)
	if !errors.Is(err, bug009Sentinel) {
		t.Fatalf("error = %v, errors.Is did not find sentinel", err)
	}
	if dst.Port != 8080 {
		t.Fatalf("Port = %d", dst.Port)
	}
}
