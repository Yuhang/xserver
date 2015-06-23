package handshake

import (
	"log"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/utils"
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

var (
	certificate []byte
)

func init() {
	prefix := []byte{0x01, 0x0a, 0x41, 0x0e}
	suffix := []byte{0x02, 0x15, 0x02, 0x02, 0x15, 0x05, 0x02, 0x15, 0x0e}
	middle := make([]byte, 64)
	xio.NewRandomReader(time.Now().UnixNano()).ReadBytes(middle)
	magic := make([]byte, len(prefix)+len(middle)+len(suffix))
	copy(magic, prefix)
	copy(magic[len(prefix):], middle)
	copy(magic[len(prefix)+len(middle):], suffix)
	certificate = magic
	log.Printf("[certificate]: \n%s\n", utils.Formatted(certificate))
}
