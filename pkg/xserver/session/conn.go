package session

import (
	"encoding/hex"
	"errors"
	"net"
	"strconv"
	"strings"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/amf"
	"github.com/spinlock/xserver/pkg/xserver/amf/amf0"
	"github.com/spinlock/xserver/pkg/xserver/async"
	"github.com/spinlock/xserver/pkg/xserver/rpc"
	"github.com/spinlock/xserver/pkg/xserver/xio"
	"github.com/spinlock/xserver/pkg/xserver/xlog"
)

type connHandler struct {
	session  *Session
	fr       *flowReader
	fw       *flowWriter
	addrchgi bool
}

func newConnHandler(session *Session, fr *flowReader, fw *flowWriter) *connHandler {
	return &connHandler{session, fr, fw, false}
}

func (h *connHandler) OnAmfMessage(name string, callback float64, r *amf0.Reader) error {
	if h.fw.closed {
		return errors.New("conn.onAmfMessage.closed")
	}
	switch name {
	default:
		return h.onDefault(name, callback, r)
	case "connect":
		return h.onConnect(callback, r)
	case "setPeerInfo":
		return h.onSetPeerInfo(callback, r)
	case "initStream":
		return nil
	case "createStream":
		return h.onCreateStream(callback, r)
	case "deleteStream":
		return nil
	case "setAddressChangeInform":
		return h.onSetAddressChangeInform(callback, r)
	case "request":
		return h.onRequest(callback, r)
	case "relay":
		return h.onRelay(callback, r)
	case "addressChange":
		return h.onAddressChange(callback, r)
	case "proxySend":
		return h.onProxySend(callback, r, true)
	case "proxySend2":
		return h.onProxySend(callback, r, false)
	case "broadcastBySessionId":
		return h.onBroadcastByXid(callback, r, true)
	case "broadcastBySessionId2":
		return h.onBroadcastByXid(callback, r, false)
	}
}

func (h *connHandler) OnClose() {
	if !h.fw.closed {
		h.fw.closed = true
		h.fw.End()
		xlog.OutLog.Printf("[session]: xid = %d, reader.fid = %d, writer.fid = %d, flow closed\n", h.session.xid, h.fr.fid, h.fw.fid)
	}
	h.session.Close()
}

func (h *connHandler) DeceptiveAck() bool {
	return false
}

func (h *connHandler) onDefault(name string, callback float64, r *amf0.Reader) error {
	if err := h.newCallFailedResponse(name, callback); err != nil {
		return errors.New("conn.onAmfData.call failed")
	} else {
		return nil
	}
}

func (h *connHandler) onConnect(callback float64, r *amf0.Reader) error {
	if obj, err := r.ReadObject(); err != nil {
		return errors.New("conn.onConnect.read object")
	} else if amfx, ok := obj.GetNumber("objectEncoding"); !ok {
		return errors.New("conn.onConnect.amf version")
	} else if amfx == 0 {
		if err := h.newAmf0RejectResponse(callback); err != nil {
			return errors.New("conn.onConnect.reject amf0 response")
		}
	} else {
		if err := h.newSuccessResponse(callback, h.session.xid, h.session.raddr); err != nil {
			return errors.New("conn.onConnect.success response")
		}
	}
	return nil
}

func (h *connHandler) onSetPeerInfo(callback float64, r *amf0.Reader) error {
	const (
		keepAliveServer, keepAlivePeer = 1000 * 20, 1000 * 5
	)
	addrs := []*net.UDPAddr{}
	for r.Len() != 0 {
		if s, err := r.ReadString(); err != nil {
			return errors.New("conn.onSetPeerInfo.read address")
		} else if len(s) != 0 {
			if addr, err := net.ResolveUDPAddr("udp", s); err == nil {
				if ip4 := addr.IP.To4(); ip4 != nil {
					addr.IP = ip4
				}
				addrs = append(addrs, addr)
			} else {
				xlog.ErrLog.Printf("[session]: parse addr = %s, error = '%v', addr = [%s]\n", s, addr)
			}
		}
	}
	h.session.addrs = addrs
	if err := h.newKeepAliveResponse(keepAliveServer, keepAlivePeer); err != nil {
		return errors.New("conn.onSetPeerInfo.keep alive response")
	}
	return nil
}

func (h *connHandler) onCreateStream(callback float64, r *amf0.Reader) error {
	for {
		h.session.lastsid++
		if h.session.lastsid != 0 {
			break
		}
	}
	if err := h.newCreateStreamResponse(callback, h.session.lastsid); err != nil {
		return errors.New("conn.onCreateStream.create new stream")
	}
	return nil
}

func (h *connHandler) onSetAddressChangeInform(callback float64, r *amf0.Reader) error {
	h.addrchgi = true
	return nil
}

func (h *connHandler) onRequest(callback float64, r *amf0.Reader) error {
	rpc.Call(h.session.xid, h.session.raddr, callback, r.Bytes(), true)
	return nil
}

func (h *connHandler) onRelay(callback float64, r *amf0.Reader) error {
	if pidss, err := r.ReadString(); err != nil {
		return errors.New("conn.onRelay.read pid")
	} else if pidbs, err := hex.DecodeString(pidss); err != nil || len(pidbs) != 0x20 {
		return errors.New("conn.onRelay.decode pid")
	} else if bs, err := newRelayMessage(h.session.pid, r.Bytes()); err != nil {
		return errors.New("conn.onRelay.generate response")
	} else {
		async.Call(uint64(h.session.xid), func() {
			if s := FindByPid(string(pidbs)); s != nil {
				s.Lock()
				defer s.Unlock()
				if s.closed {
					return
				}
				defer s.flush()
				if fw := s.mainfw; fw != nil {
					fw.AddFragments(true, split(bs)...)
				}
			}
		})
		return nil
	}
}

func (h *connHandler) onAddressChange(callback float64, r *amf0.Reader) error {
	if h.addrchgi {
		if err := h.newAddressChangeResponse(h.session.raddr); err != nil {
			return errors.New("conn.onAddressChange.write response")
		}
	}
	return nil
}

func (h *connHandler) onProxySend(callback float64, r *amf0.Reader, reliable bool) error {
	rpc.Call(h.session.xid, h.session.raddr, 0, r.Bytes(), reliable)
	return nil
}

func (h *connHandler) onBroadcastByXid(callback float64, r *amf0.Reader, reliable bool) error {
	if s, err := r.ReadString(); err != nil {
		return errors.New("conn.onBroadcastByXid.read xids")
	} else if len(s) == 0 {
		return nil
	} else {
		xids := make([]uint32, 0, 32)
		for _, v := range strings.Split(s, "_") {
			if x, err := strconv.ParseInt(v, 10, 64); err != nil {
				return errors.New("conn.onBroadcastByXid.parse xid")
			} else {
				xids = append(xids, uint32(x))
			}
		}
		BroadcastByXid(xids, r.Bytes(), h.session.xid, reliable)
		return nil
	}
}

func (h *connHandler) newAmf0RejectResponse(callback float64) error {
	if w, err := newAmfMessageWriter("_error", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "error")
		obj.SetString("code", "NetConnection.Connect.Rejected")
		obj.SetString("description", "ObjectEncoding client must be in a AMF3 format (not AMF0)")
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *connHandler) newCallFailedResponse(name string, callback float64) error {
	if w, err := newAmfMessageWriter("_error", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "error")
		obj.SetString("code", "NetConnection.Call.Failed")
		obj.SetString("description", "Method '"+name+"' not found")
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *connHandler) newSuccessResponse(callback float64, xid uint32, raddr *net.UDPAddr) error {
	if w, err := newAmfMessageWriter("_result", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "status")
		obj.SetString("code", "NetConnection.Connect.Success")
		obj.SetString("description", "Connection succeeded")
		obj.SetNumber("objectEncoding", 3.0)
		if xid != 0 {
			obj.SetNumber("sessionId", float64(xid))
		}
		if raddr != nil {
			obj.SetString("address", raddr.String())
		}
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *connHandler) newKeepAliveResponse(keepAliveServer, keepAlivePeer uint32) error {
	w := xio.NewPacketWriter(nil)
	if err := w.Write8(0x04); err != nil {
		return err
	}
	if err := w.Write32(0); err != nil {
		return err
	}
	if err := w.Write16(0x29); err != nil {
		return err
	}
	if err := w.Write32(keepAliveServer); err != nil {
		return err
	}
	if err := w.Write32(keepAlivePeer); err != nil {
		return err
	}
	h.fw.AddFragments(true, split(w.Bytes())...)
	return nil
}

func (h *connHandler) newCreateStreamResponse(callback float64, sid uint32) error {
	if w, err := newAmfMessageWriter("_result", callback); err != nil {
		return err
	} else {
		if err := w.WriteNumber(float64(sid)); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *connHandler) newAddressChangeResponse(raddr *net.UDPAddr) error {
	if w, err := newAmfMessageWriter("onIPChange", 0); err != nil {
		return err
	} else {
		if err := w.WriteString(raddr.String()); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func newRelayMessage(pid string, data []byte) ([]byte, error) {
	if w, err := newAmfMessageWriter("onRelay", 0); err != nil {
		return nil, err
	} else {
		if err := w.WriteString(hex.EncodeToString([]byte(pid))); err != nil {
			return nil, err
		}
		if err := w.WriteBytes(data); err != nil {
			return nil, err
		}
		return w.Bytes(), nil
	}
}

func (h *connHandler) OnRawMessage(code uint8, r *xio.PacketReader) error {
	if h.fw.closed {
		return errors.New("conn.onRawMessage.closed")
	}
	if flag, err := r.Read16(); err != nil {
		return errors.New("conn.onRawMessage.read flag")
	} else if flag != 0x03 {
		return errors.New("conn.onRawMessage.unkonwn flag")
	}
	if sid, err := r.Read32(); err != nil {
		return errors.New("conn.onRawMessage.read sid")
	} else if sid != 0 {
		if err := h.newSetBufferTime(sid); err != nil {
			return errors.New("conn.onRawMessage.setbuffertime response")
		}
	}
	return nil
}

func (h *connHandler) newSetBufferTime(sid uint32) error {
	w := xio.NewPacketWriter(nil)
	if err := w.Write8(0x04); err != nil {
		return err
	}
	if err := w.Write32(0); err != nil {
		return err
	}
	if err := w.Write16(0); err != nil {
		return err
	}
	if err := w.Write32(sid); err != nil {
		return err
	}
	h.fw.AddFragments(true, split(w.Bytes())...)
	return nil
}
