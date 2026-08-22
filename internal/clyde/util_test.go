package clyde

import (
	"math"
	"testing"
)

func TestNumberIntRejectsUnsafeValues(t *testing.T) {
	tests := []any{
		math.Inf(1),
		1.25,
		jsonNumber("not-a-number"),
	}
	for _, value := range tests {
		if got := numberInt(value); got != 0 {
			t.Fatalf("numberInt(%#v) = %d, want 0", value, got)
		}
	}
}

func TestNumberIntAcceptsSafeValues(t *testing.T) {
	if got := numberInt(float64(42)); got != 42 {
		t.Fatalf("unexpected float conversion: %d", got)
	}
	if got := numberInt(jsonNumber("17")); got != 17 {
		t.Fatalf("unexpected jsonNumber conversion: %d", got)
	}
}
