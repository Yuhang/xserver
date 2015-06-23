package tcp

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/counts"
	"github.com/spinlock/xserver/pkg/xserver/utils"
	"github.com/spinlock/xserver/pkg/xserver/xlog"
)

const (
	MaxSendBufferSize = 1024 * 1024 * 8
	MaxRecvBufferSize = 1024 * 1024 * 32
)

const (
	MaxPacketSize = 1024 * 1024 * 10
)

type Server struct {
	port uint16
	recv chan []byte
}

func newServer(port uint16) *Server {
	s := &Server{}
	s.port = port
	s.recv = make(chan []byte, 1024)
	go s.main()
	return s
}

func (s *Server) Recv() []byte {
	bs := <-s.recv
	return bs
}

func (s *Server) main() {
	for {
		ln, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4zero, Port: int(s.port)})
		if err != nil {
			counts.Count("tcp.listen.error", 1)
			log.Printf("[tcp]: listen port %d failed '%v'\n", s.port, err)
		} else {
			counts.Count("tcp.listen", 1)
			for {
				if conn, err := ln.AcceptTCP(); err != nil {
					counts.Count("tcp.accept.error", 1)
					log.Printf("[tcp]: accept port %d failed '%v'\n", s.port, err)
					break
				} else {
					counts.Count("tcp.accept", 1)
					log.Printf("[tcp]: accept port %d from [%s]\n", s.port, conn.RemoteAddr())
					conn.SetWriteBuffer(MaxSendBufferSize)
					conn.SetReadBuffer(MaxRecvBufferSize)
					conn.SetNoDelay(true)
					var once sync.Once
					sig := make(chan int)
					raise := func() {
						once.Do(func() {
							conn.Close()
							close(sig)
							counts.Count("tcp.accept.close", 1)
						})
					}
					go recver(conn, s.recv, sig, raise)
				}
			}
			ln.Close()
			counts.Count("tcp.listen.close", 1)
		}
		time.Sleep(time.Second * 5)
	}
}

func sender(conn *net.TCPConn, send <-chan []byte, sig <-chan int, raise func()) {
	defer raise()
	for {
		select {
		case <-sig:
			return
		case data := <-send:
			if len(data) == 0 {
				continue
			}
			if err := writeData(conn, data); err != nil {
				log.Printf("[tcp]: send error = '%v'\n", err)
				return
			}
			xlog.TcpLog.Printf("tcp.send:\n%s", utils.Formatted(data))
		}
	}
}

func recver(conn *net.TCPConn, recv chan<- []byte, sig <-chan int, raise func()) {
	defer raise()
	for {
		select {
		case <-sig:
			return
		default:
			if data, err := readData(conn); err != nil {
				log.Printf("[tcp]: recv error = '%v'\n", err)
				return
			} else if len(data) != 0 {
				select {
				case <-sig:
					return
				case recv <- data:
				}
				xlog.TcpLog.Printf("tcp.recv:\n%s", utils.Formatted(data))
			}
		}
	}
}

func readData(conn *net.TCPConn) ([]byte, error) {
	head := make([]byte, 8)
	for {
		if n, err := readBytes(conn, head); err != nil {
			if e, ok := err.(*net.OpError); ok && e.Timeout() {
				if n == 0 {
					continue
				}
			}
			return nil, err
		}
		break
	}
	magic := uint64(0)
	for i := 0; i < 8; i++ {
		magic = (magic << 8) + uint64(uint8(head[i]))
	}
	if uint32(magic>>32) != 0xdeadbeaf {
		return nil, errors.New(fmt.Sprintf("magic = %016x", magic))
	}
	if size := int(uint32(magic)); size > MaxPacketSize {
		return nil, errors.New(fmt.Sprintf("size = %d, max = %d", size, MaxPacketSize))
	} else {
		data := make([]byte, size)
		if _, err := readBytes(conn, data); err != nil {
			return nil, err
		}
		return data, nil
	}
}

func readBytes(conn *net.TCPConn, buf []byte) (int, error) {
	off := 0
	for off != len(buf) {
		if err := conn.SetReadDeadline(time.Now().Add(time.Second * 90)); err != nil {
			return 0, err
		}
		if n, err := conn.Read(buf[off:]); err != nil {
			return off, err
		} else {
			off += n
		}
	}
	return off, nil
}

func writeData(conn *net.TCPConn, data []byte) error {
	buff := make([]byte, 8+len(data))
	magic := (uint64(0xdeadbeaf) << 32) + uint64(uint32(len(data)))
	for i, shift := 0, uint(56); i < 8; i, shift = i+1, shift-8 {
		buff[i] = uint8(magic >> shift)
	}
	copy(buff[8:], data)
	return writeBytes(conn, buff)
}

func writeBytes(conn *net.TCPConn, buf []byte) error {
	off := 0
	for off != len(buf) {
		if err := conn.SetWriteDeadline(time.Now().Add(time.Second * 10)); err != nil {
			return err
		}
		if n, err := conn.Write(buf[off:]); err != nil {
			return err
		} else {
			off += n
		}
	}
	return nil
}
