package session

import (
	"bytes"
	"container/list"
	"fmt"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

type flowAck struct {
	stage uint64
	conts list.List
}

type flowAckRange struct {
	beg, end uint64
}

func newFlowAck(stage uint64) *flowAck {
	ack := &flowAck{}
	ack.stage = stage
	ack.conts.Init()
	return ack
}

func (ack *flowAck) AddRange(beg, end uint64) {
	ack.conts.PushBack(&flowAckRange{beg, end})
}

func parseFlowAck(r *xio.PacketReader) (*flowAck, error) {
	if stage, err := r.Read7BitValue64(); err != nil {
		return nil, err
	} else {
		ack := newFlowAck(stage)
		var beg, end uint64
		for r.Len() != 0 {
			if beg, err = r.Read7BitValue64(); err != nil {
				return nil, err
			}
			if end, err = r.Read7BitValue64(); err != nil {
				return nil, err
			}
			beg = beg + stage + 2
			end = end + beg
			ack.AddRange(beg, end)
			stage = end
		}
		return ack, nil
	}
}

func storeFlowAck(ack *flowAck, w *xio.PacketWriter) error {
	if err := w.Write7BitValue64(ack.stage); err != nil {
		return err
	} else {
		stage := ack.stage
		for e := ack.conts.Front(); e != nil; e = e.Next() {
			r := e.Value.(*flowAckRange)
			if err := w.Write7BitValue64(r.beg - stage - 2); err != nil {
				return err
			}
			if err := w.Write7BitValue64(r.end - r.beg); err != nil {
				return err
			}
			stage = r.end
		}
		return nil
	}
}

func (ack *flowAck) Size() (int, error) {
	if add, err := xio.SizeOf7BitValue64(ack.stage); err != nil {
		return 0, err
	} else {
		total := add
		stage := ack.stage
		for e := ack.conts.Front(); e != nil; e = e.Next() {
			r := e.Value.(*flowAckRange)
			if add, err := xio.SizeOf7BitValue64(r.beg - stage - 2); err != nil {
				return 0, err
			} else {
				total += add
			}
			if add, err := xio.SizeOf7BitValue64(r.end - r.beg); err != nil {
				return 0, err
			} else {
				total += add
			}
			stage = r.end
		}
		return total, nil
	}
}

func (ack *flowAck) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "{")
	fmt.Fprintf(&buf, "[%d,%d]", 0, ack.stage)
	for e := ack.conts.Front(); e != nil; e = e.Next() {
		r := e.Value.(*flowAckRange)
		fmt.Fprintf(&buf, "[%d,%d]", r.beg, r.end)
	}
	fmt.Fprintf(&buf, "}")
	return buf.String()
}
