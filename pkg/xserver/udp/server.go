package udp

import (
	"log"
	"net"
	"sync"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/counts"
)

const (
	MaxSendBufferSize = 1024 * 1024 * 8
	MaxRecvBufferSize = 1024 * 1024 * 32
)

const (
	MaxPacketSize = 1024 * 2
)

type datagram struct {
	addr *net.UDPAddr
	data []byte
}

type Server struct {
	port uint16
	send chan *datagram
	recv chan *datagram
}

func newServer(port uint16) *Server {
	s := &Server{}
	s.port = port
	s.send = make(chan *datagram, 2048)
	s.recv = make(chan *datagram, 2048)
	go s.main()
	return s
}

func (s *Server) Recv() (uint16, *net.UDPAddr, []byte) {
	dg := <-s.recv
	return s.port, dg.addr, dg.data
}

func (s *Server) Send(addr *net.UDPAddr, data []byte) {
	s.send <- &datagram{addr, data}
}

func (s *Server) main() {
	for {
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: int(s.port)})
		if err != nil {
			counts.Count("udp.listen.error", 1)
			log.Printf("[udp]: listen port %d failed '%v'\n", s.port, err)
		} else {
			counts.Count("udp.listen", 1)
			log.Printf("[udp]: listen port %d\n", s.port)
			conn.SetWriteBuffer(MaxSendBufferSize)
			conn.SetReadBuffer(MaxRecvBufferSize)
			var once sync.Once
			sig := make(chan int)
			raise := func() {
				once.Do(func() {
					conn.Close()
					close(sig)
				})
			}
			go sender(conn, s.send, sig, raise)
			go recver(conn, s.recv, sig, raise)
			<-sig
			counts.Count("udp.listen.close", 1)
		}
		for i := 0; i < 50; i++ {
			time.Sleep(time.Millisecond * 100)
			for {
				select {
				case <-s.send:
					continue
				default:
				}
				break
			}
		}
	}
}

func sender(conn *net.UDPConn, send <-chan *datagram, sig <-chan int, raise func()) {
	defer raise()
	for {
		select {
		case <-sig:
			return
		case dg := <-send:
			addr, data := dg.addr, dg.data
			if addr == nil || len(data) == 0 {
				continue
			}
			if _, err := conn.WriteToUDP(data, addr); err != nil {
				log.Printf("[udp]: send error = '%v'\n", err)
				return
			}
		}
	}
}

func recver(conn *net.UDPConn, recv chan<- *datagram, sig <-chan int, raise func()) {
	defer raise()
	for {
		select {
		case <-sig:
			return
		default:
			bs := make([]byte, MaxPacketSize)
			if n, addr, err := conn.ReadFromUDP(bs); err != nil {
				log.Printf("[udp]: recv error = '%v'\n", err)
				return
			} else if addr != nil && n > 0 {
				data := bs[:n]
				select {
				case <-sig:
					return
				case recv <- &datagram{addr, data}:
				}
			}
		}
	}
}
