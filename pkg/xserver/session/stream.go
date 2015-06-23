package session

import (
	"errors"
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

type streamHandler struct {
	session       *Session
	fr            *flowReader
	fw            *flowWriter
	play, publish struct {
		p        *publication
		callback float64
	}
	bound    uint32
	unstable bool
}

func newStreamHandler(session *Session, fr *flowReader, fw *flowWriter) *streamHandler {
	h := &streamHandler{}
	h.session = session
	h.fr, h.fw = fr, fw
	h.play.p, h.publish.p = nil, nil
	h.bound = 0
	h.unstable = false
	return h
}

func (h *streamHandler) OnAmfMessage(name string, callback float64, r *amf0.Reader) error {
	if h.fw.closed {
		return errors.New("stream.onAmfMessage.closed")
	}
	switch name {
	default:
		return h.onDefault(name, callback, r)
	case "play":
		return h.onPlay(callback, r)
	case "publish":
		return h.onPublish(callback, r)
	case "closeStream":
		return h.onCloseStream(callback, r)
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

func (h *streamHandler) OnClose() {
	h.disenage()
	if !h.fw.closed {
		h.fw.closed = true
		h.fw.End()
		xlog.OutLog.Printf("[session]: xid = %d, reader.fid = %d, writer.fid = %d, flow closed\n", h.session.xid, h.fr.fid, h.fw.fid)
	}
}

func (h *streamHandler) DeceptiveAck() bool {
	return h.unstable
}

func (h *streamHandler) disenage() error {
	if p := h.play.p; p != nil {
		name, callback := p.name, h.play.callback
		h.play.p = nil
		p.remove(h)
		if err := h.newUnplayResponse(name, callback); err != nil {
			return err
		}
	}
	if p := h.publish.p; p != nil {
		name, callback := p.name, h.publish.callback
		h.publish.p, h.unstable = nil, false
		p.stop()
		if !p.rpc {
			call := func(x *streamHandler) {
				s := x.session
				s.Lock()
				defer s.Unlock()
				if s.closed || x.play.p != p {
					return
				}
				defer s.flush()
				x.newUnpublishNotifyResponse(p.name, x.play.callback)
			}
			async.Call(p.gid, func() {
				if l, _ := p.list(); l != nil {
					for e := l.Front(); e != nil; e = e.Next() {
						call(e.Value.(*streamHandler))
					}
				}
			})
		}
		if err := h.newUnpublishResponse(name, callback); err != nil {
			return err
		}
	}
	return nil
}

func (h *streamHandler) onDefault(name string, callback float64, r *amf0.Reader) error {
	if p := h.publish.p; p == nil {
		xlog.OutLog.Printf("[session]: xid = %d, reader.fid = %d, writer.fid = %d, message on non-published stream\n", h.session.xid, h.fr.fid, h.fw.fid)
		return nil
	} else if p.rpc {
		xlog.OutLog.Printf("[session]: xid = %d, reader.fid = %d, writer.fid = %d, unhandled call on rpc stream\n", h.session.xid, h.fr.fid, h.fw.fid)
		return nil
	} else {
		if w, err := newAmfBytesWriter(name); err != nil {
			return errors.New("stream.onAmfData.write name")
		} else if err := w.WriteBytes(r.Bytes()); err != nil {
			return errors.New("stream.onAmfData.write body")
		} else {
			data := split(w.Bytes())
			call := func(x *streamHandler) {
				s := x.session
				s.Lock()
				defer s.Unlock()
				if s.closed || x.play.p != p {
					return
				}
				defer s.flush()
				x.fw.AddFragments(p.reliable, data...)
			}
			async.Call(p.gid, func() {
				if l, ok := p.list(); ok && l != nil {
					for e := l.Front(); e != nil; e = e.Next() {
						call(e.Value.(*streamHandler))
					}
				}
			})
		}
		return nil
	}
}

func (h *streamHandler) onPlay(callback float64, r *amf0.Reader) error {
	if err := h.disenage(); err != nil {
		return errors.New("stream.onPlay.disenage")
	}
	if stream, err := r.ReadString(); err != nil {
		return errors.New("stream.onPlay.read stream")
	} else {
		if p := newPublication(stream); p.add(h) {
			if err := h.newPlayResetResponse(stream, callback); err != nil {
				return errors.New("stream.onPlay.reset response")
			}
			if err := h.newPlaySuccessResponse(stream, callback); err != nil {
				return errors.New("stream.onPlay.play response")
			}
			h.play.p = p
			h.play.callback = callback
			h.bound++
			if err := h.newPlayBoundResponse(h.bound); err != nil {
				return errors.New("stream.onPlay.bound response")
			}
		} else {
			if err := h.newPlayFailedResponse(stream, callback); err != nil {
				return errors.New("stream.onPlay.failed response")
			}
		}
		return nil
	}
}

func (h *streamHandler) onPublish(callback float64, r *amf0.Reader) error {
	if err := h.disenage(); err != nil {
		return errors.New("stream.onPublish.disenage")
	}
	if stream, err := r.ReadString(); err != nil {
		return errors.New("stream.onPublish.read stream")
	} else {
		if p := newPublication(stream); p.start(h) {
			if err := h.newPublishSuccessResponse(stream, callback); err != nil {
				return errors.New("stream.onPublish.publish response")
			}
			h.publish.p, h.unstable = p, !p.reliable
			h.publish.callback = callback
			if !p.rpc {
				call := func(x *streamHandler) {
					s := x.session
					s.Lock()
					defer s.Unlock()
					if s.closed || x.play.p != p {
						return
					}
					defer s.flush()
					x.newPublishNotifyResponse(p.name, x.play.callback)
				}
				async.Call(p.gid, func() {
					if l, _ := p.list(); l != nil {
						for e := l.Front(); e != nil; e = e.Next() {
							call(e.Value.(*streamHandler))
						}
					}
				})
			}
		} else {
			if err := h.newPublishFailedResponse(stream, callback); err != nil {
				return errors.New("stream.onPublish.failed response")
			}
		}
		return nil
	}
}

func (h *streamHandler) onCloseStream(callback float64, r *amf0.Reader) error {
	if err := h.disenage(); err != nil {
		return errors.New("stream.onCloseStream.disenage")
	} else {
		return nil
	}
}

func (h *streamHandler) onProxySend(callback float64, r *amf0.Reader, reliable bool) error {
	rpc.Call(h.session.xid, h.session.raddr, 0, r.Bytes(), reliable)
	return nil
}

func (h *streamHandler) onBroadcastByXid(callback float64, r *amf0.Reader, reliable bool) error {
	if s, err := r.ReadString(); err != nil {
		return errors.New("stream.onBroadcastByXid.read xids")
	} else if len(s) == 0 {
		return nil
	} else {
		xids := make([]uint32, 0, 32)
		for _, v := range strings.Split(s, "_") {
			if x, err := strconv.ParseInt(v, 10, 64); err != nil {
				return errors.New("stream.onBroadcastByXid.parse xid")
			} else {
				xids = append(xids, uint32(x))
			}
		}
		BroadcastByXid(xids, r.Bytes(), h.session.xid, reliable)
		return nil
	}
}

func (h *streamHandler) newUnplayResponse(stream string, callback float64) error {
	if w, err := newAmfMessageWriter("onStatus", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "status")
		obj.SetString("code", "NetStream.Play.Stop")
		obj.SetString("description", "Stopped playing "+stream)
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *streamHandler) newUnpublishResponse(stream string, callback float64) error {
	if w, err := newAmfMessageWriter("onStatus", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "status")
		obj.SetString("code", "NetStream.Unpublish.Success")
		obj.SetString("description", stream+" is now unpublished")
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *streamHandler) newUnpublishNotifyResponse(stream string, callback float64) error {
	if w, err := newAmfMessageWriter("onStatus", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "status")
		obj.SetString("code", "NetStream.Play.UnpublishNotify")
		obj.SetString("description", stream+" is now unpublished")
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *streamHandler) newPublishNotifyResponse(stream string, callback float64) error {
	if w, err := newAmfMessageWriter("onStatus", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "status")
		obj.SetString("code", "NetStream.Play.PublishNotify")
		obj.SetString("description", stream+" is now published")
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *streamHandler) newPlayFailedResponse(stream string, callback float64) error {
	if w, err := newAmfMessageWriter("onStatus", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "status")
		obj.SetString("code", "NetStream.Play.Failed")
		obj.SetString("description", "Play closed stream "+stream)
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *streamHandler) newPlayBoundResponse(bound uint32) error {
	w := xio.NewPacketWriter(nil)
	if err := w.Write8(0x04); err != nil {
		return err
	}
	if err := w.Write32(0); err != nil {
		return err
	}
	if err := w.Write16(0x22); err != nil {
		return err
	}
	if err := w.Write32(bound); err != nil {
		return err
	}
	if err := w.Write32(1); err != nil {
		return err
	}
	h.fw.AddFragments(true, split(w.Bytes())...)
	return nil
}

func (h *streamHandler) newPlayResetResponse(stream string, callback float64) error {
	if w, err := newAmfMessageWriter("onStatus", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "status")
		obj.SetString("code", "NetStream.Play.Reset")
		obj.SetString("description", "Playing and resetting "+stream)
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *streamHandler) newPlaySuccessResponse(stream string, callback float64) error {
	if w, err := newAmfMessageWriter("onStatus", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "status")
		obj.SetString("code", "NetStream.Play.Start")
		obj.SetString("description", "Started playing "+stream)
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *streamHandler) newPublishFailedResponse(stream string, callback float64) error {
	if w, err := newAmfMessageWriter("onStatus", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "status")
		obj.SetString("code", "NetStream.Publish.BadName")
		obj.SetString("description", stream+" is already published")
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *streamHandler) newPublishSuccessResponse(stream string, callback float64) error {
	if w, err := newAmfMessageWriter("onStatus", callback); err != nil {
		return err
	} else {
		obj := amf.NewObject()
		obj.SetString("level", "status")
		obj.SetString("code", "NetStream.Publish.Start")
		obj.SetString("description", stream+" is now published")
		if err := w.WriteObject(obj); err != nil {
			return err
		}
		h.fw.AddFragments(true, split(w.Bytes())...)
		return nil
	}
}

func (h *streamHandler) OnRawMessage(code uint8, r *xio.PacketReader) error {
	if h.fw.closed {
		return errors.New("stream.onRawMessage.closed")
	}
	if flag, err := r.Read16(); err != nil {
		return errors.New("stream.onRawMessage.read flag")
	} else if flag != 0x22 {
		return errors.New("stream.onRawMessage.unknown flag")
	}
	return nil
}
