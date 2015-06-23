package session

import (
	"github.com/spinlock/xserver/pkg/xserver/rtmfp"
	"github.com/spinlock/xserver/pkg/xserver/xio"
	"net"
)

type packet struct {
	yid      uint32
	recvtime int64
	stmptime uint16
	msgs     []rtmfp.ResponseMessage
}

func (pkt *packet) Yid() uint32 {
	return pkt.yid
}

func (pkt *packet) Marker() uint8 {
	return 0x4a
}

func (pkt *packet) EchoTime() (bool, int64, uint16) {
	return true, pkt.recvtime, pkt.stmptime
}

func (pkt *packet) Messages() []rtmfp.ResponseMessage {
	return pkt.msgs
}

type response interface {
	rtmfp.ResponseMessage
	Info() (uint64, uint64)
	SetLastInfo(lastfid, laststage uint64) int
}

type flowResponse struct {
	header bool
	fw     struct {
		fid       uint64
		signature string
	}
	fr struct {
		fid uint64
	}
	f struct {
		stage uint64
		delta uint64
		flags uint8
		data  []byte
	}
}

func newFlowResponse(fw *flowWriter, f *fragment, stageack uint64) *flowResponse {
	rsp := &flowResponse{}
	rsp.header = false
	rsp.fw.fid = fw.fid
	rsp.fw.signature = ""
	rsp.fr.fid = 0
	if stageack == 0 {
		rsp.fw.signature = fw.signature
		rsp.fr.fid = fw.reader.fid
	}
	rsp.f.stage = f.stage
	rsp.f.flags = f.flags
	rsp.f.delta = 0
	if f.stage > stageack {
		rsp.f.delta = f.stage - stageack
	}
	rsp.f.data = f.data
	return rsp
}

func (rsp *flowResponse) Info() (uint64, uint64) {
	return rsp.fw.fid, rsp.f.stage
}

func (rsp *flowResponse) SetLastInfo(lastfid, laststage uint64) int {
	if (rsp.fw.fid == lastfid) && (rsp.f.stage == laststage+1) {
		rsp.header = false
	} else {
		rsp.header = true
	}
	size := 1 + len(rsp.f.data)
	if rsp.header {
		if add, err := xio.SizeOf7BitValue64(rsp.fw.fid); err == nil {
			size += add
		}
		if add, err := xio.SizeOf7BitValue64(rsp.f.stage); err == nil {
			size += add
		}
		if add, err := xio.SizeOf7BitValue64(rsp.f.delta); err == nil {
			size += add
		}
		if len(rsp.fw.signature) != 0 {
			size += 1 + len(rsp.fw.signature)
			if rsp.fr.fid != 0 {
				more := 1
				if add, err := xio.SizeOf7BitValue64(rsp.fr.fid); err == nil {
					more += add
				}
				if add, err := xio.SizeOf7BitValue64(uint64(more)); err == nil {
					size += add
				}
				size += more
			}
		}
		size += 1
	}
	return size
}

func (rsp *flowResponse) Code() uint8 {
	if rsp.header {
		return 0x10
	} else {
		return 0x11
	}
}

func (rsp *flowResponse) WriteTo(w *xio.PacketWriter) error {
	flags := rsp.f.flags
	if rsp.header {
		flags |= flagsHeader
	}
	if err := w.Write8(flags); err != nil {
		return err
	}
	if rsp.header {
		if err := w.Write7BitValue64(rsp.fw.fid); err != nil {
			return err
		}
		if err := w.Write7BitValue64(rsp.f.stage); err != nil {
			return err
		}
		if err := w.Write7BitValue64(rsp.f.delta); err != nil {
			return err
		}
		if len(rsp.fw.signature) != 0 {
			if err := w.WriteString8(rsp.fw.signature); err != nil {
				return err
			}
			if rsp.fr.fid != 0 {
				more := 1
				if add, err := xio.SizeOf7BitValue64(rsp.fr.fid); err != nil {
					return err
				} else {
					more += add
				}
				if err := w.Write7BitValue64(uint64(more)); err != nil {
					return err
				}
				if err := w.Write8(0x0a); err != nil {
					return err
				}
				if err := w.Write7BitValue64(rsp.fr.fid); err != nil {
					return err
				}
			}
		}
		if err := w.Write8(0); err != nil {
			return err
		}
	}
	if err := w.WriteBytes(rsp.f.data); err != nil {
		return err
	}
	return nil
}

type keepAliveResponse struct {
	passive bool
}

func newKeepAliveResponse(passive bool) *keepAliveResponse {
	return &keepAliveResponse{passive}
}

func (rsp *keepAliveResponse) Info() (uint64, uint64) {
	return 0, 0
}

func (rsp *keepAliveResponse) SetLastInfo(lastfid, laststage uint64) int {
	return 0
}

func (rsp *keepAliveResponse) Code() uint8 {
	if rsp.passive {
		return 0x41
	} else {
		return 0x01
	}
}

func (rsp *keepAliveResponse) WriteTo(w *xio.PacketWriter) error {
	return nil
}

type flowAckResponse struct {
	fid uint64
	cnt uint64
	ack *flowAck
}

func newFlowAckResponse(fid uint64, cnt uint64, ack *flowAck) *flowAckResponse {
	return &flowAckResponse{fid, cnt, ack}
}

func (rsp *flowAckResponse) Info() (uint64, uint64) {
	return 0, 0
}

func (rsp *flowAckResponse) SetLastInfo(lastfid, laststage uint64) int {
	size := 0
	if add, err := xio.SizeOf7BitValue64(rsp.fid); err == nil {
		size += add
	}
	if add, err := xio.SizeOf7BitValue64(rsp.cnt); err == nil {
		size += add
	}
	if add, err := rsp.ack.Size(); err != nil {
		size += add
	}
	return size
}

func (rsp *flowAckResponse) Code() uint8 {
	return 0x51
}

func (rsp *flowAckResponse) WriteTo(w *xio.PacketWriter) error {
	if err := w.Write7BitValue64(rsp.fid); err != nil {
		return err
	}
	if err := w.Write7BitValue64(rsp.cnt); err != nil {
		return err
	}
	if err := storeFlowAck(rsp.ack, w); err != nil {
		return err
	}
	return nil
}

type errorResponse struct {
}

func newErrorResponse() *errorResponse {
	return &errorResponse{}
}

func (rsp *errorResponse) Info() (uint64, uint64) {
	return 0, 0
}

func (rsp *errorResponse) SetLastInfo(lastfid, laststage uint64) int {
	return 0
}

func (rsp *errorResponse) Code() uint8 {
	return 0x0c
}

func (rsp *errorResponse) WriteTo(w *xio.PacketWriter) error {
	return nil
}

type handshakeResponse struct {
	pid    string
	tag    []byte
	addr   *net.UDPAddr
	public bool
}

func (rsp *handshakeResponse) Info() (uint64, uint64) {
	return 0, 0
}

func (rsp *handshakeResponse) SetLastInfo(lastfid, laststage uint64) int {
	return 0
}

func (rsp *handshakeResponse) Code() uint8 {
	return 0x0f
}

func (rsp *handshakeResponse) WriteTo(w *xio.PacketWriter) error {
	if err := w.Write8(0x22); err != nil {
		return err
	}
	if err := w.Write8(0x21); err != nil {
		return err
	}
	if err := w.Write8(0x0f); err != nil {
		return err
	}
	if err := w.WriteBytes([]byte(rsp.pid)); err != nil {
		return err
	}
	if err := w.WriteAddress(rsp.addr, rsp.public); err != nil {
		return err
	}
	if err := w.WriteBytes(rsp.tag); err != nil {
		return err
	}
	return nil
}
