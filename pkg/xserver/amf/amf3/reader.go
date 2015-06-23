package amf3

import (
	"errors"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/amf"
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

type Reader struct {
	*xio.PacketReader
	stringref []string
	objectref []interface{}
	traitsref []*Traits
}

func NewReader(buf *xio.PacketReader) *Reader {
	r := &Reader{}
	r.PacketReader = buf
	r.stringref = nil
	r.objectref = nil
	r.traitsref = nil
	return r
}

func (r *Reader) addStringRef(s string) {
	if cap(r.stringref) == 0 {
		r.stringref = make([]string, 0, 4)
	}
	r.stringref = append(r.stringref, s)
}

func (r *Reader) getStringRef(i int) (string, error) {
	if i >= 0 && i < len(r.stringref) {
		return r.stringref[i], nil
	} else {
		return "", errors.New("amf3.stringref out of range")
	}
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
		return nil, errors.New("amf3.objectref out of range")
	}
}

func (r *Reader) addTraitsRef(t *Traits) {
	if cap(r.traitsref) == 0 {
		r.traitsref = make([]*Traits, 0, 4)
	}
	r.traitsref = append(r.traitsref, t)
}

func (r *Reader) getTraitsRef(i int) (*Traits, error) {
	if i >= 0 && i < len(r.traitsref) {
		return r.traitsref[i], nil
	} else {
		return nil, errors.New("amf3.traitsref out of range")
	}
}

func (r *Reader) readType() (uint8, error) {
	if t, err := r.Read8(); err != nil {
		return 0, errors.New("amf3.read type")
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
			return nil, errors.New("amf3.not supported")
		case Amf3Undefined:
			return nil, nil
		case Amf3Null:
			return nil, nil
		case Amf3BooleanFalse:
			return false, nil
		case Amf3BooleanTrue:
			return true, nil
		case Amf3Integer:
			return r.readIntegerValue()
		case Amf3Number:
			return r.readNumberValue()
		case Amf3String:
			return r.readStringValue()
		case Amf3Date:
			return r.readDateValue()
		case Amf3Object:
			return r.readObjectValue()
		case Amf3ByteArray:
			return r.readByteArrayValue()
		}
	}
}

func (r *Reader) TestNull() bool {
	if t, err := r.Test8(); err != nil {
		return false
	} else {
		return t == Amf3Null
	}
}

func (r *Reader) ReadNull() error {
	if t, err := r.readType(); err != nil {
		return err
	} else {
		switch t {
		default:
			return errors.New("amf3.not null")
		case Amf3Null:
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
			return false, errors.New("amf3.not boolean")
		case Amf3Null:
			return false, nil
		case Amf3BooleanFalse:
			return false, nil
		case Amf3BooleanTrue:
			return true, nil
		}
	}
}

func (r *Reader) ReadInteger() (int, error) {
	if t, err := r.readType(); err != nil {
		return 0, err
	} else {
		switch t {
		default:
			return 0, errors.New("amf3.not integer")
		case Amf3Null:
			return 0, nil
		case Amf3Integer:
			return r.readIntegerValue()
		}
	}
}

func (r *Reader) readIntegerValue() (int, error) {
	if v, err := r.Read7BitValue32(); err != nil {
		return 0, errors.New("amf3.read integer")
	} else {
		if (v & (1 << 28)) == 0 {
			return int(v), nil
		} else {
			return int(int32(v | 0xe0000000)), nil
		}
	}
}

func (r *Reader) ReadNumber() (float64, error) {
	if t, err := r.readType(); err != nil {
		return 0, err
	} else {
		switch t {
		default:
			return 0, errors.New("amf3.not number")
		case Amf3Null:
			return 0, nil
		case Amf3Number:
			return r.readNumberValue()
		}
	}
}

func (r *Reader) readNumberValue() (float64, error) {
	if v, err := r.ReadFloat64(); err != nil {
		return 0, errors.New("amf3.read number")
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
			return "", errors.New("amf3.not string")
		case Amf3Null:
			return "", nil
		case Amf3String:
			return r.readStringValue()
		}
	}
}

func (r *Reader) readStringValue() (string, error) {
	if v, err := r.Read7BitValue32(); err != nil {
		return "", errors.New("amf3.read string.head")
	} else {
		if (v & 0x01) == 0 {
			return r.getStringRef(int(v >> 1))
		} else {
			if l := int(v >> 1); l == 0 {
				return "", nil
			} else {
				buf := make([]byte, l)
				if err := r.ReadBytes(buf); err != nil {
					return "", errors.New("amf3.read string.body")
				}
				s := string(buf)
				r.addStringRef(s)
				return s, nil
			}
		}
	}
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
			return nil, errors.New("amf3.not date")
		case Amf3Null:
			return toTime(0), nil
		case Amf3Date:
			return r.readDateValue()
		}
	}
}

func (r *Reader) readDateValue() (*time.Time, error) {
	if v, err := r.Read7BitValue32(); err != nil {
		return nil, errors.New("amf3.read date.head")
	} else {
		if (v & 0x01) == 0 {
			if o, err := r.getObjectRef(int(v >> 1)); err != nil {
				return nil, err
			} else if t, ok := o.(*time.Time); ok {
				return t, nil
			} else {
				return nil, errors.New("amf3.ref not date")
			}
		} else {
			if d, err := r.readNumberValue(); err != nil {
				return nil, errors.New("amf3.read date.body")
			} else {
				t := toTime(int64(d))
				r.addObjectRef(t)
				return t, nil
			}
		}
	}
}

func (r *Reader) ReadObject() (*amf.Object, error) {
	if t, err := r.readType(); err != nil {
		return nil, err
	} else {
		switch t {
		default:
			return nil, errors.New("amf3.not object")
		case Amf3Null:
			return nil, nil
		case Amf3Object:
			return r.readObjectValue()
		}
	}
}

func (r *Reader) readObjectValue() (*amf.Object, error) {
	if v, err := r.Read7BitValue32(); err != nil {
		return nil, errors.New("amf3.read object.head")
	} else {
		if (v & 0x01) == 0 {
			if o, err := r.getObjectRef(int(v >> 1)); err != nil {
				return nil, err
			} else if obj, ok := o.(*amf.Object); ok {
				return obj, nil
			} else {
				return nil, errors.New("amf3.ref not object")
			}
		} else {
			if t, err := r.loadTraits(v); err != nil {
				return nil, err
			} else {
				o := amf.NewObject()
				r.addObjectRef(o)
				for e := t.infos.Front(); e != nil; e = e.Next() {
					s := e.Value.(string)
					if v, err := r.Read(); err != nil {
						return nil, errors.New("amf3.read object.body")
					} else if err := o.Set(s, v); err != nil {
						return nil, err
					}
				}
				return o, nil
			}
		}
	}
}

func (r *Reader) loadTraits(ref uint32) (*Traits, error) {
	if (ref & 0x03) == 0x01 {
		return r.getTraitsRef(int(ref >> 2))
	} else if (ref & 0x0f) != 0x03 {
		return nil, errors.New("amf3.not supported.traits")
	} else {
		if _, err := r.readStringValue(); err != nil {
			return nil, errors.New("amf3.read traits.name")
		}
		t := NewTraits()
		r.addTraitsRef(t)
		n := int(ref >> 4)
		for i := 0; i < n; i++ {
			if s, err := r.readStringValue(); err != nil {
				return nil, errors.New("amf3.read traits.info")
			} else {
				t.add(s)
			}
		}
		return t, nil
	}
}

func (r *Reader) ReadByteArray() ([]byte, error) {
	if t, err := r.readType(); err != nil {
		return nil, err
	} else {
		switch t {
		default:
			return nil, errors.New("amf3.not byte array")
		case Amf3Null:
			return nil, nil
		case Amf3ByteArray:
			return r.readByteArrayValue()
		}
	}
}

func (r *Reader) readByteArrayValue() ([]byte, error) {
	if v, err := r.Read7BitValue32(); err != nil {
		return nil, errors.New("amf3.read byte array.head")
	} else {
		if (v & 0x01) == 0 {
			if o, err := r.getObjectRef(int(v >> 1)); err != nil {
				return nil, err
			} else if buf, ok := o.([]byte); ok {
				return buf, nil
			} else {
				return nil, errors.New("amf3.ref not byte array")
			}
		} else {
			buf := make([]byte, int(v>>1))
			if err := r.ReadBytes(buf); err != nil {
				return nil, errors.New("amf3.read byte array.body")
			}
			r.addObjectRef(buf)
			return buf, nil
		}
	}
}
