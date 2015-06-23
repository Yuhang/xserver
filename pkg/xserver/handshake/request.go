package handshake

import (
	"crypto/sha256"
	"errors"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/cookies"
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

type helloRequest struct {
	epd  []byte
	tag  []byte
	mode uint8
}

func parseHelloRequest(r *xio.PacketReader) (*helloRequest, error) {
	var err error
	if _, err = r.Read8(); err != nil {
		return nil, errors.New("hello.read ignore")
	}
	size := uint8(0)
	if size, err = r.Read8(); err != nil {
		return nil, errors.New("hello.read epd.len")
	} else if size <= 1 {
		return nil, errors.New("hello.too small epd.len")
	} else {
		size--
		if int(size) > r.Len() {
			return nil, errors.New("hello.too big epd.len")
		}
	}
	mode := uint8(0)
	if mode, err = r.Read8(); err != nil {
		return nil, errors.New("hello.read mode")
	}
	epd := make([]byte, int(size))
	if err = r.ReadBytes(epd); err != nil {
		return nil, errors.New("hello.read epd")
	}
	tag := make([]byte, 16)
	if err = r.ReadBytes(tag); err != nil {
		return nil, errors.New("hello.read tag")
	}
	return &helloRequest{epd, tag, mode}, nil
}

type assignRequest struct {
	yid       uint32
	pid       string
	cookie    []byte
	pubkey    []byte
	initiator []byte
}

func parseAssignRequest(r *xio.PacketReader) (*assignRequest, error) {
	var err error
	yid := uint32(0)
	if yid, err = r.Read32(); err != nil {
		return nil, errors.New("assign.read yid")
	}
	coolen := uint64(0)
	if coolen, err = r.Read7BitValue64(); err != nil {
		return nil, errors.New("assign.read cookie.len")
	} else if coolen != cookies.CookieSize {
		return nil, errors.New("assign.bad cookie.len")
	}
	coobuf := make([]byte, cookies.CookieSize)
	if err = r.ReadBytes(coobuf); err != nil {
		return nil, errors.New("assign.read cookie.value")
	}
	size := uint64(0)
	if size, err = r.Read7BitValue64(); err != nil {
		return nil, errors.New("assign.read pid.size")
	} else if size == 0 || size > uint64(r.Len()) {
		return nil, errors.New("assign.bad pid.size")
	}
	pid := sha256.Sum256(r.Bytes()[:size])
	if len(pid) != 0x20 {
		return nil, errors.New("assign.bad pid.len")
	}
	if size, err = r.Read7BitValue64(); err != nil {
		return nil, errors.New("assign.read pubkey.len")
	} else if size <= 2 {
		return nil, errors.New("assign.too small pubkey.len")
	} else {
		size -= 2
		if size > uint64(r.Len()) {
			return nil, errors.New("assign.too big pubkey.len")
		}
		if err := r.Skip(2); err != nil {
			return nil, errors.New("assign.skip useless")
		}
	}
	pubkey := make([]byte, int(size))
	if err = r.ReadBytes(pubkey); err != nil {
		return nil, errors.New("assign.read pubkey")
	}
	if size, err = r.Read7BitValue64(); err != nil {
		return nil, errors.New("assign.read initiator.len")
	} else if size == 0 || size > uint64(r.Len()) {
		return nil, errors.New("assign.bad initiator.len")
	}
	initiator := make([]byte, int(size))
	if err = r.ReadBytes(initiator); err != nil {
		return nil, errors.New("assign.read initiator")
	}
	return &assignRequest{yid, string(pid[:]), coobuf, pubkey, initiator}, nil
}
