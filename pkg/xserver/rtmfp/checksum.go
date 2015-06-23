package rtmfp

import (
	"github.com/spinlock/xserver/pkg/xserver/utils"
)

func checksum(bs []byte) uint16 {
	if len(bs)%2 != 0 {
		utils.Panic("checksum data length")
	}
	sum := uint32(0)
	cnt := len(bs) / 2
	for i := 0; i < cnt; i++ {
		sum += uint32(uint8(bs[i*2])) * 256
		sum += uint32(uint8(bs[i*2+1]))
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum = (sum >> 16) + (sum & 0xffff)
	return uint16(^sum)
}
