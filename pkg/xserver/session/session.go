package session

import (
	"container/list"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/args"
	"github.com/spinlock/xserver/pkg/xserver/async"
	"github.com/spinlock/xserver/pkg/xserver/cookies"
	"github.com/spinlock/xserver/pkg/xserver/counts"
	"github.com/spinlock/xserver/pkg/xserver/rpc"
	"github.com/spinlock/xserver/pkg/xserver/rtmfp"
	"github.com/spinlock/xserver/pkg/xserver/udp"
	"github.com/spinlock/xserver/pkg/xserver/utils"
	"github.com/spinlock/xserver/pkg/xserver/xio"
	"github.com/spinlock/xserver/pkg/xserver/xlog"
)

const (
	maxKeepalive = 6
)

var (
	intKeepalive = int64(args.Heartbeat())
)

type Session struct {
	xid    uint32
	yid    uint32
	pid    string
	cookie string
	lport  uint16
	raddr  *net.UDPAddr
	addrs  []*net.UDPAddr
	closed bool
	manage struct {
		cnt      int
		lasttime int64
	}
	stmptime uint16
	rtmfp.AESEngine
	lastfid uint64
	lastsid uint32
	mainfw  *flowWriter
	readers map[uint64]*flowReader
	writers map[uint64]*flowWriter
	rsplist list.List
	sync.Mutex
}

func (s *Session) Handshake(tag []byte, raddr *net.UDPAddr) ([]*net.UDPAddr, bool) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		counts.Count("session.p2p.closed", 1)
		return nil, false
	}
	defer s.flush()

	xlog.OutLog.Printf("[session]: xid = %d, raddr = [%s], handshake to [%s]\n", s.xid, s.raddr, raddr)

	s.send(&handshakeResponse{s.pid, tag, raddr, true})

	addrs := make([]*net.UDPAddr, 1+len(s.addrs))
	addrs[0] = s.raddr
	copy(addrs[1:], s.addrs)
	return addrs, true
}

func HandlePacket(lport uint16, raddr *net.UDPAddr, xid uint32, data []byte) {
	s := FindByXid(xid)
	if s == nil {
		counts.Count("session.notfound", 1)
		return
	}
	s.Lock()
	defer s.Unlock()
	if s.closed {
		counts.Count("session.hasclosed", 1)
		return
	}
	defer s.flush()

	var err error
	if data, err = rtmfp.DecodePacket(s, data); err != nil {
		counts.Count("session.decode.error", 1)
		xlog.ErrLog.Printf("[session]: decode error = '%v'\n", err)
		return
	}
	xlog.OutLog.Printf("[session]: recv addr = [%s], data.len = %d\n%s\n", raddr, len(data), utils.Formatted(data))

	s.lport, s.raddr = lport, raddr

	if len(s.cookie) != 0 {
		cookies.Commit(s.cookie)
		s.cookie = ""
		rpc.Join(s.xid, s.raddr)
	}

	s.manage.cnt, s.manage.lasttime = 0, time.Now().UnixNano()

	if err = s.handle(xio.NewPacketReader(data[6:])); err != nil {
		counts.Count("session.handle.error", 1)
		xlog.ErrLog.Printf("[session]: handle error = '%v'\n", err)
	}
}

func (s *Session) handle(r *xio.PacketReader) error {
	if marker, err := r.Read8(); err != nil {
		return errors.New("packet.read marker")
	} else {
		if s.stmptime, err = r.Read16(); err != nil {
			return errors.New("packet.read time")
		}
		switch marker | 0xf0 {
		default:
			counts.Count("session.marker.unknown", 1)
			return errors.New(fmt.Sprintf("packet.unknown marker = 0x%02x", marker))
		case 0xfd:
			if _, err = r.Read16(); err != nil {
				return errors.New("packet.read ping time")
			}
		case 0xf9:
		}
	}

	msglist := list.New()
	for r.Len() != 0 {
		if msg, err := rtmfp.ParseRequestMessage(r); err != nil {
			if err != rtmfp.EOP {
				return err
			}
			break
		} else {
			msglist.PushBack(msg)
		}
	}

	var lastreq *flowRequest = nil
	for e := msglist.Front(); e != nil; e = e.Next() {
		msg := e.Value.(*rtmfp.RequestMessage)
		if msg.Code != 0x11 && lastreq != nil {
			if err := s.handleFlowRequest(lastreq); err != nil {
				return err
			}
			lastreq = nil
		}
		switch msg.Code {
		default:
			s.Close()
			counts.Count("session.code.unknown", 1)
			return errors.New(fmt.Sprintf("message.close code = 0x%02x", msg.Code))
		case 0x4c:
			s.Close()
			counts.Count("session.code.close", 1)
			return nil
		case 0x01:
			s.send(newKeepAliveResponse(true))
		case 0x41:
		case 0x5e:
			if req, err := parseFlowErrorRequest(msg.PacketReader); err != nil {
				counts.Count("session.parse5e.error", 1)
				return err
			} else if fw := s.writers[req.fid]; fw != nil {
				fw.reader.handler.OnClose()
			} else {
				xlog.OutLog.Printf("[session]: xid = %d, writer.fid = %d, flow not found 0x5e\n", s.xid, req.fid)
			}
		case 0x51:
			if req, err := parseFlowAckRequest(msg.PacketReader); err != nil {
				counts.Count("session.parse51.error", 1)
				return err
			} else if fw := s.writers[req.fid]; fw != nil {
				fw.CommitAck(req.cnt, req.ack)
			} else {
				xlog.OutLog.Printf("[session]: xid = %d, writer.fid = %d, flow not found 0x51\n", s.xid, req.fid)
			}
		case 0x10:
			if req, err := parseFlowRequest(msg.PacketReader); err != nil {
				counts.Count("session.parse10.error", 1)
				return err
			} else {
				lastreq = req
			}
		case 0x11:
			if req, err := parseFlowRequestSlice(msg.PacketReader); err != nil {
				counts.Count("session.parse11.error", 1)
				return err
			} else if lastreq != nil {
				lastreq.AddSlice(req)
			} else {
				xlog.OutLog.Printf("[session]: xid = %d, not following message\n", s.xid)
			}
		}
	}
	if lastreq != nil {
		return s.handleFlowRequest(lastreq)
	}
	return nil
}

func (s *Session) handleFlowRequest(req *flowRequest) error {
	if fr, err := s.getFlowReader(req.fid, req.signature); err != nil {
		counts.Count("session.flow.error", 1)
		return errors.New("flow.create flow reader")
	} else if fr != nil {
		fr.AddFragments(req.stageack, req.Fragments()...)
		fr.CommitAck()
		return nil
	} else {
		xlog.OutLog.Printf("[session]: xid = %d, reader.fid = %d, flow not found\n", s.xid, req.fid)
		return nil
	}
}

func (s *Session) getFlowReader(fid uint64, signature string) (*flowReader, error) {
	if fr := s.readers[fid]; fr != nil {
		if len(signature) == 0 || signature == fr.signature {
			return fr, nil
		}
		return nil, errors.New("reader.signature.unmatched")
	} else {
		if len(signature) == 0 || s.closed {
			return nil, nil
		}
		if len(signature) <= 4 || signature[:4] != "\x00\x54\x43\x04" {
			return nil, errors.New("reader.signature.unsupported")
		}

		s.lastfid++
		fw := newFlowWriter(s, signature, s.lastfid)
		fr := newFlowReader(s, signature, fid)

		var h messageHandler
		if signature[4:] == "\x00" {
			s.mainfw = fw
			h = newConnHandler(s, fr, fw)
		} else {
			h = newStreamHandler(s, fr, fw)
		}

		fw.reader, fr.handler = fr, h

		s.writers[fw.fid] = fw
		s.readers[fr.fid] = fr
		xlog.OutLog.Printf("[session]: xid = %d, reader.fid = %d, writer.fid = %d, flow created\n", s.xid, fr.fid, fw.fid)
		return fr, nil
	}
}

func (s *Session) Close() {
	if s.closed {
		return
	}
	s.closed = true
	for _, fw := range s.writers {
		fw.reader.handler.OnClose()
	}
	s.send(newErrorResponse())
	counts.Count("session.close", 1)
	xlog.OutLog.Printf("[session]: xid = %d, session closed\n", s.xid)
	rpc.Exit(s.xid, s.raddr)
}

func (s *Session) Manage() bool {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		xlog.OutLog.Printf("[session]: xid = %d, session deleted, closed\n", s.xid)
		return true
	}
	defer s.flush()

	now := time.Now().UnixNano()
	if s.manage.lasttime < now-int64(time.Second)*intKeepalive {
		if cnt := s.manage.cnt; cnt < maxKeepalive {
			s.manage.cnt, s.manage.lasttime = cnt+1, now
			s.send(newKeepAliveResponse(false))
		} else {
			s.Close()
			xlog.OutLog.Printf("[session]: xid = %d, session deleted, timeout\n", s.xid)
			return true
		}
	}

	for _, fw := range s.writers {
		if fw.Manage() {
			if fw.closed {
				fr := fw.reader
				delete(s.readers, fr.fid)
				delete(s.writers, fw.fid)
				xlog.OutLog.Printf("[session]: xid = %d, reader.fid = %d, writer.fid = %d, flow deleted\n", s.xid, fr.fid, fw.fid)
			}
		}
	}
	return false
}

func (s *Session) send(rsp response) {
	s.rsplist.PushBack(rsp)
}

func (s *Session) flush() {
	if s.rsplist.Len() == 0 {
		return
	}
	const limit = 1320
	lastfid, laststage := uint64(0), uint64(0)
	size := 0
	msgs := make([]rtmfp.ResponseMessage, 0, 8)
	for s.rsplist.Len() != 0 {
		e := s.rsplist.Front()
		rsp := e.Value.(response)
		if size += 3 + rsp.SetLastInfo(lastfid, laststage); size <= limit || len(msgs) == 0 {
			s.rsplist.Remove(e)
			lastfid, laststage = rsp.Info()
			msgs = append(msgs, rsp)
			continue
		}
		flush(s, msgs)
		lastfid, laststage = 0, 0
		size = 0
		msgs = msgs[:0]
	}
	if len(msgs) != 0 {
		flush(s, msgs)
	}
}

func flush(s *Session, msgs []rtmfp.ResponseMessage) {
	lport, raddr := s.lport, s.raddr
	if data, err := rtmfp.PacketToBytes(&packet{s.yid, s.manage.lasttime, s.stmptime, msgs}); err != nil {
		counts.Count("session.tobytes.error", 1)
		xlog.ErrLog.Printf("[session]: packet to bytes error = '%v'\n", err)
		return
	} else {
		xlog.OutLog.Printf("[session]: send addr = [%s], data.len = %d\n%s\n", raddr, len(data), utils.Formatted(data))
		if data, err = rtmfp.EncodePacket(s, s.yid, data); err != nil {
			counts.Count("session.encode.error", 1)
			xlog.ErrLog.Printf("[session]: encode packet error = '%v'\n", err)
			return
		}
		udp.Send(lport, raddr, data)
	}
}

func CloseAll(xids []uint32) {
	call := func(s *Session) {
		s.Lock()
		defer s.Unlock()
		s.Close()
	}
	async.Call(uint64(time.Now().UnixNano()), func() {
		for _, xid := range xids {
			if s := FindByXid(xid); s != nil {
				call(s)
			}
		}
	})
}

func RecvPull(xids []uint32, data []byte, reliable bool) {
	if bs, err := newRecvPullMessage(data, reliable); err != nil {
		xlog.ErrLog.Printf("[session]: recvPull error = '%v'\n", err)
	} else {
		data := split(bs)
		call := func(s *Session) {
			s.Lock()
			defer s.Unlock()
			if s.closed {
				return
			}
			defer s.flush()
			if fw := s.mainfw; fw != nil {
				fw.AddFragments(reliable, data...)
			}
		}
		async.Call(uint64(time.Now().UnixNano()), func() {
			for _, xid := range xids {
				if s := FindByXid(xid); s != nil {
					call(s)
				}
			}
		})
	}
}

func Callback(xid uint32, data []byte, callback float64, reliable bool) {
	if bs, err := newCallbackMessage(callback, data); err != nil {
		xlog.ErrLog.Printf("[session]: callback error = '%v'\n", err)
	} else {
		async.Call(uint64(time.Now().UnixNano()), func() {
			if s := FindByXid(xid); s != nil {
				s.Lock()
				defer s.Unlock()
				if s.closed {
					return
				}
				defer s.flush()
				if fw := s.mainfw; fw != nil {
					fw.AddFragments(reliable, split(bs)...)
				}
			}
		})
	}
}

func BroadcastByXid(xids []uint32, data []byte, from uint32, reliable bool) {
	if bs, err := newBroadcastByXidMessage(data, from, reliable); err != nil {
		xlog.ErrLog.Printf("[session]: broadcastByXid error = '%v'\n", err)
	} else {
		data := split(bs)
		call := func(s *Session) {
			s.Lock()
			defer s.Unlock()
			if s.closed {
				return
			}
			defer s.flush()
			if fw := s.mainfw; fw != nil {
				fw.AddFragments(reliable, data...)
			}
		}
		async.Call(uint64(time.Now().UnixNano()), func() {
			for _, xid := range xids {
				if s := FindByXid(xid); s != nil {
					call(s)
				}
			}
		})
	}
}

func newCallbackMessage(callback float64, data []byte) ([]byte, error) {
	if w, err := newAmfMessageWriter("_result", callback); err != nil {
		return nil, err
	} else {
		if err := w.WriteBytes(data); err != nil {
			return nil, err
		}
		return w.Bytes(), nil
	}
}

func newRecvPullMessage(data []byte, reliable bool) ([]byte, error) {
	name := "recvPull"
	if !reliable {
		name = "recvPull2"
	}
	if w, err := newAmfMessageWriter(name, 0); err != nil {
		return nil, err
	} else {
		if err := w.WriteBytes(data); err != nil {
			return nil, err
		}
		return w.Bytes(), nil
	}
}

func newBroadcastByXidMessage(data []byte, from uint32, reliable bool) ([]byte, error) {
	name := "broadcastBySessionId"
	if !reliable {
		name = "broadcastBySessionId2"
	}
	if w, err := newAmfMessageWriter(name, 0); err != nil {
		return nil, err
	} else {
		if err := w.WriteBytes(data); err != nil {
			return nil, err
		}
		if err := w.WriteNumber(float64(from)); err != nil {
			return nil, err
		}
		return w.Bytes(), nil
	}
}
