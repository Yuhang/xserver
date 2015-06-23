package xio

import (
	"errors"
	"io"
	"math"
	"net"
)

type RawWriter struct {
	buffer
}

func NewRawWriter(buf []byte) *RawWriter {
	w := &RawWriter{}
	w.init(buf, 0, len(buf))
	return w
}

func (w *RawWriter) Len() int {
	return len(w.bytes())
}

func (w *RawWriter) Bytes() []byte {
	return w.bytes()
}

func (w *RawWriter) Write(bs []byte) (int, error) {
	return w.writeBytes(bs)
}

func (w *RawWriter) Write8(v uint8) error {
	return w.writeByte(v)
}

func (w *RawWriter) Write16(v uint16) error {
	s := uint(8)
	for i := 0; i < 2; i++ {
		if err := w.writeByte(uint8(v >> s)); err != nil {
			return err
		}
		s -= 8
	}
	return nil
}

func (w *RawWriter) Write32(v uint32) error {
	s := uint(24)
	for i := 0; i < 4; i++ {
		if err := w.writeByte(uint8(v >> s)); err != nil {
			return err
		}
		s -= 8
	}
	return nil
}

func (w *RawWriter) WriteBytes(bs []byte) error {
	if n, err := w.writeBytes(bs); err != nil {
		return err
	} else if n != len(bs) {
		return io.EOF
	}
	return nil
}

func (w *RawWriter) WriteString(s string) error {
	if n, err := w.writeString(s); err != nil {
		return err
	} else if n != len(s) {
		return io.EOF
	}
	return nil
}

func (w *RawWriter) Write7BitValue32(v uint32) error {
	if v >= (1 << 29) {
		return errors.New("too big number")
	}
	shift, last := uint(0), uint8(v)
	if v >= (1 << 21) {
		shift = 22
	} else {
		for m := uint32(0x80); v >= m; m <<= 7 {
			shift += 7
		}
		last &= 0x7f
	}
	for shift >= 7 {
		if err := w.writeByte(0x80 | uint8(v>>shift)); err != nil {
			return err
		}
		shift -= 7
	}
	if err := w.writeByte(last); err != nil {
		return err
	}
	return nil
}

func SizeOf7BitValue32(v uint32) (int, error) {
	if v >= (1 << 29) {
		return 0, errors.New("too big number")
	}
	if v >= (1 << 21) {
		return 4, nil
	} else {
		n := 1
		for m := uint32(0x80); v >= m; m <<= 7 {
			n++
		}
		return n, nil
	}
}

func (w *RawWriter) Write7BitValue64(v uint64) error {
	if v >= (1 << 57) {
		return errors.New("too big number")
	}
	shift, last := uint(0), uint8(v)
	if v >= (1 << 49) {
		shift = 50
	} else {
		for m := uint64(0x80); v >= m; m <<= 7 {
			shift += 7
		}
		last &= 0x7f
	}
	for shift >= 7 {
		if err := w.writeByte(0x80 | uint8(v>>shift)); err != nil {
			return err
		}
		shift -= 7
	}
	if err := w.writeByte(last); err != nil {
		return err
	}
	return nil
}

func SizeOf7BitValue64(v uint64) (int, error) {
	if v >= (1 << 57) {
		return 0, errors.New("too big number")
	}
	if v >= (1 << 49) {
		return 8, nil
	} else {
		n := 1
		for m := uint64(0x80); v >= m; m <<= 7 {
			n++
		}
		return n, nil
	}
}

func (w *RawWriter) WriteString8(s string) error {
	if l := len(s); l > 0xff {
		return errors.New("too long string")
	} else {
		if err := w.Write8(uint8(l)); err != nil {
			return err
		}
		return w.WriteString(s)
	}
}

func (w *RawWriter) WriteString16(s string) error {
	if l := len(s); l > 0xffff {
		return errors.New("too long string")
	} else {
		if err := w.Write16(uint16(l)); err != nil {
			return err
		}
		return w.WriteString(s)
	}
}

func (w *RawWriter) WriteString32(s string) error {
	if l := len(s); l > 0xffffffff {
		return errors.New("too long string")
	} else {
		if err := w.Write32(uint32(l)); err != nil {
			return err
		}
		return w.WriteString(s)
	}
}

func (w *RawWriter) WriteFloat64(v float64) error {
	bits := math.Float64bits(v)
	for s := 56; s >= 0; s -= 8 {
		if err := w.Write8(uint8(bits >> uint(s))); err != nil {
			return err
		}
	}
	return nil
}

func (w *RawWriter) WriteAddress(addr *net.UDPAddr, public bool) error {
	flag := uint8(0)
	if public {
		flag = 0x02
	} else {
		flag = 0x01
	}
	if len(addr.IP) != 4 {
		flag |= 0x80
	}
	if err := w.Write8(flag); err != nil {
		return err
	}
	if err := w.WriteBytes(addr.IP); err != nil {
		return err
	}
	if err := w.Write16(uint16(addr.Port)); err != nil {
		return err
	}
	return nil
}

type PacketWriter struct {
	RawWriter
}

func NewPacketWriter(buf []byte) *PacketWriter {
	w := &PacketWriter{}
	w.init(buf, 0, len(buf))
	return w
}

func (w *PacketWriter) SetBytes(buf []byte) {
	w.init(buf, 0, len(buf))
}

func (w *PacketWriter) Skip(n int) error {
	_, woff := w.offset()
	return w.setoffset(0, woff+n)
}

func (w *PacketWriter) Offset() int {
	_, woff := w.offset()
	return woff
}

func (w *PacketWriter) SetOffset(off int) error {
	return w.setoffset(0, off)
}

func (w *PacketWriter) Expand(n int) error {
	_, woff := w.offset()
	if err := w.setoffset(0, woff+n); err != nil {
		return err
	}
	return w.setoffset(0, woff)
}
