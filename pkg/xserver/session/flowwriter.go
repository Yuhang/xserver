package session

import (
	"container/list"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/args"
	"github.com/spinlock/xserver/pkg/xserver/xlog"
)

var (
	retrans = args.Retrans()
)

type flowWriter struct {
	session   *Session
	signature string
	fid       uint64
	closed    bool
	manage    struct {
		idx      int
		lasttime int64
	}
	frags  list.List
	stage  uint64
	reader *flowReader
}

func newFlowWriter(session *Session, signature string, fid uint64) *flowWriter {
	fw := &flowWriter{}
	fw.session = session
	fw.signature = signature
	fw.fid = fid
	fw.closed = false
	fw.manage.idx, fw.manage.lasttime = 0, 0
	fw.frags.Init()
	fw.stage = 0
	return fw
}

func (fw *flowWriter) End() {
	fw.frags.Init()
	fw.stage++
	flags := uint8(flagsAbandoned | flagsEnd)
	f := &fragment{fw.stage, flags, nil, time.Now().UnixNano()}
	fw.session.send(newFlowResponse(fw, f, f.stage-1))
	xlog.OutLog.Printf("[flows]: xid = %d, writer.fid = %d, send: stage = %d, stageack = %d, frag = %v\n", fw.session.xid, fw.fid, fw.stage, f.stage-1, f)
}

func (fw *flowWriter) CommitAck(cnt uint64, ack *flowAck) {
	xlog.OutLog.Printf("[flows]: xid = %d, writer.fid = %d, recv: stage = %d, ack = %v\n", fw.session.xid, fw.fid, fw.stage, ack)
	for {
		if e := fw.frags.Front(); e != nil {
			if f := e.Value.(*fragment); f.stage <= ack.stage {
				fw.frags.Remove(e)
				continue
			}
		}
		break
	}
	now := time.Now().UnixNano()
	fw.manage.idx, fw.manage.lasttime = 0, now
	if e := fw.frags.Front(); e != nil {
		lastsend := now - int64(time.Millisecond)*100
		stageack := e.Value.(*fragment).stage - 1
		for econt := ack.conts.Front(); econt != nil && e != nil; econt = econt.Next() {
			r := econt.Value.(*flowAckRange)
			for e != nil {
				f := e.Value.(*fragment)
				if f.stage < r.beg {
					if f.sendtime < lastsend {
						f.sendtime = now
						fw.session.send(newFlowResponse(fw, f, stageack))
						xlog.OutLog.Printf("[flows]: xid = %d, writer.fid = %d, send: stage = %d, stageack = %d, frag = %v\n", fw.session.xid, fw.fid, fw.stage, stageack, f)
					}
					e = e.Next()
				} else if f.stage <= r.end {
					enext := e.Next()
					fw.frags.Remove(e)
					e = enext
				} else {
					break
				}
			}
		}
		for e != nil {
			f := e.Value.(*fragment)
			if f.sendtime < lastsend {
				f.sendtime = now
				fw.session.send(newFlowResponse(fw, f, stageack))
				xlog.OutLog.Printf("[flows]: xid = %d, writer.fid = %d, send: stage = %d, stageack = %d, frag = %v\n", fw.session.xid, fw.fid, fw.stage, stageack, f)
			}
			e = e.Next()
		}
	}
}

func (fw *flowWriter) AddFragments(reliable bool, frags ...[]byte) {
	if len(frags) == 0 {
		return
	}
	stageack := fw.stage
	if e := fw.frags.Front(); e != nil {
		stageack = e.Value.(*fragment).stage - 1
	}
	cnt := len(frags)
	now := time.Now().UnixNano()
	for i := 0; i < cnt; i++ {
		flags := uint8(0)
		if i != 0 {
			flags |= flagsWithBefore
		}
		if i != cnt-1 {
			flags |= flagsWithAfter
		}
		fw.stage++
		f := &fragment{fw.stage, flags, frags[i], now}
		if reliable {
			fw.frags.PushBack(f)
		}
		fw.session.send(newFlowResponse(fw, f, stageack))
		xlog.OutLog.Printf("[flows]: xid = %d, writer.fid = %d, send: stage = %d, stageack = %d, frag = %v\n", fw.session.xid, fw.fid, fw.stage, stageack, f)
	}
}

func (fw *flowWriter) Manage() bool {
	if fw.frags.Len() == 0 {
		return true
	}
	now := time.Now().UnixNano()
	if fw.manage.lasttime < now-int64(time.Millisecond)*int64(retrans[fw.manage.idx]) {
		if max := len(retrans) - 1; fw.manage.idx < max {
			fw.manage.idx++
		}
		fw.manage.lasttime = now
		lastsend := now - int64(time.Millisecond)*100
		if f := fw.frags.Back().Value.(*fragment); f.sendtime < lastsend {
			stageack := fw.frags.Front().Value.(*fragment).stage - 1
			f.sendtime = now
			fw.session.send(newFlowResponse(fw, f, stageack))
			xlog.OutLog.Printf("[flows]: xid = %d, writer.fid = %d, send: stage = %d, stageack = %d, frag = %v\n", fw.session.xid, fw.fid, fw.stage, stageack, f)
		}
	}
	return false
}

func split(data []byte) [][]byte {
	const step = 256
	if cnt := (step - 1 + len(data)) / step; cnt == 0 {
		return nil
	} else {
		ret := make([][]byte, cnt)
		for i, beg := 0, 0; i < cnt; i++ {
			end := beg + step
			if end > len(data) {
				end = len(data)
			}
			ret[i] = data[beg:end]
			beg = end
		}
		return ret
	}
}
