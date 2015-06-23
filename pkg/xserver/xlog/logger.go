package xlog

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"
)

import (
	"github.com/spinlock/xserver/pkg/xserver/args"
	"github.com/spinlock/xserver/pkg/xserver/utils"
)

var (
	OutLog, ErrLog, SssLog, TcpLog *Logger
)

type Logger struct {
	filename string
	file     *os.File
	Printf   func(format string, v ...interface{})
}

func init() {
	loggers := make(map[string]*Logger)

	OutLog = newLogger(loggers, "outlog")
	ErrLog = newLogger(loggers, "errlog")
	SssLog = newLogger(loggers, "ssslog")
	TcpLog = newLogger(loggers, "tcplog")

	go func() {
		debug := args.IsDebug()
		for {
			time.Sleep(time.Second)
			for _, l := range loggers {
				if l.file == nil {
					if file, err := os.OpenFile(l.filename, os.O_WRONLY|os.O_APPEND, 0666); err == nil {
						log.Printf("[logger]: open '%s'\n", l.filename)
						l.file = file
						logger := log.New(file, "", log.LstdFlags)
						l.Printf = func(format string, v ...interface{}) {
							logger.Printf(format, v...)
							if debug {
								log.Printf(format, v...)
							}
						}
					}
				} else {
					if _, err := os.Stat(l.filename); err != nil {
						log.Printf("[logger]: close '%s'\n", l.filename)
						l.Printf = func(format string, v ...interface{}) {}
						l.file.Close()
						l.file = nil
					}
				}
			}
		}
	}()
}

func newLogger(loggers map[string]*Logger, name string) *Logger {
	if l := loggers[name]; l != nil {
		return l
	} else {
		if len(name) == 0 {
			utils.Panic("invalid logger name")
		}
		l = &Logger{}
		l.filename = fmt.Sprintf("log/%s", name)
		l.file = nil
		l.Printf = func(format string, v ...interface{}) {}
		loggers[name] = l
		log.Printf("[logger]: add logger '%s'\n", l.filename)
		return l
	}
}

func StringToHex(s string) encodedString {
	return encodedString(s)
}

func BytesToHex(bs []byte) encodedBytes {
	return encodedBytes(bs)
}

type encodedString string

func (e encodedString) String() string {
	return hex.EncodeToString([]byte(e))
}

type encodedBytes []byte

func (e encodedBytes) String() string {
	return hex.EncodeToString(e)
}
