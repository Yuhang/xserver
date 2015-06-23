package amf3

import (
	"container/list"
)

type Traits struct {
	infos *list.List
}

func NewTraits() *Traits {
	t := &Traits{}
	t.infos = list.New()
	return t
}

func (t *Traits) add(s string) {
	t.infos.PushBack(s)
}
