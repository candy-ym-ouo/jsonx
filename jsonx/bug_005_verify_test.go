package jsonx_test

import "testing"

import "jsonx/jsonx"

type bug005Marshaler struct{ Value string }

func (m *bug005Marshaler) MarshalJSON() ([]byte, error) {
	return []byte(`{"value":"` + m.Value + `"}`), nil
}

func TestBug005TypedNilMarshalerIsNull(t *testing.T) {
	var value *bug005Marshaler
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("typed nil marshaler panicked: %v", r)
		}
	}()
	out, err := jsonx.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "null" {
		t.Fatalf("got %s, want null", out)
	}
}
