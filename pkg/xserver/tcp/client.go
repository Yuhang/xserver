package tcp

import (
	"log"
	"net"
	"sync"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/counts"
)

type Client struct {
	ip   string
	port uint16
	send chan []byte
	recv chan []byte
}

func newClient(ip string, port uint16) *Client {
	c := &Client{}
	c.ip, c.port = ip, port
	c.send = make(chan []byte, 1024)
	c.recv = make(chan []byte, 1024)
	go c.main()
	return c
}

func (c *Client) Send(bs []byte) {
	c.send <- bs
}

func (c *Client) Recv() []byte {
	bs := <-c.recv
	return bs
}

func (c *Client) main() {
	for {
		conn, err := net.DialTCP("tcp4", nil, &net.TCPAddr{IP: net.ParseIP(c.ip), Port: int(c.port)})
		if err != nil {
			counts.Count("tcp.connect.error", 1)
			log.Printf("[tcp]: connect %s:%d failed '%v'\n", c.ip, c.port, err)
		} else {
			counts.Count("tcp.connect", 1)
			log.Printf("[tcp]: connect to %s\n", conn.RemoteAddr())
			conn.SetWriteBuffer(MaxSendBufferSize)
			conn.SetReadBuffer(MaxRecvBufferSize)
			conn.SetNoDelay(true)
			var once sync.Once
			sig := make(chan int)
			raise := func() {
				once.Do(func() {
					conn.Close()
					close(sig)
				})
			}
			go sender(conn, c.send, sig, raise)
			go recver(conn, c.recv, sig, raise)
			<-sig
			counts.Count("tcp.connect.close", 1)
		}
		for i := 0; i < 50; i++ {
			time.Sleep(time.Millisecond * 100)
			for {
				select {
				case <-c.send:
					continue
				default:
				}
				break
			}
		}
	}
}
