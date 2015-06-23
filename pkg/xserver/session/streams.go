package session

import (
	"container/list"
	"sync"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/utils"
)

var streams struct {
	buckets [256]struct {
		pubmap map[string]*publication
		sync.Mutex
	}
}

type publication struct {
	name     string
	gid      uint64
	rpc      bool
	closed   bool
	master   *streamHandler
	slaves   *list.List
	reliable bool
	bid      uint16
	sync.Mutex
}

func init() {
	for i := 0; i < len(streams.buckets); i++ {
		streams.buckets[i].pubmap = make(map[string]*publication, 8192)
	}
}

func newPublication(name string) *publication {
	if name == "recvPull" || name == "recvPull2" {
		p := &publication{}
		p.name, p.gid = name, uint64(time.Now().UnixNano())
		p.rpc = true
		p.closed = false
		p.master, p.slaves = nil, nil
		if name != "recvPull2" {
			p.reliable = true
		} else {
			p.reliable = false
		}
		return p
	}

	bid := utils.Hash16S(name) % uint16(len(streams.buckets))

	b := &streams.buckets[bid]
	b.Lock()
	p := b.pubmap[name]
	if p == nil {
		p = &publication{}
		p.name, p.gid = name, uint64(time.Now().UnixNano())
		p.rpc = false
		p.closed = false
		p.master, p.slaves = nil, list.New()
		p.bid = bid
		p.reliable = true
		b.pubmap[name] = p
	}
	b.Unlock()
	return p
}

func Streams() int {
	count := 0
	for i := 0; i < len(streams.buckets); i++ {
		b := &streams.buckets[i]
		b.Lock()
		count += len(b.pubmap)
		b.Unlock()
	}
	return count
}

func DumpStreams() map[string]interface{} {
	all := make(map[string]interface{}, 8192)
	for i := 0; i < len(streams.buckets); i++ {
		b := &streams.buckets[i]
		b.Lock()
		for name, p := range b.pubmap {
			x := make(map[string]interface{})
			if p != nil {
				if m := p.master; m != nil {
					x["master"] = m.session.xid
				} else {
					x["master"] = 0
				}
				if l, _ := p.list(); l != nil {
					s := make([]uint32, 0, l.Len())
					for e := l.Front(); e != nil; e = e.Next() {
						s = append(s, e.Value.(*streamHandler).session.xid)
					}
					x["slaves"] = s
				}
				x["closed"] = p.closed
			}
			all[name] = x
		}
		b.Unlock()
	}
	return all
}

func (p *publication) start(master *streamHandler) bool {
	p.Lock()
	ok := false
	if !p.closed {
		if p.master == nil {
			p.master, ok = master, true
		}
	}
	p.Unlock()
	return ok
}

func (p *publication) stop() {
	p.Lock()
	ok := false
	if !p.closed {
		p.closed = true
		if !p.rpc {
			ok = true
		}
	}
	p.Unlock()
	if ok {
		b := &streams.buckets[p.bid]
		b.Lock()
		delete(b.pubmap, p.name)
		b.Unlock()
	}
}

func (p *publication) list() (*list.List, bool) {
	p.Lock()
	l, ok := p.slaves, !p.closed
	p.Unlock()
	return l, ok
}

func (p *publication) add(h *streamHandler) bool {
	p.Lock()
	ok := false
	if !p.closed {
		ok = true
		if !p.rpc {
			l := list.New()
			l.PushBack(h)
			for e := p.slaves.Front(); e != nil; e = e.Next() {
				if o := e.Value.(*streamHandler); o != h {
					l.PushBack(o)
				}
			}
			p.slaves = l
		}
	}
	p.Unlock()
	return ok
}

func (p *publication) remove(h *streamHandler) {
	p.Lock()
	ok := false
	if !p.closed {
		if !p.rpc {
			l := list.New()
			for e := p.slaves.Front(); e != nil; e = e.Next() {
				if o := e.Value.(*streamHandler); o != h {
					l.PushBack(o)
				}
			}
			p.slaves = l
			if p.master == nil && p.slaves.Len() == 0 {
				p.closed, ok = true, true
			}
		}
	}
	p.Unlock()
	if ok {
		b := &streams.buckets[p.bid]
		b.Lock()
		delete(b.pubmap, p.name)
		b.Unlock()
	}
}
