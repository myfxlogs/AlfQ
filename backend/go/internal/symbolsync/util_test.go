package symbolsync

import (
	"testing"
)

func TestMarshalJSON(t *testing.T) {
	result := marshalJSON(map[string]string{"key": "value"})
	if result == nil {
		t.Fatal("marshalJSON returned nil")
	}
	if len(result) == 0 {
		t.Fatal("marshalJSON returned empty bytes")
	}
}

func TestMarshalJSON_Nil(t *testing.T) {
	result := marshalJSON(nil)
	if result == nil {
		t.Fatal("marshalJSON returned nil for nil input")
	}
}
