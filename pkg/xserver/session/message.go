package session

import (
	"errors"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/amf/amf0"
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

type messageHandler interface {
	OnClose()
	OnAmfMessage(name string, callback float64, r *amf0.Reader) error
	OnRawMessage(code uint8, r *xio.PacketReader) error
	DeceptiveAck() bool
}

func handleMessage(h messageHandler, r *xio.PacketReader) error {
	if code, err := r.Read8(); err != nil {
		return errors.New("message.read code")
	} else {
		switch code {
		default:
			return h.OnRawMessage(code, r)
		case 0x11:
			if err := r.Skip(5); err != nil {
				return errors.New("message.skip useless")
			}
			return handleAmfMessage(h, amf0.NewReader(r), true)
		case 0x14:
			if err := r.Skip(4); err != nil {
				return errors.New("message.skip useless")
			}
			return handleAmfMessage(h, amf0.NewReader(r), true)
		case 0x0f:
			if err := r.Skip(5); err != nil {
				return errors.New("message.skip useless")
			}
			return handleAmfMessage(h, amf0.NewReader(r), false)
		}
	}
}

func handleAmfMessage(h messageHandler, r *amf0.Reader, withcallback bool) error {
	var err error
	name := ""
	if name, err = r.ReadString(); err != nil {
		return errors.New("message.amf.read name")
	}
	callback := float64(0)
	if withcallback {
		if callback, err = r.ReadNumber(); err != nil {
			return errors.New("message.amf.read callback")
		}
		if r.Len() != 0 && r.TestNull() {
			if err := r.ReadNull(); err != nil {
				return errors.New("message.amf.read null")
			}
		}
	}
	return h.OnAmfMessage(name, callback, r)
}

func newAmfMessageWriter(name string, callback float64) (*amf0.Writer, error) {
	w := amf0.NewWriter(xio.NewPacketWriter(nil))
	if err := w.Write8(0x14); err != nil {
		return nil, errors.New("message.amf.write code")
	}
	if err := w.Write32(0); err != nil {
		return nil, errors.New("message.amf.write useless")
	}
	if err := w.WriteString(name); err != nil {
		return nil, errors.New("message.amf.write name")
	}
	if err := w.WriteNumber(callback); err != nil {
		return nil, errors.New("message.amf.write callback")
	}
	if err := w.WriteNull(); err != nil {
		return nil, errors.New("message.amf.write null")
	}
	return w, nil
}

func newAmfBytesWriter(name string) (*amf0.Writer, error) {
	w := amf0.NewWriter(xio.NewPacketWriter(nil))
	if err := w.Write8(0x0f); err != nil {
		return nil, errors.New("message.amf.write code")
	}
	if err := w.Write8(0); err != nil {
		return nil, errors.New("message.amf.write useless1")
	}
	if err := w.Write32(0); err != nil {
		return nil, errors.New("message.amf.write useless2")
	}
	if err := w.WriteString(name); err != nil {
		return nil, errors.New("message.amf.write name")
	}
	return w, nil
}
