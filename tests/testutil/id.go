package testutil

import (
	"encoding/json"
	"strconv"
	"testing"
)

// MustID parses id fields that may be JSON number or string.
func MustID(t *testing.T, raw interface{}, field string) int64 {
	t.Helper()

	switch v := raw.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			t.Fatalf("invalid %s json.Number %q: %v", field, v, err)
		}
		return n
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			t.Fatalf("invalid %s string %q: %v", field, v, err)
		}
		return n
	default:
		t.Fatalf("unexpected %s type %T", field, raw)
		return 0
	}
}
