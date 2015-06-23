package session

import (
	"container/list"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/xio"
	"github.com/spinlock/xserver/pkg/xserver/xlog"
)

type flowReader struct {
	session   *Session
	signature string
	fid       uint64
	frags     list.List
	stage     uint64
	ready     list.List
	handler   messageHandler
}

func newFlowReader(session *Session, signature string, fid uint64) *flowReader {
	fr := &flowReader{}
	fr.session = session
	fr.signature = signature
	fr.fid = fid
	fr.frags.Init()
	fr.stage = 0
	fr.ready.Init()
	return fr
}

func (fr *flowReader) CommitAck() {
	ack := newFlowAck(fr.stage)
	if e := fr.frags.Front(); e != nil {
		f := e.Value.(*fragment)
		beg, end := f.stage, f.stage
		for e = e.Next(); e != nil; e = e.Next() {
			f = e.Value.(*fragment)
			if f.stage == end+1 {
				end++
			} else {
				ack.AddRange(beg, end)
				beg, end = f.stage, f.stage
			}
		}
		ack.AddRange(beg, end)
	}
	cnt := uint64(0)
	if size := fr.frags.Len(); size == 0 {
		cnt = 0x7f
	} else if size < 0x3f00 {
		cnt = 0x3f00 - uint64(size)
	}
	fr.session.send(newFlowAckResponse(fr.fid, cnt, ack))
	xlog.OutLog.Printf("[flows]: xid = %d, reader.fid = %d, send: stage = %d, ack = %v\n", fr.session.xid, fr.fid, fr.stage, ack)
}

func (fr *flowReader) AddFragments(stageack uint64, frags ...*fragment) {
	xlog.OutLog.Printf("[flows]: xid = %d, reader.fid = %d, recv: stage = %d, stageack = %d, frags = %v\n", fr.session.xid, fr.fid, fr.stage, stageack, frags)
	if fr.handler.DeceptiveAck() {
		for _, f := range frags {
			if f.WithBefore() || f.WithAfter() {
				continue
			}
			if stageack+1 < f.stage {
				stageack = f.stage - 1
				xlog.OutLog.Printf("[flows]: xid = %d, reader.fid = %d, skip: set stageack = %d\n", fr.session.xid, fr.fid, stageack)
			}
		}
	}
	nothing := true
	if fr.stage < stageack {
		for {
			if e := fr.frags.Front(); e != nil {
				if f := e.Value.(*fragment); f.stage <= stageack {
					fr.frags.Remove(e)
					if fr.accept(f) {
						return
					}
					continue
				}
			}
			break
		}
		if fr.stage < stageack {
			fr.stage = stageack
			fr.deliver()
			xlog.ErrLog.Printf("[flows]: xid = %d, reader.fid = %d, skip to stage %d\n", fr.session.xid, fr.fid, fr.stage)
		}
		nothing = false
	}
	if len(frags) != 0 {
		lower, enext := fr.stage, fr.frags.Front()
		for _, f := range frags {
			if f.stage <= lower {
				xlog.OutLog.Printf("[flows]: xid = %d, reader.fid = %d, stage %d has already been received\n", fr.session.xid, fr.fid, f.stage)
				continue
			}
			for {
				if enext == nil {
					fr.frags.PushBack(f)
					nothing = false
				} else if fnext := enext.Value.(*fragment); f.stage < fnext.stage {
					fr.frags.InsertBefore(f, enext)
					nothing = false
				} else if fnext.stage == f.stage {
					enext = enext.Next()
					xlog.OutLog.Printf("[flows]: xid = %d, reader.fid = %d, stage %d has already been received\n", fr.session.xid, fr.fid, f.stage)
				} else {
					enext = enext.Next()
					continue
				}
				lower = f.stage
				break
			}
		}
	}
	if nothing {
		return
	}
	for {
		if e := fr.frags.Front(); e != nil {
			if f := e.Value.(*fragment); f.stage == fr.stage+1 {
				fr.frags.Remove(e)
				if fr.accept(f) {
					return
				}
				continue
			}
		}
		break
	}
	if sum := fr.frags.Len() + fr.ready.Len(); sum > 128 {
		xlog.ErrLog.Printf("[flows]: xid = %d, reader.fid = %d, too many stages = %d\n", fr.session.xid, fr.fid, sum)
	}
}

func (fr *flowReader) accept(f *fragment) bool {
	if next := fr.stage + 1; next > f.stage {
		xlog.ErrLog.Printf("[flows]: xid = %d, reader.fid = %d, accept invalid stage\n", fr.session.xid, fr.fid)
	} else {
		fr.stage = f.stage
		if next != f.stage {
			fr.deliver()
			xlog.ErrLog.Printf("[flows]: xid = %d, reader.fid = %d, skip stage in accept\n", fr.session.xid, fr.fid)
		}
		if f.Abandoned() {
			fr.deliver()
			xlog.OutLog.Printf("[flows]: xid = %d, reader.fid = %d, abandoned fragment\n", fr.session.xid, fr.fid)
		} else {
			if !f.WithBefore() {
				fr.deliver()
			}
			fr.ready.PushBack(f)
			if !f.WithAfter() {
				fr.deliver()
			}
		}
		if f.End() {
			fr.deliver()
			fr.handler.OnClose()
			return true
		}
	}
	return false
}

func (fr *flowReader) deliver() {
	if fr.ready.Len() != 0 {
		bs := fr.merge()
		fr.ready.Init()
		if len(bs) != 0 {
			if err := handleMessage(fr.handler, xio.NewPacketReader(bs)); err != nil {
				xlog.OutLog.Printf("[flows]: xid = %d, reader.fid = %d, deliver error = '%v'\n", fr.session.xid, fr.fid, err)
			}
		}
	}
}

func (fr *flowReader) merge() []byte {
	switch fr.ready.Len() {
	case 0:
		return nil
	case 1:
		f := fr.ready.Front().Value.(*fragment)
		if f.WithBefore() || f.WithAfter() {
			xlog.ErrLog.Printf("[flows]: xid = %d, reader.fid = %d, merge fragments failed\n", fr.session.xid, fr.fid)
			return nil
		}
		return f.data
	default:
		if fr.ready.Front().Value.(*fragment).WithBefore() || fr.ready.Back().Value.(*fragment).WithAfter() {
			xlog.ErrLog.Printf("[flows]: xid = %d, reader.fid = %d, merge fragments failed\n", fr.session.xid, fr.fid)
			return nil
		}
		size := 0
		for e := fr.ready.Front(); e != nil; e = e.Next() {
			f := e.Value.(*fragment)
			size += len(f.data)
		}
		if size == 0 {
			return nil
		}
		bs, p := make([]byte, size), 0
		for e := fr.ready.Front(); e != nil; e = e.Next() {
			f := e.Value.(*fragment)
			l := len(f.data)
			if l != 0 {
				copy(bs[p:], f.data)
				p += l
			}
		}
		return bs
	}
}
