package session

import (
	"container/list"
	"encoding/hex"
	"errors"
	"net"
	"sync"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/args"
	"github.com/spinlock/xserver/pkg/xserver/counts"
	"github.com/spinlock/xserver/pkg/xserver/rtmfp"
	"github.com/spinlock/xserver/pkg/xserver/utils"
	"github.com/spinlock/xserver/pkg/xserver/xlog"
)

var sessions struct {
	buckets [256]struct {
		xidmap map[uint32]*Session
		pidmap map[string]*Session
		sync.RWMutex
	}
	lastxid uint32
	manages [32]struct {
		freshlist *list.List
		alivelist *list.List
		sync.Mutex
	}
	sync.Mutex
}

func init() {
	sessions.lastxid = 0
	for i := 0; i < len(sessions.buckets); i++ {
		sessions.buckets[i].xidmap = make(map[uint32]*Session, 8192)
		sessions.buckets[i].pidmap = make(map[string]*Session, 8192)
	}
	for i := 0; i < len(sessions.manages); i++ {
		m := &sessions.manages[i]
		m.freshlist = list.New()
		m.alivelist = list.New()
		go func() {
			manage := args.Manage()
			for {
				m.Lock()
				if m.freshlist.Len() != 0 {
					m.alivelist.PushBackList(m.freshlist)
					m.freshlist.Init()
				}
				m.Unlock()
				count := 0
				if e := m.alivelist.Front(); e != nil {
					for e != nil {
						next := e.Next()
						if s := e.Value.(*Session); s.Manage() {
							delSessionByXid(s.xid)
							delSessionByPid(s.pid)
							m.alivelist.Remove(e)
							count++
							xlog.SssLog.Printf("[exit] %s [%s] xid = %d cnt = %d\n", xlog.StringToHex(s.pid), s.raddr, s.xid, s.manage.cnt)
						}
						e = next
					}
				}
				if count != 0 {
					counts.Count("session.cleanup", count)
				}
				time.Sleep(time.Millisecond * time.Duration(manage))
			}
		}()
	}
}

func Create(yid uint32, pid string, cookie string, encrypt, decrypt []byte, lport uint16, raddr *net.UDPAddr) (uint32, error) {
	s := &Session{}
	s.xid = 0
	s.yid = yid
	s.pid = pid
	s.lport, s.raddr = lport, raddr
	s.addrs = nil
	s.cookie = cookie
	s.closed = false
	s.manage.cnt, s.manage.lasttime = 0, time.Now().UnixNano()
	s.stmptime = 0
	s.AESEngine = rtmfp.NewAESEngine()
	if err := s.SetKey(encrypt, decrypt); err != nil {
		return 0, err
	}
	s.lastfid = 0
	s.lastsid = 0
	s.mainfw = nil
	s.readers = make(map[uint64]*flowReader)
	s.writers = make(map[uint64]*flowWriter)
	s.rsplist.Init()

	sessions.Lock()
	defer sessions.Unlock()

	xid := sessions.lastxid
	for {
		xid++
		if xid == 0 {
			continue
		}
		if xid == sessions.lastxid {
			return 0, errors.New("too many sessions")
		}
		if getSessionByXid(xid) == nil {
			break
		}
	}
	s.xid = xid
	sessions.lastxid = xid
	addSessionByXid(xid, s)
	addSessionByPid(pid, s)

	m := &sessions.manages[int(xid%uint32(len(sessions.manages)))]
	m.Lock()
	m.freshlist.PushBack(s)
	m.Unlock()

	counts.Count("session.new", 1)
	return xid, nil
}

func hashXidToBid(xid uint32) uint16 {
	return uint16(xid) % uint16(len(sessions.buckets))
}

func hashPidToBid(pid string) uint16 {
	return utils.Hash16S(pid) % uint16(len(sessions.buckets))
}

func FindByXid(xid uint32) *Session {
	return getSessionByXid(xid)
}

func FindByPid(pid string) *Session {
	return getSessionByPid(pid)
}

func addSessionByXid(xid uint32, s *Session) {
	b := &sessions.buckets[hashXidToBid(xid)]
	b.Lock()
	b.xidmap[xid] = s
	b.Unlock()
}

func addSessionByPid(pid string, s *Session) {
	b := &sessions.buckets[hashPidToBid(pid)]
	b.Lock()
	b.pidmap[pid] = s
	b.Unlock()
}

func delSessionByXid(xid uint32) {
	b := &sessions.buckets[hashXidToBid(xid)]
	b.Lock()
	delete(b.xidmap, xid)
	b.Unlock()
}

func delSessionByPid(pid string) {
	b := &sessions.buckets[hashPidToBid(pid)]
	b.Lock()
	delete(b.pidmap, pid)
	b.Unlock()
}

func getSessionByXid(xid uint32) *Session {
	b := &sessions.buckets[hashXidToBid(xid)]
	b.RLock()
	s := b.xidmap[xid]
	b.RUnlock()
	return s
}

func getSessionByPid(pid string) *Session {
	b := &sessions.buckets[hashPidToBid(pid)]
	b.RLock()
	s := b.pidmap[pid]
	b.RUnlock()
	return s
}

func Summary() map[string]interface{} {
	xids, pids := 0, 0
	zclosed, zmanage := 0, make([]int, maxKeepalive+1)
	for i := 0; i < len(sessions.buckets); i++ {
		b := &sessions.buckets[i]
		b.RLock()
		xids += len(b.xidmap)
		pids += len(b.pidmap)
		for _, s := range b.xidmap {
			if s.closed {
				zclosed++
			} else if k := s.manage.cnt; k >= 0 && k < len(zmanage) {
				zmanage[k]++
			}
		}
		b.RUnlock()
	}
	return map[string]interface{}{
		"xids": xids,
		"pids": pids,
		"z": map[string]interface{}{
			"closed": zclosed,
			"manage": zmanage,
		},
	}
}

func MapSize() map[string]interface{} {
	const n = len(sessions.buckets)
	xids := make([]int, n)
	pids := make([]int, n)
	for i := 0; i < n; i++ {
		b := &sessions.buckets[i]
		b.RLock()
		xids[i] = len(b.xidmap)
		pids[i] = len(b.pidmap)
		b.RUnlock()
	}
	return map[string]interface{}{
		"xids": xids,
		"pids": pids,
	}
}

func DumpAll() []map[string]interface{} {
	all := make([]map[string]interface{}, 0, 8192)
	for i := 0; i < len(sessions.buckets); i++ {
		b := &sessions.buckets[i]
		b.RLock()
		for _, s := range b.xidmap {
			s.Lock()
			all = append(all, map[string]interface{}{
				"xid":    s.xid,
				"yid":    s.yid,
				"pid":    hex.EncodeToString([]byte(s.pid)),
				"raddr":  s.raddr.String(),
				"addrs":  s.addrs,
				"closed": s.closed,
				"manage": map[string]interface{}{
					"cnt":      s.manage.cnt,
					"lasttime": s.manage.lasttime,
				},
			})
			s.Unlock()
		}
		b.RUnlock()
	}
	return all
}
