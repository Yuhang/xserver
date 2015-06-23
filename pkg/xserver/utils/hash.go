package utils

const (
	golden_ratio_prime_32 = uint32(0x9e370001)
)

func Hash16(v uint32) uint16 {
	hash := v * golden_ratio_prime_32
	return uint16(hash >> 11)
}

func Hash16S(s string) uint16 {
	hash := uint32(0)
	for i := 0; i < len(s); i++ {
		hash = hash*31 + uint32(uint8(s[i]))
	}
	return Hash16(hash)
}
