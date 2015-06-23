package session

import (
	"bytes"
	"fmt"
)

const (
	flagsHeader     = 0x80
	flagsWithAfter  = 0x10
	flagsWithBefore = 0x20
	flagsAbandoned  = 0x02
	flagsEnd        = 0x01
)

type fragment struct {
	stage    uint64
	flags    uint8
	data     []byte
	sendtime int64
}

func (f *fragment) WithAfter() bool {
	return (f.flags & flagsWithAfter) != 0
}

func (f *fragment) WithBefore() bool {
	return (f.flags & flagsWithBefore) != 0
}

func (f *fragment) Abandoned() bool {
	return (f.flags & flagsAbandoned) != 0
}

func (f *fragment) End() bool {
	return (f.flags & flagsEnd) != 0
}

func (f *fragment) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "{")
	fmt.Fprintf(&buf, "%d:", f.stage)
	if f.Abandoned() {
		fmt.Fprintf(&buf, "A")
	} else {
		ff, fl := !f.WithBefore(), !f.WithAfter()
		if ff && fl {
			fmt.Fprintf(&buf, "M")
		} else if ff {
			fmt.Fprintf(&buf, "[")
		} else if fl {
			fmt.Fprintf(&buf, "]")
		} else {
			fmt.Fprintf(&buf, "+")
		}
	}
	if f.End() {
		fmt.Fprintf(&buf, "E")
	}
	fmt.Fprintf(&buf, "}")
	return buf.String()
}
