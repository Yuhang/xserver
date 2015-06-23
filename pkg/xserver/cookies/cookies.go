package cookies

import (
	"sync"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/counts"
	"github.com/spinlock/xserver/pkg/xserver/xio"
)

const (
	CookieSize = 0x40
)

var cookies struct {
	values map[string]*Cookie
	sync.Mutex
}

func init() {
	cookies.values = make(map[string]*Cookie, 16384)
	go func() {
		for {
			limit := time.Now().UnixNano() - int64(time.Minute)*5
			count := 0
			cookies.Lock()
			for value, cookie := range cookies.values {
				if cookie.alloctime < limit {
					delete(cookies.values, value)
					count++
				}
			}
			cookies.Unlock()
			if count != 0 {
				counts.Count("cookie.timeout", count)
			}
			time.Sleep(time.Second * 15)
		}
	}()
}

func Count() int {
	cookies.Lock()
	n := len(cookies.values)
	cookies.Unlock()
	return n
}

func New() *Cookie {
	cookies.Lock()
	defer cookies.Unlock()
	now := time.Now().UnixNano()
	buf := make([]byte, CookieSize)
	for i, v := 0, now; i < 8; i, v = i + 1, v >> 8 {
		buf[i] = uint8(v)
	}
	rnd := xio.NewRandomReader(now)
	for i := 0; i < 4; i++ {
		rnd.ReadBytes(buf[8:])
		value := string(buf)
		if c := cookies.values[value]; c != nil {
			continue
		}
		c := &Cookie{}
		c.value = value
		c.alloctime = now
		cookies.values[value] = c
		counts.Count("cookie.new", 1)
		return c
	}
	counts.Count("cookie.null", 1)
	return nil
}

func Find(value string) *Cookie {
	cookies.Lock()
	c := cookies.values[value]
	cookies.Unlock()
	if c == nil {
		counts.Count("cookie.notfound", 1)
	}
	return c
}

func Commit(value string) {
	cookies.Lock()
	delete(cookies.values, value)
	cookies.Unlock()
	counts.Count("cookie.commit", 1)
}
