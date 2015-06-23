package cookies

import (
	"sync"
)

type Cookie struct {
	Xid       uint32
	Pid       string
	Responder []byte
	value     string
	alloctime int64
	sync.Mutex
}

func (c *Cookie) Value() string {
	return c.value
}
