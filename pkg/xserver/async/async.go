package async

import (
	"container/list"
	"sync"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/counts"
	"github.com/spinlock/xserver/pkg/xserver/utils"
	"github.com/spinlock/xserver/pkg/xserver/xlog"
)

var groups [32]struct {
	lock sync.Mutex
	cond *sync.Cond
	list *list.List
}

func init() {
	for i := 0; i < len(groups); i++ {
		g := &groups[i]
		g.cond = sync.NewCond(&g.lock)
		g.list = list.New()
		main := func() {
			defer func() {
				if x := recover(); x != nil {
					counts.Count("async.panic", 1)
					xlog.ErrLog.Printf("[async]: panic = %v\n%s\n", x, utils.Trace())
				}
			}()
			for {
				var f func()
				g.lock.Lock()
				if e := g.list.Front(); e != nil {
					f = g.list.Remove(e).(func())
				} else {
					g.cond.Wait()
				}
				g.lock.Unlock()
				if f != nil {
					f()
				}
			}
		}
		go func() {
			for {
				main()
			}
		}()
	}
}

func Call(gid uint64, f func()) {
	if f == nil {
		return
	}
	g := &groups[gid%uint64(len(groups))]
	g.lock.Lock()
	g.list.PushBack(f)
	g.cond.Signal()
	g.lock.Unlock()
}
