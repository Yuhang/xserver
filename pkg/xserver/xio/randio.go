package xio

import (
	"math/rand"
)

type RandomReader struct {
	*rand.Rand
}

func NewRandomReader(seed int64) *RandomReader {
	r := &RandomReader{}
	r.Rand = rand.New(rand.NewSource(seed))
	return r
}

func (r *RandomReader) Read(bs []byte) (int, error) {
	r.ReadBytes(bs)
	return len(bs), nil
}

func (r *RandomReader) Read8() uint8 {
	return rand8(r.Rand)
}

func (r *RandomReader) Read16() uint16 {
	return rand16(r.Rand)
}

func (r *RandomReader) Read32() uint32 {
	return rand32(r.Rand)
}

func (r *RandomReader) ReadBytes(bs []byte) error {
	for i := 0; i < len(bs); i++ {
		bs[i] = byte(rand8(r.Rand))
	}
	return nil
}

func rand8(rnd *rand.Rand) uint8 {
	return uint8(rnd.Uint32() >> 24)
}

func rand16(rnd *rand.Rand) uint16 {
	return uint16(rnd.Uint32() >> 16)
}

func rand32(rnd *rand.Rand) uint32 {
	return rnd.Uint32()
}
