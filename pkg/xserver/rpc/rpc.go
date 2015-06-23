package rpc

import (
	"net"
)

import (
	"github.com/golang/protobuf/proto"

	"github.com/spinlock/xserver/pkg/xserver/args"
	"github.com/spinlock/xserver/pkg/xserver/async"
	"github.com/spinlock/xserver/pkg/xserver/counts"
	"github.com/spinlock/xserver/pkg/xserver/tcp"
	"github.com/spinlock/xserver/pkg/xserver/xlog"
)

func Join(xid uint32, raddr *net.UDPAddr) {
	if clt := tcp.GetClient(); clt == nil {
		return
	} else if bs, err := newXRequest(xid, raddr, "join", 0, nil, true); err != nil {
		counts.Count("rpc.join.error", 1)
		xlog.ErrLog.Printf("[rpc]: rpc join error = '%v'\n", err)
	} else {
		counts.Count("rpc.join", 1)
		async.Call(uint64(xid), func() {
			clt.Send(bs)
		})
	}
}

func Exit(xid uint32, raddr *net.UDPAddr) {
	if clt := tcp.GetClient(); clt == nil {
		return
	} else if bs, err := newXRequest(xid, raddr, "exit", 0, nil, true); err != nil {
		counts.Count("rpc.exit.error", 1)
		xlog.ErrLog.Printf("[rpc]: rpc exit error = '%v'\n", err)
	} else {
		counts.Count("rpc.exit", 1)
		async.Call(uint64(xid), func() {
			clt.Send(bs)
		})
	}
}

func Call(xid uint32, raddr *net.UDPAddr, callback float64, data []byte, reliable bool) {
	if clt := tcp.GetClient(); clt == nil {
		counts.Count("rpc.call.noclient", 1)
		xlog.ErrLog.Printf("[rpc]: rpc is disabled\n")
	} else if bs, err := newXRequest(xid, raddr, "call", callback, data, reliable); err != nil {
		counts.Count("rpc.call.error", 1)
		xlog.ErrLog.Printf("[rpc]: rpc call error = '%v'\n", err)
	} else {
		counts.Count("rpc.call", 1)
		async.Call(uint64(xid), func() {
			clt.Send(bs)
		})
	}
}

func newXRequest(xid uint32, raddr *net.UDPAddr, code string, callback float64, data []byte, reliable bool) ([]byte, error) {
	port := uint32(args.RpcListenPort())
	x := &XRequest{}
	x.Port = &port
	x.Code = &code
	x.Reliable = &reliable
	x.Callback = &callback
	if raddr != nil {
		addr := raddr.String()
		x.Addr = &addr
	}
	if len(data) != 0 {
		x.Data = data
	}
	x.Xid = &xid
	return proto.Marshal(x)
}

func DecodeXResponse(bs []byte) *XResponse {
	x := &XResponse{}
	if err := proto.Unmarshal(bs, x); err != nil {
		counts.Count("rpc.xresponse.error", 1)
		xlog.ErrLog.Printf("[rpc]: rpc decode.xresponse error = '%v'\n", err)
		return nil
	}
	return x
}

func DecodeXMessage(bs []byte) *XMessage {
	x := &XMessage{}
	if err := proto.Unmarshal(bs, x); err != nil {
		counts.Count("rpc.xmessage.error", 1)
		xlog.ErrLog.Printf("[rpc]: rpc decode.xmessage error = '%v'\n", err)
		return nil
	}
	return x
}
