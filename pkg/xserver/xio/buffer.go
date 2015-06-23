package xio

import (
	"errors"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/utils"
)

type buffer struct {
	data       []byte
	roff, woff int
}

func (b *buffer) init(data []byte, roff, woff int) {
	b.data = data
	if b.setoffset(roff, woff) != nil {
		utils.Panic("bad offset")
	}
}

func (b *buffer) bytes() []byte {
	return b.data
}

func (b *buffer) offset() (int, int) {
	return b.roff, b.woff
}

func (b *buffer) setoffset(roff, woff int) error {
	if roff < 0 || roff > len(b.data) || woff < 0 {
		return errors.New("invalid offset")
	}
	b.roff, b.woff = roff, woff
	b.grow(0)
	return nil
}

func (b *buffer) testByte() (byte, error) {
	if b.roff == len(b.data) {
		return 0, errors.New("end of buffer")
	} else {
		v := b.data[b.roff]
		return v, nil
	}
}

func (b *buffer) readByte() (byte, error) {
	if b.roff == len(b.data) {
		return 0, errors.New("end of buffer")
	} else {
		v := b.data[b.roff]
		b.roff++
		return v, nil
	}
}

func (b *buffer) readBytes(bs []byte) (int, error) {
	if n := len(bs); n == 0 {
		return 0, nil
	} else if b.roff == len(b.data) {
		return 0, errors.New("end of buffer")
	} else {
		r := copy(bs, b.data[b.roff:])
		b.roff += r
		return r, nil
	}
}

func (b *buffer) writeByte(v byte) error {
	b.grow(1)
	b.data[b.woff] = v
	b.woff++
	return nil
}

func (b *buffer) writeBytes(bs []byte) (int, error) {
	if n := len(bs); n == 0 {
		return 0, nil
	} else {
		b.grow(n)
		r := copy(b.data[b.woff:], bs)
		b.woff += r
		return r, nil
	}
}

func (b *buffer) writeString(s string) (int, error) {
	if n := len(s); n == 0 {
		return 0, nil
	} else {
		b.grow(n)
		r := copy(b.data[b.woff:], s)
		b.woff += r
		return r, nil
	}
}

func (b *buffer) grow(n int) {
	if end := b.woff + n; end > len(b.data) {
		if end <= cap(b.data) {
			b.data = b.data[:end]
		} else {
			olen, nlen := end, 2048
			for nlen < olen {
				nlen = nlen * 2
			}
			data := make([]byte, nlen)
			copy(data, b.data)
			b.data = data[:olen]
		}
	}
}
