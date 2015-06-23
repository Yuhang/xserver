package handshake

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/args"
	"github.com/spinlock/xserver/pkg/xserver/cookies"
	"github.com/spinlock/xserver/pkg/xserver/counts"
	"github.com/spinlock/xserver/pkg/xserver/rtmfp"
	"github.com/spinlock/xserver/pkg/xserver/session"
	"github.com/spinlock/xserver/pkg/xserver/udp"
	"github.com/spinlock/xserver/pkg/xserver/utils"
	"github.com/spinlock/xserver/pkg/xserver/xio"
	"github.com/spinlock/xserver/pkg/xserver/xlog"
)

var (
	cryptkey = []byte("Adobe Systems 02")
)

type Handshake struct {
	rtmfp.AESEngine
	rtmfp.DHEngine
	lport uint16
	raddr *net.UDPAddr
}

func newHandshake() *Handshake {
	h := &Handshake{}
	h.AESEngine = rtmfp.NewAESEngine()
	if err := h.SetKey(cryptkey, cryptkey); err != nil {
		utils.Panic(fmt.Sprintf("handshake init error = '%v'", err))
	}
	h.DHEngine = rtmfp.NewDHEngine()
	counts.Count("handshake.new", 1)
	return h
}

func HandlePacket(lport uint16, raddr *net.UDPAddr, data []byte) {
	h := getHandshake()
	if h == nil {
		return
	}
	defer putHandshake(h)

	var err error
	if data, err = rtmfp.DecodePacket(h, data); err != nil {
		counts.Count("handshake.decode.error", 1)
		xlog.ErrLog.Printf("[handshake]: decode error = '%v'\n", err)
		return
	}
	xlog.OutLog.Printf("[handshake]: recv addr = [%s], data.len = %d\n%s\n", raddr, len(data), utils.Formatted(data))

	h.lport, h.raddr = lport, raddr

	var pkt *packet
	if pkt, err = h.handle(xio.NewPacketReader(data[6:])); err != nil {
		counts.Count("handshake.handle.error", 1)
		xlog.ErrLog.Printf("[handshake]: handle error = '%v'\n", err)
		return
	} else if pkt == nil {
		xlog.OutLog.Printf("[handshake]: response packet is empty\n")
		return
	}

	if data, err = rtmfp.PacketToBytes(pkt); err != nil {
		counts.Count("handshake.tobytes.error", 1)
		xlog.ErrLog.Printf("[handshake]: packet to bytes error = '%v'\n", err)
		return
	}
	xlog.OutLog.Printf("[handshake]: send addr = [%s], data.len = %d\n%s\n", raddr, len(data), utils.Formatted(data))

	if data, err = rtmfp.EncodePacket(h, pkt.yid, data); err != nil {
		counts.Count("handshake.encode.error", 1)
		xlog.ErrLog.Printf("[handshake]: encode packet error = '%v'\n", err)
		return
	}
	udp.Send(lport, raddr, data)
}

func (h *Handshake) handle(r *xio.PacketReader) (*packet, error) {
	if marker, err := r.Read8(); err != nil {
		return nil, errors.New("packet.read marker")
	} else {
		if _, err := r.Read16(); err != nil {
			return nil, errors.New("packet.read time")
		}
		if marker != 0x0b {
			counts.Count("handshake.marker.unknown", 1)
			return nil, errors.New(fmt.Sprintf("packet.unknown marker = 0x%02x", marker))
		}
	}
	if msg, err := rtmfp.ParseRequestMessage(r); err != nil {
		return nil, err
	} else {
		switch msg.Code {
		default:
			counts.Count("handshake.code.unknown", 1)
			return nil, errors.New(fmt.Sprintf("message.unknown code = 0x%02x", msg.Code))
		case 0x30:
			if rsp, err := h.handleHello(msg.PacketReader); err != nil {
				counts.Count("handshake.hello.error", 1)
				return nil, err
			} else {
				return &packet{0, rsp}, nil
			}
		case 0x38:
			if rsp, yid, err := h.handleAssign(msg.PacketReader); err != nil {
				counts.Count("handshake.assign.error", 1)
				return nil, err
			} else {
				return &packet{yid, rsp}, nil
			}
		}
	}
}

type handshakeError struct {
	msg   string
	pidbs []byte
	raddr *net.UDPAddr
}

func (err *handshakeError) Error() string {
	return fmt.Sprintf("%s, pid = %s, from = %s", err.msg, xlog.BytesToHex(err.pidbs), err.raddr)
}

func (h *Handshake) handleHello(r *xio.PacketReader) (rtmfp.ResponseMessage, error) {
	if req, err := parseHelloRequest(r); err != nil {
		return nil, err
	} else {
		switch req.mode {
		default:
			return nil, errors.New(fmt.Sprintf("hello.unknown mode = 0x%02x", req.mode))
		case 0x0f:
			if s := session.FindByPid(string(req.epd)); s == nil {
				counts.Count("p2p.session.notfound", 1)
				return nil, &handshakeError{"hello.handshake.session not found", req.epd, h.raddr}
			} else if addrs, ok := s.Handshake(req.tag, h.raddr); !ok {
				counts.Count("p2p.session.hasclosed", 1)
				return nil, &handshakeError{"hello.handshake.session has been closed", req.epd, h.raddr}
			} else {
				counts.Count("p2p.handshake", 1)
				return &handshakeResponse{req.tag, addrs}, nil
			}
		case 0x0a:
			if uri, err := url.ParseRequestURI(string(req.epd)); err != nil {
				return nil, errors.New("hello.parse uri")
			} else if app := uri.Path; len(app) == 0 {
				return nil, errors.New("hello.parse app")
			} else {
				if ss := strings.Split(app, "/"); len(ss) != 1 {
					app = ""
					for _, s := range ss {
						if len(s) != 0 {
							app = s
							break
						}
					}
				}
				if len(app) == 0 || !args.IsAuthorizedApp(app) {
					counts.Count("handshake.app.unauthorized", 1)
					return nil, errors.New(fmt.Sprintf("hello.unauthorized app = %s", app))
				}
			}
			cookie := cookies.New()
			if cookie == nil {
				return nil, errors.New("hello.null cookie")
			}
			cookie.Lock()
			defer cookie.Unlock()
			counts.Count("handshake.hello", 1)
			xlog.OutLog.Printf("[handshake]: new cookie from [%s]\n", h.raddr)
			return &helloResponse{req.tag, cookie.Value()}, nil
		}
	}
}

func (h *Handshake) handleAssign(r *xio.PacketReader) (rtmfp.ResponseMessage, uint32, error) {
	if req, err := parseAssignRequest(r); err != nil {
		return nil, 0, err
	} else {
		cookie := cookies.Find(string(req.cookie))
		if cookie == nil {
			return nil, 0, errors.New("assign.cookie not found")
		}
		cookie.Lock()
		defer cookie.Unlock()
		if cookie.Xid == 0 {
			responder, encrypt, decrypt := rtmfp.ComputeSharedKeys(h, req.pubkey, req.initiator)
			cookie.Pid = req.pid
			cookie.Responder = responder
			if xid, err := session.Create(req.yid, cookie.Pid, cookie.Value(), encrypt, decrypt, h.lport, h.raddr); err != nil {
				counts.Count("handshake.session.error", 1)
				return nil, 0, errors.New(fmt.Sprintf("assign.create session = %v", err))
			} else {
				cookie.Xid = xid
				counts.Count("handshake.assign", 1)
				xlog.SssLog.Printf("[join] %s [%s] xid = %d\n", xlog.StringToHex(cookie.Pid), h.raddr, xid)
			}
			xlog.OutLog.Printf("[handshake]: new session xid = %d from [%s]\n", cookie.Xid, h.raddr)
		}
		return &assignResponse{cookie.Xid, cookie.Responder}, req.yid, nil
	}
}
