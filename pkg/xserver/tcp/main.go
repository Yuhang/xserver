package tcp

import (
	"github.com/spinlock/xserver/pkg/xserver/args"
)

var (
	srv *Server
	clt *Client
)

func init() {
	if port := args.RpcListenPort(); port != 0 {
		srv = newServer(port)
	}
	if ip, port := args.RpcRemote(); port != 0 {
		clt = newClient(ip, port)
	}
}

func GetServer() *Server {
	return srv
}

func GetClient() *Client {
	return clt
}
