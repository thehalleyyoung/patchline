package canonical

import "testing"

func TestHashIsStableForStringKeyedMaps(t *testing.T) {
	first := map[string]any{
		"b": "two",
		"a": map[string]string{"z": "last", "m": "middle"},
	}
	second := map[string]any{
		"a": map[string]string{"m": "middle", "z": "last"},
		"b": "two",
	}

	if Hash(first) != Hash(second) {
		t.Fatalf("expected stable canonical hash")
	}
}
