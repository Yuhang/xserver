package utils

import (
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

var (
	Starttime = time.Now().Unix()
)

func init() {
	go func() {
		for {
			time.Sleep(time.Minute * 5)
			runtime.GC()
		}
	}()
	log.Printf("[compile]: %s\n", Compile)
	log.Printf("[version]: %s\n", Version)
}

func Trace() string {
	b := make([]byte, 4096)
	m := string(b[:runtime.Stack(b, false)])
	if i := strings.LastIndex(m, "\n"); i == -1 || i == len(m)-1 {
		return m
	} else {
		return m[:i+1] + "... ...\n"
	}
}

func Panic(msg string) {
	log.Printf("[panic] %s, trace = \n%s\n", msg, Trace())
	os.Exit(-1)
}
