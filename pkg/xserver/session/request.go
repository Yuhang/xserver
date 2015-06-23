package session

import (
	"errors"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

type flowRequest struct {
	fid       uint64
	signature string
	stage     uint64
	stageack  uint64
	slices    []*flowRequestSlice
}

type flowRequestSlice struct {
	flags uint8
	data  []byte
}

type flowAckRequest struct {
	fid uint64
	cnt uint64
	ack *flowAck
}

type flowErrorRequest struct {
	fid uint64
}

func (req *flowRequest) Fragments() []*fragment {
	frags := make([]*fragment, len(req.slices))
	for i := 0; i < len(req.slices); i++ {
		stage := req.stage + uint64(i)
		flags, data := req.slices[i].flags, req.slices[i].data
		frags[i] = &fragment{stage, flags, data, 0}
	}
	return frags
}

func (req *flowRequest) AddSlice(slice *flowRequestSlice) {
	req.slices = append(req.slices, slice)
}

func parseFlowRequest(r *xio.PacketReader) (*flowRequest, error) {
	var err error
	flags := uint8(0)
	if flags, err = r.Read8(); err != nil {
		return nil, errors.New("flow.read flags")
	}
	fid := uint64(0)
	if fid, err = r.Read7BitValue64(); err != nil {
		return nil, errors.New("flow.read fid")
	}
	stage, delta := uint64(0), uint64(0)
	if stage, err = r.Read7BitValue64(); err != nil {
		return nil, errors.New("flow.read stage")
	}
	if delta, err = r.Read7BitValue64(); err != nil {
		return nil, errors.New("flow.read delta")
	}
	signature := ""
	if (flags & flagsHeader) != 0 {
		if signature, err = r.ReadString8(); err != nil {
			return nil, errors.New("flow.read signature")
		}
		for {
			if size, err := r.Read8(); err != nil {
				return nil, errors.New("flow.read header content size")
			} else if size == 0 {
				break
			} else if n := int(size); n > r.Len() {
				return nil, errors.New("flows.too big header content size")
			} else if err = r.Skip(n); err != nil {
				return nil, errors.New("flows.skip header")
			}
		}
	}
	data := r.Bytes()
	req := &flowRequest{}
	req.fid = fid
	req.signature = signature
	req.stage = stage
	req.stageack = stage - delta
	req.slices = make([]*flowRequestSlice, 0, 4)
	req.AddSlice(&flowRequestSlice{flags, data})
	return req, nil
}

func parseFlowRequestSlice(r *xio.PacketReader) (*flowRequestSlice, error) {
	var err error
	flags := uint8(0)
	if flags, err = r.Read8(); err != nil {
		return nil, errors.New("flow.read flags")
	}
	data := r.Bytes()
	return &flowRequestSlice{flags, data}, nil
}

func parseFlowAckRequest(r *xio.PacketReader) (*flowAckRequest, error) {
	var err error
	fid := uint64(0)
	if fid, err = r.Read7BitValue64(); err != nil {
		return nil, errors.New("flowack.read fid")
	}
	cnt := uint64(0)
	if cnt, err = r.Read7BitValue64(); err != nil {
		return nil, errors.New("flowack.read cnt")
	}
	var ack *flowAck
	if ack, err = parseFlowAck(r); err != nil {
		return nil, errors.New("flowack.read ack")
	}
	return &flowAckRequest{fid, cnt, ack}, nil
}

func parseFlowErrorRequest(r *xio.PacketReader) (*flowErrorRequest, error) {
	var err error
	fid := uint64(0)
	if fid, err = r.Read7BitValue64(); err != nil {
		return nil, errors.New("flowerror.read fid")
	}
	return &flowErrorRequest{fid}, nil
}
