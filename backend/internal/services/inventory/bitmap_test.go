package inventory_test

import (
	"testing"

	"github.com/kelvins-io/12306cn/backend/internal/services/inventory"
)

func TestSegmentMaskAndOccupy(t *testing.T) {
	need := inventory.SegmentMask(1, 3) // BC|CD for A-B-C-D
	if need != 0b110 {
		t.Fatalf("mask want 0b110 got %b", need)
	}
	var occ uint64
	if !inventory.Available(occ, need) {
		t.Fatal("should be available")
	}
	occ = inventory.Occupy(occ, need)
	if inventory.Available(occ, need) {
		t.Fatal("should not be available after occupy")
	}
	// AB only
	ab := inventory.SegmentMask(0, 1)
	if !inventory.Available(occ, ab) {
		t.Fatal("AB should still be free")
	}
	occ = inventory.Release(occ, need)
	if occ != 0 {
		t.Fatalf("want 0 after release got %d", occ)
	}
}
