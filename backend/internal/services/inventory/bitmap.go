package inventory

// SegmentMask returns bitmask covering segments [fromSeq, toSeq).
func SegmentMask(fromSeq, toSeq int) uint64 {
	if toSeq <= fromSeq || fromSeq < 0 {
		return 0
	}
	var mask uint64
	for i := fromSeq; i < toSeq; i++ {
		if i >= 64 {
			break
		}
		mask |= 1 << uint(i)
	}
	return mask
}

// Available returns true if none of the required segments are occupied.
func Available(occupancy, need uint64) bool {
	return occupancy&need == 0
}

// Occupy sets the needed bits.
func Occupy(occupancy, need uint64) uint64 {
	return occupancy | need
}

// Release clears the needed bits.
func Release(occupancy, need uint64) uint64 {
	return occupancy &^ need
}

// RemainingCount counts how many seats have free mask among occupancy list.
func RemainingCount(occupancies []uint64, need uint64) int {
	n := 0
	for _, o := range occupancies {
		if Available(o, need) {
			n++
		}
	}
	return n
}
