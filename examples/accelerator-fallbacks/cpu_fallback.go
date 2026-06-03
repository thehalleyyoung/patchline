package fallbackfixture

func Score(seed uint64, feature uint64) uint64 {
	x := seed ^ feature
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33
	return x % 1000000
}
