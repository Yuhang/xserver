package rtmfp

import (
	"errors"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

var (
	EOP = &eop{}
)

func PacketXid(data []byte) (uint32, error) {
	if len(data) < 12 {
		return 0, errors.New("too small packet")
	}
	xid := uint32(0)
	r := xio.NewPacketReader(data)
	for i := 0; i < 3; i++ {
		if v, err := r.Read32(); err != nil {
			return 0, err
		} else {
			xid ^= v
		}
	}
	return xid, nil
}

func DecodePacket(engine AESEngine, data []byte) ([]byte, error) {
	if len(data) < 4+16 {
		return nil, errors.New("packet.too small")
	}
	if err := engine.Decode(data[4:]); err != nil {
		return nil, err
	} else {
		sum1 := uint16(uint8(data[4])) * 256
		sum1 += uint16(uint8(data[5]))
		sum2 := checksum(data[6:])
		if sum1 != sum2 {
			return nil, errors.New("packet.checksum error")
		}
		return data, nil
	}
}

type RequestMessage struct {
	Code uint8
	*xio.PacketReader
}

func ParseRequestMessage(r *xio.PacketReader) (*RequestMessage, error) {
	var err error
	code := uint8(0)
	if code, err = r.Read8(); err != nil {
		return nil, errors.New("message.read code")
	} else if code == 0xff {
		return nil, EOP
	}
	var size uint16
	if size, err = r.Read16(); err != nil {
		return nil, errors.New("message.read size")
	}
	rlen := r.Len()
	mlen := int(size)
	if rlen < mlen {
		return nil, errors.New("message.bad content length")
	} else {
		data := r.Bytes()[:mlen]
		if err := r.Skip(mlen); err != nil {
			return nil, errors.New("message.skip forward")
		}
		return &RequestMessage{code, xio.NewPacketReader(data)}, nil
	}
}

type eop struct {
}

func (e *eop) Error() string {
	return "message.end of packet"
}
