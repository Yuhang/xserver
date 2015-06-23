package utils

import (
	"bytes"
	"fmt"
)

func FormatX(prefix string, bs []byte) string {
	if len(bs) == 0 {
		return ""
	}
	var b bytes.Buffer
	num := 0
	for {
		fmt.Fprintf(&b, "%s%08x ", prefix, num)
		max := 16
		if l := len(bs); l < max {
			max = l
		}
		for i := 0; i < max; i++ {
			if i != 0 && i != max-1 {
				if i%2 == 0 {
					fmt.Fprintf(&b, " ")
				}
			}
			fmt.Fprintf(&b, "%02x", bs[i])
		}
		bs = bs[max:]
		if len(bs) == 0 {
			return b.String()
		} else {
			fmt.Fprintf(&b, "\n")
		}
		num += max
	}
}

func FormatA(prefix string, bs []byte) string {
	if len(bs) == 0 {
		return ""
	}
	var b bytes.Buffer
	num := 0
	for {
		fmt.Fprintf(&b, "%s%08x ", prefix, num)
		max := 16
		for i := 0; i < max; i++ {
			if i != 0 && i != max-1 {
				if i%2 == 0 {
					fmt.Fprintf(&b, " ")
				}
			}
			if i < len(bs) {
				fmt.Fprintf(&b, "%02x", bs[i])
			} else {
				fmt.Fprintf(&b, "  ")
			}
		}
		fmt.Fprintf(&b, "    ")
		for i := 0; i < max; i++ {
			if i < len(bs) {
				c := bs[i]
				if c >= 32 && c <= 126 {
					fmt.Fprintf(&b, "%s", string(c))
				} else {
					fmt.Fprintf(&b, "%s", ".")
				}
			} else {
				fmt.Fprintf(&b, " ")
			}
		}
		if l := len(bs); l < max {
			max = l
		}
		bs = bs[max:]
		if len(bs) == 0 {
			return b.String()
		} else {
			fmt.Fprintf(&b, "\n")
		}
		num += max
	}
}

type Formatted []byte

func (bs Formatted) String() string {
	return FormatA("    ", []byte(bs))
}
