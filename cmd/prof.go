package main

import (
	_ "net/http/pprof"

	"github.com/spinlock/xserver/pkg/xserver"
)

func main() {
	xserver.Start()
}
