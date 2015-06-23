package xserver

import (
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/args"
	"github.com/spinlock/xserver/pkg/xserver/counts"
	"github.com/spinlock/xserver/pkg/xserver/handshake"
	"github.com/spinlock/xserver/pkg/xserver/rpc"
	"github.com/spinlock/xserver/pkg/xserver/rtmfp"
	"github.com/spinlock/xserver/pkg/xserver/session"
	"github.com/spinlock/xserver/pkg/xserver/tcp"
	"github.com/spinlock/xserver/pkg/xserver/udp"
	"github.com/spinlock/xserver/pkg/xserver/utils"
	"github.com/spinlock/xserver/pkg/xserver/xlog"
)

import (
	_ "github.com/spinlock/xserver/pkg/xserver/http"
)

func Start() {
	shell := func(f func()) {
		s := func() {
			defer func() {
				if x := recover(); x != nil {
					counts.Count("server.panic", 1)
					xlog.ErrLog.Printf("[server]: panic = %v\n%s\n", x, utils.Trace())
				}
			}()
			f()
		}
		for {
			s()
		}
	}
	for _, udpsrv := range udp.GetServers() {
		s := udpsrv
		f := func() {
			for {
				if lport, addr, data := s.Recv(); addr != nil && len(data) != 0 {
					if xid, err := rtmfp.PacketXid(data); err != nil {
						continue
					} else if xid == 0 {
						handshake.HandlePacket(lport, addr, data)
					} else {
						session.HandlePacket(lport, addr, xid, data)
					}
				}
			}
		}
		for i := 0; i < args.Parallel(); i++ {
			go shell(f)
		}
	}
	if s := tcp.GetServer(); s != nil {
		f := func() {
			for {
				if bs := s.Recv(); len(bs) != 0 {
					if x := rpc.DecodeXMessage(bs); x != nil {
						if b := x.Broadcast; b != nil {
							xids, data, reliable := b.Xids, b.Data, *b.Reliable
							if len(xids) == 0 || len(data) == 0 {
								continue
							}
							session.RecvPull(xids, data, reliable)
						}
						if c := x.Close; c != nil {
							xids := c.Xids
							if len(xids) == 0 {
								continue
							}
							session.CloseAll(xids)
						}
					}
				}
			}
		}
		go shell(f)
	}
	if c := tcp.GetClient(); c != nil {
		f := func() {
			for {
				if bs := c.Recv(); len(bs) != 0 {
					if x := rpc.DecodeXResponse(bs); x != nil {
						xid, data, callback, reliable := *x.Xid, x.Data, *x.Callback, *x.Reliable
						if xid == 0 || len(data) == 0 {
							continue
						}
						session.Callback(xid, data, callback, reliable)
					}
				}
			}
		}
		go shell(f)
	}
	for {
		time.Sleep(time.Minute)
	}
}
