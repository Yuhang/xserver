package xio

import (
	"io"
	"math"
)

type RawReader struct {
	buffer
}

func NewRawReader(buf []byte) *RawReader {
	r := &RawReader{}
	r.init(buf, 0, 0)
	return r
}

func (r *RawReader) Len() int {
	data := r.bytes()
	roff, _ := r.offset()
	return len(data) - roff
}

func (r *RawReader) Bytes() []byte {
	data := r.bytes()
	roff, _ := r.offset()
	return data[roff:]
}

func (r *RawReader) Read(bs []byte) (int, error) {
	return r.readBytes(bs)
}

func (r *RawReader) Test8() (uint8, error) {
	if v, err := r.testByte(); err != nil {
		return 0, err
	} else {
		return uint8(v), nil
	}
}

func (r *RawReader) Read8() (uint8, error) {
	if v, err := r.readByte(); err != nil {
		return 0, err
	} else {
		return uint8(v), nil
	}
}

func (r *RawReader) Read16() (uint16, error) {
	v := uint16(0)
	for i := 0; i < 2; i++ {
		if b, err := r.readByte(); err != nil {
			return 0, err
		} else {
			v = (v << 8) + uint16(uint8(b))
		}
	}
	return v, nil
}

func (r *RawReader) Read32() (uint32, error) {
	v := uint32(0)
	for i := 0; i < 4; i++ {
		if b, err := r.readByte(); err != nil {
			return 0, err
		} else {
			v = (v << 8) + uint32(uint8(b))
		}
	}
	return v, nil
}

func (r *RawReader) ReadBytes(bs []byte) error {
	if n, err := r.readBytes(bs); err != nil {
		return err
	} else if n != len(bs) {
		return io.EOF
	}
	return nil
}

func (r *RawReader) Read7BitValue32() (uint32, error) {
	v := uint32(0)
	for i := 0; ; i++ {
		if b, err := r.readByte(); err != nil {
			return 0, err
		} else if i < 3 {
			v = (v << 7) + uint32(uint8(b&0x7f))
			if (b & 0x80) == 0 {
				return v, nil
			}
		} else {
			v = (v << 8) + uint32(uint8(b))
			return v, nil
		}
	}
}

func (r *RawReader) Read7BitValue64() (uint64, error) {
	v := uint64(0)
	for i := 0; ; i++ {
		if b, err := r.readByte(); err != nil {
			return 0, err
		} else if i < 7 {
			v = (v << 7) + uint64(uint8(b&0x7f))
			if (b & 0x80) == 0 {
				return v, nil
			}
		} else {
			v = (v << 8) + uint64(uint8(b))
			return v, nil
		}
	}
}

func (r *RawReader) ReadString8() (string, error) {
	if l, err := r.Read8(); err != nil {
		return "", err
	} else {
		bs := make([]byte, l)
		if err := r.ReadBytes(bs); err != nil {
			return "", err
		}
		return string(bs), nil
	}
}

func (r *RawReader) ReadString16() (string, error) {
	if l, err := r.Read16(); err != nil {
		return "", err
	} else {
		bs := make([]byte, l)
		if err := r.ReadBytes(bs); err != nil {
			return "", err
		}
		return string(bs), nil
	}
}

func (r *RawReader) ReadString32() (string, error) {
	if l, err := r.Read32(); err != nil {
		return "", err
	} else {
		bs := make([]byte, l)
		if err := r.ReadBytes(bs); err != nil {
			return "", err
		}
		return string(bs), nil
	}
}

func (r *RawReader) ReadFloat64() (float64, error) {
	bits := uint64(0)
	for i := 0; i < 8; i++ {
		if v, err := r.Read8(); err != nil {
			return 0, err
		} else {
			bits = (bits << 8) + uint64(v)
		}
	}
	return math.Float64frombits(bits), nil
}

type PacketReader struct {
	RawReader
}

func NewPacketReader(buf []byte) *PacketReader {
	r := &PacketReader{}
	r.init(buf, 0, 0)
	return r
}

func (r *PacketReader) SetBytes(buf []byte) {
	r.init(buf, 0, 0)
}

func (r *PacketReader) Skip(n int) error {
	roff, _ := r.offset()
	return r.setoffset(roff+n, 0)
}

func (r *PacketReader) Offset() int {
	roff, _ := r.offset()
	return roff
}

func (r *PacketReader) SetOffset(off int) error {
	return r.setoffset(off, 0)
}
