package amf

import (
	"errors"
	"time"
)

type Object struct {
	Values map[string]interface{}
}

func NewObject() *Object {
	o := &Object{}
	o.Values = make(map[string]interface{})
	return o
}

func (o *Object) Set(field string, v interface{}) error {
	switch v.(type) {
	default:
		return errors.New("amf.object.unsupported type")
	case nil, bool, int, float64, string, []byte:
	case *Object:
	}
	o.Values[field] = v
	return nil
}

func (o *Object) SetNull(field string) {
	o.Values[field] = nil
}

func (o *Object) SetBoolean(field string, v bool) {
	o.Values[field] = v
}

func (o *Object) SetInteger(field string, v int) {
	o.Values[field] = v
}

func (o *Object) SetNumber(field string, v float64) {
	o.Values[field] = v
}

func (o *Object) SetDate(field string, t *time.Time) {
	if t == nil {
		o.Values[field] = nil
	} else {
		o.Values[field] = t
	}
}

func (o *Object) SetString(field string, s string) {
	o.Values[field] = s
}

func (o *Object) SetObject(field string, x *Object) {
	if x == nil {
		o.Values[field] = nil
	} else {
		o.Values[field] = x
	}
}

func (o *Object) SetByteArray(field string, bs []byte) {
	o.Values[field] = bs
}

func (o *Object) Get(field string) (interface{}, bool) {
	v, ok := o.Values[field]
	return v, ok
}

func (o *Object) Has(field string) bool {
	_, ok := o.Get(field)
	return ok
}

func (o *Object) GetNull(field string) bool {
	if v, ok := o.Get(field); ok {
		return v == nil
	}
	return false
}

func (o *Object) GetBoolean(field string) (bool, bool) {
	if v, ok := o.Get(field); ok {
		if b, ok := v.(bool); ok {
			return b, true
		}
	}
	return false, false
}

func (o *Object) GetInteger(field string) (int, bool) {
	if v, ok := o.Get(field); ok {
		if i, ok := v.(int); ok {
			return i, true
		}
	}
	return 0, false
}

func (o *Object) GetNumber(field string) (float64, bool) {
	if v, ok := o.Get(field); ok {
		if f, ok := v.(float64); ok {
			return f, true
		}
	}
	return 0, false
}

func (o *Object) GetDate(field string) (*time.Time, bool) {
	if v, ok := o.Get(field); ok {
		if t, ok := v.(*time.Time); ok {
			return t, true
		}
	}
	return nil, false
}

func (o *Object) GetString(field string) (string, bool) {
	if v, ok := o.Get(field); ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

func (o *Object) GetObject(field string) (*Object, bool) {
	if v, ok := o.Get(field); ok {
		if x, ok := v.(*Object); ok {
			return x, true
		}
	}
	return nil, false
}

func (o *Object) GetByteArray(field string) ([]byte, bool) {
	if v, ok := o.Get(field); ok {
		if b, ok := v.([]byte); ok {
			return b, true
		}
	}
	return nil, false
}
