package amf3

import (
	"errors"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/amf"
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

const (
	MaxInt = (1 << 28) - 1
	MinInt = -(1 << 28)
)

type Writer struct {
	*xio.PacketWriter
	objectref []*amf.Object
}

func NewWriter(buf *xio.PacketWriter) *Writer {
	w := &Writer{}
	w.PacketWriter = buf
	w.objectref = nil
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
		return errors.New("amf3.write type")
	}
	return nil
}

func (w *Writer) Write(v interface{}) error {
	switch v.(type) {
	default:
		return errors.New("amf3.not supported")
	case nil:
		return w.WriteNull()
	case bool:
		return w.WriteBoolean(v.(bool))
	case int:
		return w.WriteInteger(v.(int))
	case float64:
		return w.WriteNumber(v.(float64))
	case string:
		return w.WriteString(v.(string))
	case *time.Time:
		return w.WriteDate(v.(*time.Time))
	case *amf.Object:
		return w.WriteObject(v.(*amf.Object))
	case []byte:
		return w.WriteByteArray(v.([]byte))
	}
}

func (w *Writer) WriteNull() error {
	return w.writeType(Amf3Null)
}

func (w *Writer) WriteBoolean(v bool) error {
	if v {
		return w.writeType(Amf3BooleanTrue)
	} else {
		return w.writeType(Amf3BooleanFalse)
	}
}

func (w *Writer) WriteInteger(v int) error {
	if v > MaxInt || v < MinInt {
		return errors.New("amf3.integer overflow")
	}
	if err := w.writeType(Amf3Integer); err != nil {
		return err
	}
	if err := w.Write7BitValue32(uint32(v & 0x1fffffff)); err != nil {
		return errors.New("amf3.write integer")
	}
	return nil
}

func (w *Writer) WriteNumber(v float64) error {
	if err := w.writeType(Amf3Number); err != nil {
		return err
	}
	return w.writeNumberValue(v)
}

func (w *Writer) writeNumberValue(v float64) error {
	if err := w.WriteFloat64(v); err != nil {
		return errors.New("amf3.write number")
	}
	return nil
}

func (w *Writer) WriteString(s string) error {
	if err := w.writeType(Amf3String); err != nil {
		return err
	}
	return w.writeStringValue(s)
}

func (w *Writer) writeStringValue(s string) error {
	if len(s) == 0 {
		v := uint32(0x01)
		if err := w.Write7BitValue32(v); err != nil {
			return errors.New("amf3.write string.head")
		}
		return nil
	}
	v := uint32((len(s) << 1) | 0x01)
	if err := w.Write7BitValue32(v); err != nil {
		return errors.New("amf3.write string.head")
	}
	if err := w.WriteString(s); err != nil {
		return errors.New("amf3.write string.body")
	}
	return nil
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
	if err := w.writeType(Amf3Date); err != nil {
		return err
	}
	w.addObjectRef(nil)
	v := uint32(0x01)
	if err := w.Write7BitValue32(v); err != nil {
		return errors.New("amf3.write date.head")
	}
	if err := w.writeNumberValue(float64(toMillisecond(t))); err != nil {
		return errors.New("amf3.write date.body")
	}
	return nil
}

func (w *Writer) WriteObject(o *amf.Object) error {
	if o == nil {
		return w.WriteNull()
	}
	if err := w.writeType(Amf3Object); err != nil {
		return err
	}
	return w.writeObjectValue(o)
}

func (w *Writer) writeObjectValue(o *amf.Object) error {
	if idx, ok := w.getObjectRef(o); ok {
		v := uint32(idx << 1)
		if err := w.Write7BitValue32(v); err != nil {
			return errors.New("amf3.write object.head")
		}
		return nil
	}
	w.addObjectRef(o)
	v := uint32((len(o.Values) << 4) | 0x03)
	if err := w.Write7BitValue32(v); err != nil {
		return errors.New("amf3.write object.head")
	}
	if t, err := w.storeTraits(o); err != nil {
		return err
	} else {
		for e := t.infos.Front(); e != nil; e = e.Next() {
			v := o.Values[e.Value.(string)]
			if err := w.Write(v); err != nil {
				return errors.New("amf3.write object.body")
			}
		}
		return nil
	}
}

func (w *Writer) storeTraits(o *amf.Object) (*Traits, error) {
	if err := w.writeStringValue(""); err != nil {
		return nil, errors.New("amf3.write traits.name")
	}
	t := NewTraits()
	for s, _ := range o.Values {
		if err := w.writeStringValue(s); err != nil {
			return nil, errors.New("amf3.write traits.info")
		} else {
			t.add(s)
		}
	}
	return t, nil
}

func (w *Writer) WriteByteArray(buf []byte) error {
	if err := w.writeType(Amf3ByteArray); err != nil {
		return err
	}
	w.addObjectRef(nil)
	v := uint32((len(buf) << 1) | 0x01)
	if err := w.Write7BitValue32(v); err != nil {
		return errors.New("amf3.write byte array.head")
	}
	if err := w.WriteBytes(buf); err != nil {
		return errors.New("amf3.write byte array.body")
	}
	return nil
}
