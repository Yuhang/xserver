package amf0

import (
	"errors"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/amf"
	"github.com/spinlock/xserver/pkg/xserver/amf/amf3"
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

type Writer struct {
	*xio.PacketWriter
	objectref  []*amf.Object
	amf3Writer *amf3.Writer
}

func NewWriter(buf *xio.PacketWriter) *Writer {
	w := &Writer{}
	w.PacketWriter = buf
	w.objectref = nil
	w.amf3Writer = nil
	return w
}

func (w *Writer) addObjectRef(o *amf.Object) {
	if cap(w.objectref) == 0 {
		w.objectref = make([]*amf.Object, 0, 4)
	}
	w.objectref = append(w.objectref, o)
}

func (w *Writer) getObjectRef(o *amf.Object) (int, bool) {
	for i, ref := range w.objectref {
		if ref == o {
			return i, true
		}
	}
	return -1, false
}

func (w *Writer) writeType(t uint8) error {
	if err := w.Write8(t); err != nil {
		return errors.New("amf0.write type")
	}
	return nil
}

func (w *Writer) Write(v interface{}) error {
	switch v.(type) {
	default:
		return errors.New("amf0.not supported")
	case nil:
		return w.WriteNull()
	case bool:
		return w.WriteBoolean(v.(bool))
	case float64:
		return w.WriteNumber(v.(float64))
	case string:
		return w.WriteString(v.(string))
	case *time.Time:
		return w.WriteDate(v.(*time.Time))
	case *amf.Object:
		return w.WriteObject(v.(*amf.Object))
	}
}

func (w *Writer) WriteNull() error {
	return w.writeType(Amf0Null)
}

func (w *Writer) WriteBoolean(v bool) error {
	if err := w.writeType(Amf0Boolean); err != nil {
		return err
	}
	b := uint8(0)
	if v {
		b = uint8(0x01)
	}
	if err := w.Write8(b); err != nil {
		return errors.New("amf0.write boolean")
	}
	return nil
}

func (w *Writer) WriteNumber(v float64) error {
	if err := w.writeType(Amf0Number); err != nil {
		return err
	}
	return w.writeNumberValue(v)
}

func (w *Writer) writeNumberValue(v float64) error {
	if err := w.WriteFloat64(v); err != nil {
		return errors.New("amf0.write number")
	}
	return nil
}

func (w *Writer) WriteString(s string) error {
	if err := w.writeType(Amf0String); err != nil {
		return err
	}
	if err := w.writeStringValue(s); err != nil {
		return errors.New("amf0.write string")
	}
	return nil
}

func (w *Writer) writeStringValue(s string) error {
	return w.WriteString16(s)
}

func toMillisecond(t *time.Time) int64 {
	if t != nil {
		const div = int64(time.Millisecond / time.Nanosecond)
		return t.UnixNano() / div
	} else {
		return 0
	}
}

func (w *Writer) WriteDate(t *time.Time) error {
	if t == nil {
		return w.WriteNull()
	}
	if err := w.writeType(Amf0Date); err != nil {
		return err
	}
	if err := w.writeNumberValue(float64(toMillisecond(t))); err != nil {
		return errors.New("amf0.write date.body")
	}
	if err := w.Write16(0); err != nil {
		return errors.New("amf0.write date.zone")
	}
	return nil
}

func (w *Writer) WriteObject(o *amf.Object) error {
	if o == nil {
		return w.WriteNull()
	}
	if idx, ok := w.getObjectRef(o); ok && idx <= 0xffff {
		return w.writeObjectRef(uint16(idx))
	} else {
		return w.writeObjectValue(o)
	}
}

func (w *Writer) writeObjectRef(ref uint16) error {
	if err := w.writeType(Amf0Reference); err != nil {
		return err
	}
	if err := w.Write16(ref); err != nil {
		return errors.New("amf0.write object.ref")
	}
	return nil
}

func (w *Writer) writeObjectValue(o *amf.Object) error {
	if err := w.writeType(Amf0Object); err != nil {
		return errors.New("amf0.write object.head")
	}
	w.addObjectRef(o)
	for s, v := range o.Values {
		if len(s) == 0 {
			continue
		}
		if err := w.writeStringValue(s); err != nil {
			return errors.New("amf0.write traits.info")
		}
		if err := w.Write(v); err != nil {
			return errors.New("amf0.write object.body")
		}
	}
	if err := w.writeStringValue(""); err != nil {
		return errors.New("amf0.write traits.end")
	}
	if err := w.Write8(Amf0ObjectEnd); err != nil {
		return errors.New("amf0.write object.end")
	}
	return nil
}

func (w *Writer) Write3(v interface{}) error {
	if w.amf3Writer == nil {
		w.amf3Writer = amf3.NewWriter(w.PacketWriter)
	}
	if err := w.writeType(Amf0ToAmf3); err != nil {
		return errors.New("amf0.amf0 to amf3")
	}
	return w.amf3Writer.Write(v)
}
