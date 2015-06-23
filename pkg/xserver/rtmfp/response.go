package rtmfp

import (
	"errors"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

var (
	paddings [][]byte
)

func init() {
	paddings = make([][]byte, AESBlockSize)
	for i := 0; i < AESBlockSize; i++ {
		p := make([]byte, (AESBlockSize-i)%AESBlockSize)
		for j := 0; j < len(p); j++ {
			p[j] = 0xff
		}
		paddings[i] = p
	}
}

func EncodePacket(engine AESEngine, yid uint32, data []byte) ([]byte, error) {
	if len(data) < 6 {
		return nil, errors.New("packet.too small")
	}
	w := xio.NewPacketWriter(data)
	if n := (w.Len() - 4) % AESBlockSize; n != 0 {
		if err := w.WriteBytes(paddings[n]); err != nil {
			return nil, err
		}
	}
	sum := checksum(w.Bytes()[6:])
	if err := w.SetOffset(4); err != nil {
		return nil, err
	}
	if err := w.Write8(uint8(sum >> 8)); err != nil {
		return nil, err
	}
	if err := w.Write8(uint8(sum)); err != nil {
		return nil, err
	}
	data = w.Bytes()
	if err := engine.Encode(data[4:]); err != nil {
		return nil, err
	}
	r := xio.NewPacketReader(data)
	if err := r.SetOffset(4); err != nil {
		return nil, err
	}
	for i := 0; i < 2; i++ {
		if v, err := r.Read32(); err != nil {
			return nil, err
		} else {
			yid ^= v
		}
	}
	w.SetBytes(data)
	if err := w.SetOffset(0); err != nil {
		return nil, err
	}
	if err := w.Write32(yid); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

type ResponsePacket interface {
	Marker() uint8
	EchoTime() (bool, int64, uint16)
	Messages() []ResponseMessage
}

type ResponseMessage interface {
	Code() uint8
	WriteTo(w *xio.PacketWriter) error
}

func PacketToBytes(pkt ResponsePacket) ([]byte, error) {
	now := time.Now().UnixNano()
	marker := pkt.Marker()
	echotime, recvtime, stmptime := pkt.EchoTime()
	if echotime {
		if recvtime+30*int64(time.Second) < now {
			echotime = false
		} else {
			marker += 0x04
		}
	}
	w := xio.NewPacketWriter(nil)
	if err := w.SetOffset(6); err != nil {
		return nil, errors.New("packet.skip header")
	}
	if err := w.Write8(marker); err != nil {
		return nil, errors.New("packet.write marker")
	}
	const div = int64(time.Millisecond)
	if err := w.Write16(uint16(now / div)); err != nil {
		return nil, errors.New("packet.write time.now")
	}
	if echotime {
		if err := w.Write16(stmptime + uint16((now-recvtime)/div)); err != nil {
			return nil, errors.New("packet.write time.ping")
		}
	}
	for _, rsp := range pkt.Messages() {
		if err := w.Write8(rsp.Code()); err != nil {
			return nil, errors.New("message.write code")
		}
		pos := w.Offset()
		if err := w.Skip(2); err != nil {
			return nil, errors.New("message.skip size")
		}
		if err := rsp.WriteTo(w); err != nil {
			return nil, errors.New("message.write content")
		}
		beg, end := pos+2, w.Offset()
		if err := w.SetOffset(pos); err != nil {
			return nil, errors.New("message.skip back")
		}
		if end < beg || end > beg+0xffff {
			return nil, errors.New("message.bad content length")
		} else if err := w.Write16(uint16(end - beg)); err != nil {
			return nil, errors.New("message.write size")
		}
		if err := w.SetOffset(end); err != nil {
			return nil, errors.New("message.skip to end")
		}
	}
	return w.Bytes(), nil
}
