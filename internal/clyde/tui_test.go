package clyde

import "testing"

func TestModelIndex(t *testing.T) {
	if got := modelIndex("2", 3); got != 1 {
		t.Fatalf("unexpected index: %d", got)
	}
	if got := modelIndex("bad", 3); got != -1 {
		t.Fatalf("unexpected invalid index: %d", got)
	}
	if got := modelIndex("4", 3); got != -1 {
		t.Fatalf("unexpected out-of-range index: %d", got)
	}
}
