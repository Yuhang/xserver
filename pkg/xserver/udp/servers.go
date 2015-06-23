package udp

import (
	"net"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/args"
	"github.com/spinlock/xserver/pkg/xserver/counts"
	"github.com/spinlock/xserver/pkg/xserver/xlog"
)

var (
	servers [65536]*Server
)

func init() {
	for _, port := range args.UdpListenPorts() {
		if servers[port] != nil {
			continue
		}
		servers[port] = newServer(port)
	}
}

func GetServers() []*Server {
	srvs := []*Server{}
	for _, s := range servers {
		if s != nil {
			srvs = append(srvs, s)
		}
	}
	return srvs
}

func Send(lport uint16, raddr *net.UDPAddr, data []byte) {
	if len(data) > 1400 {
		counts.Count("udp.toobig", 1)
		xlog.ErrLog.Printf("[udp]: udp-%d packet is too big, size = %d\n", lport, len(data))
	} else if srv := servers[lport]; srv == nil {
		counts.Count("udp.notfound", 1)
		xlog.ErrLog.Printf("[udp]: udp-%d not found\n", lport)
	} else {
		srv.Send(raddr, data)
	}
}
