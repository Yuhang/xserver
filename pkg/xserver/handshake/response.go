package handshake

import (
	"errors"
	"net"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/rtmfp"
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

type packet struct {
	yid uint32
	rsp rtmfp.ResponseMessage
}

func (pkt *packet) Marker() uint8 {
	return 0x0b
}

func (pkt *packet) EchoTime() (bool, int64, uint16) {
	return false, 0, 0
}

func (pkt *packet) Messages() []rtmfp.ResponseMessage {
	return []rtmfp.ResponseMessage{pkt.rsp}
}

type helloResponse struct {
	tag    []byte
	cookie string
}

func (rsp *helloResponse) Code() uint8 {
	return 0x70
}

func (rsp *helloResponse) WriteTo(w *xio.PacketWriter) error {
	if err := w.Write8(uint8(len(rsp.tag))); err != nil {
		return errors.New("hello.write tag.len")
	}
	if err := w.WriteBytes(rsp.tag); err != nil {
		return errors.New("hello.write tag")
	}
	if err := w.WriteString8(rsp.cookie); err != nil {
		return errors.New("hello.write value")
	}
	if err := w.WriteBytes(certificate); err != nil {
		return errors.New("hello.write certificate")
	}
	return nil
}

type handshakeResponse struct {
	tag   []byte
	addrs []*net.UDPAddr
}

func (rsp *handshakeResponse) Code() uint8 {
	return 0x71
}

func (rsp *handshakeResponse) WriteTo(w *xio.PacketWriter) error {
	if err := w.Write8(uint8(len(rsp.tag))); err != nil {
		return errors.New("handshake.write tag.len")
	}
	if err := w.WriteBytes(rsp.tag); err != nil {
		return errors.New("handshake.write tag")
	}
	for i := 0; i < len(rsp.addrs); i++ {
		if err := w.WriteAddress(rsp.addrs[i], i == 0); err != nil {
			return errors.New("handshake.write addr")
		}
	}
	return nil
}

type assignResponse struct {
	xid       uint32
	responder []byte
}

func (rsp *assignResponse) Code() uint8 {
	return 0x78
}

func (rsp *assignResponse) WriteTo(w *xio.PacketWriter) error {
	if err := w.Write32(rsp.xid); err != nil {
		return errors.New("assign.write xid")
	}
	if err := w.Write7BitValue64(uint64(len(rsp.responder))); err != nil {
		return errors.New("assign.write responder size")
	}
	if err := w.WriteBytes(rsp.responder); err != nil {
		return errors.New("assign.write responder")
	}
	if err := w.Write8(0x58); err != nil {
		return errors.New("assign.write mode")
	}
	return nil
}
