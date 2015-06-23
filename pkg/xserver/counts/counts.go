package counts

import (
	"container/list"
	"sync"
	"time"
)

var counts struct {
	buckets [128]struct {
		vmap map[string]int64
		sync.Mutex
	}
	snapshot map[string]int64
}

func init() {
	for i := 0; i < len(counts.buckets); i++ {
		counts.buckets[i].vmap = make(map[string]int64)
	}
	counts.snapshot = make(map[string]int64)
	go func() {
		for {
			time.Sleep(time.Minute)
			list := list.New()
			for i := 0; i < len(counts.buckets); i++ {
				b := &counts.buckets[i]
				b.Lock()
				if len(b.vmap) != 0 {
					list.PushBack(b.vmap)
					b.vmap = make(map[string]int64)
				}
				b.Unlock()
			}
			sum := make(map[string]int64)
			for e := list.Front(); e != nil; e = e.Next() {
				for k, v := range e.Value.(map[string]int64) {
					sum[k] += v
				}
			}
			counts.snapshot = sum
		}
	}()
}

func Count(key string, cnt int) {
	idx := uint16(time.Now().UnixNano()) % uint16(len(counts.buckets))
	b := &counts.buckets[idx]
	b.Lock()
	b.vmap[key] += int64(cnt)
	b.Unlock()
}

func Snapshot() map[string]int64 {
	return counts.snapshot
}
