package handshake

import (
	"container/list"
	"sync"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/counts"
)

var handshakes struct {
	list.List
	sync.Mutex
}

func init() {
	handshakes.Init()
	go func() {
		for {
			handshakes.Lock()
			if e := handshakes.Back(); e != nil {
				handshakes.Remove(e)
				counts.Count("handshake.release", 1)
			}
			n := handshakes.Len()
			handshakes.Unlock()
			if n > 512 {
				time.Sleep(time.Second * 2)
			} else if n > 128 {
				time.Sleep(time.Second * 5)
			} else {
				time.Sleep(time.Second * 30)
			}
		}
	}()
}

func getHandshake() *Handshake {
	handshakes.Lock()
	defer handshakes.Unlock()
	if e := handshakes.Front(); e != nil {
		return handshakes.Remove(e).(*Handshake)
	} else {
		return newHandshake()
	}
}

func putHandshake(h *Handshake) {
	if h == nil {
		return
	}
	handshakes.Lock()
	handshakes.PushFront(h)
	handshakes.Unlock()
}
