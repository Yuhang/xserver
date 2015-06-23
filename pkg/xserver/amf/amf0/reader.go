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

type Reader struct {
	*xio.PacketReader
	objectref  []interface{}
	amf3Reader *amf3.Reader
}

func NewReader(buf *xio.PacketReader) *Reader {
	r := &Reader{}
	r.PacketReader = buf
	r.objectref = nil
	r.amf3Reader = nil
	return r
}

func (r *Reader) addObjectRef(o interface{}) {
	if cap(r.objectref) == 0 {
		r.objectref = make([]interface{}, 0, 4)
	}
	r.objectref = append(r.objectref, o)
}

func (r *Reader) getObjectRef(i int) (interface{}, error) {
	if i >= 0 && i < len(r.objectref) {
		return r.objectref[i], nil
	} else {
		return nil, errors.New("amf0.objectref out of range")
	}
}

func (r *Reader) readType() (uint8, error) {
	if t, err := r.Read8(); err != nil {
		return 0, errors.New("amf0.read type")
	} else {
		return t, nil
	}
}

func (r *Reader) Read() (interface{}, error) {
	if t, err := r.readType(); err != nil {
		return nil, err
	} else {
		switch t {
		default:
			return nil, errors.New("amf0.not supported")
		case Amf0Undefined:
			return nil, nil
		case Amf0Null:
			return nil, nil
		case Amf0Boolean:
			return r.readBooleanValue()
		case Amf0Number:
			return r.readNumberValue()
		case Amf0String:
			return r.readStringValue()
		case Amf0Date:
			return r.readDateValue()
		case Amf0Object:
			return r.readObjectValue()
		case Amf0Reference:
			return r.readObjectRef()
		case Amf0ToAmf3:
			if r.amf3Reader == nil {
				r.amf3Reader = amf3.NewReader(r.PacketReader)
			}
			return r.amf3Reader.Read()
		}
	}
}

func (r *Reader) TestNull() bool {
	if t, err := r.Test8(); err != nil {
		return false
	} else {
		return t == Amf0Null
	}
}

func (r *Reader) ReadNull() error {
	if t, err := r.readType(); err != nil {
		return err
	} else {
		switch t {
		default:
			return errors.New("amf0.not null")
		case Amf0Null:
			return nil
		}
	}
}

func (r *Reader) ReadBoolean() (bool, error) {
	if t, err := r.readType(); err != nil {
		return false, err
	} else {
		switch t {
		default:
			return false, errors.New("amf0.not boolean")
		case Amf0Null:
			return false, nil
		case Amf0Boolean:
			return r.readBooleanValue()
		}
	}
}

func (r *Reader) readBooleanValue() (bool, error) {
	if v, err := r.Read8(); err != nil {
		return false, errors.New("amf0.read boolean")
	} else {
		return v != 0, nil
	}
}

func (r *Reader) ReadNumber() (float64, error) {
	if t, err := r.readType(); err != nil {
		return 0, err
	} else {
		switch t {
		default:
			return 0, errors.New("amf0.not number")
		case Amf0Null:
			return 0, nil
		case Amf0Number:
			return r.readNumberValue()
		}
	}
}

func (r *Reader) readNumberValue() (float64, error) {
	if v, err := r.ReadFloat64(); err != nil {
		return 0, errors.New("amf0.read number")
	} else {
		return v, nil
	}
}

func (r *Reader) ReadString() (string, error) {
	if t, err := r.readType(); err != nil {
		return "", err
	} else {
		switch t {
		default:
			return "", errors.New("amf0.not string")
		case Amf0Null:
			return "", nil
		case Amf0String:
			return r.readStringValue()
		}
	}
}

func (r *Reader) readStringValue() (string, error) {
	return r.ReadString16()
}

func toTime(ms int64) *time.Time {
	const div1 = int64(time.Second / time.Millisecond)
	const div2 = int64(time.Millisecond / time.Nanosecond)
	t := time.Unix(ms/div1, (ms%div1)*div2)
	return &t
}

func (r *Reader) ReadDate() (*time.Time, error) {
	if t, err := r.readType(); err != nil {
		return nil, err
	} else {
		switch t {
		default:
			return nil, errors.New("amf0.not date")
		case Amf0Null:
			return toTime(0), nil
		case Amf0Date:
			return r.readDateValue()
		}
	}
}

func (r *Reader) readDateValue() (*time.Time, error) {
	if d, err := r.readNumberValue(); err != nil {
		return nil, errors.New("amf0.read date.body")
	} else {
		if _, err := r.Read16(); err != nil {
			return nil, errors.New("amf0.read date.zone")
		}
		t := toTime(int64(d))
		return t, nil
	}
}

func (r *Reader) ReadObject() (*amf.Object, error) {
	if t, err := r.readType(); err != nil {
		return nil, err
	} else {
		switch t {
		default:
			return nil, errors.New("amf0.not object")
		case Amf0Null:
			return nil, nil
		case Amf0Object:
			return r.readObjectValue()
		}
	}
}

func (r *Reader) readObjectRef() (*amf.Object, error) {
	if ref, err := r.Read16(); err != nil {
		return nil, errors.New("amf0.read object.ref")
	} else {
		if o, err := r.getObjectRef(int(ref)); err != nil {
			return nil, err
		} else if obj, ok := o.(*amf.Object); ok {
			return obj, nil
		} else {
			return nil, errors.New("amf0.ref not object")
		}
	}
}

func (r *Reader) readObjectValue() (*amf.Object, error) {
	o := amf.NewObject()
	r.addObjectRef(o)
	for {
		if s, err := r.readStringValue(); err != nil {
			return nil, errors.New("amf0.read traits.info")
		} else {
			if len(s) == 0 {
				break
			}
			if v, err := r.Read(); err != nil {
				return nil, errors.New("amf0.read object.body")
			} else if err := o.Set(s, v); err != nil {
				return nil, err
			}
		}
	}
	if t, err := r.Read8(); err != nil {
		return nil, errors.New("amf0.read object.end")
	} else if t != Amf0ObjectEnd {
		return nil, errors.New("amf0.not object end")
	}
	return o, nil
}
