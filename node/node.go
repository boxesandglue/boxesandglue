package node

import (
	"container/list"
)

var (
	ids chan int
)

func genIntegerSequence(ids chan int) {
	i := int(0)
	for {
		ids <- i
		i++
	}
}

func init() {
	ids = make(chan int)
	go genIntegerSequence(ids)
}

type Nodelist struct {
	List   *list.List
	Width  float64
	Height float64
}

func NewNodelist() *Nodelist {
	n := &Nodelist{}
	n.List = list.New()
	return n
}

type basenode struct {
	id int
}

// NewElement creates a list element from the node type. You must ensure that the val is a
// valid node type.
func NewElement(val interface{}) *list.Element {
	return &list.Element{Value: val}
}

type Glyph struct {
	basenode
	GlyphID    int      // The font specific glyph id
	Components []string // A codepoint can contain more than one rune, for example a fi ligature contains f + i
}

func NewGlyph() *Glyph {
	n := &Glyph{}
	n.id = <-ids
	return n
}

func IsGlyph(elt *list.Element) (*Glyph, bool) {
	n, ok := elt.Value.(*Glyph)
	return n, ok
}

type Glue struct {
	basenode
	Width        float64
	Stretch      int
	Shrink       int
	StretchOrder int
	ShrinkOrder  int
}

func NewGlue() *Glue {
	n := &Glue{}
	n.id = <-ids
	return n
}

func IsGlue(elt *list.Element) (*Glue, bool) {
	n, ok := elt.Value.(*Glue)
	return n, ok
}

type HList struct {
	basenode
	List *list.Element
}

func NewHList() *HList {
	n := &HList{}
	n.id = <-ids
	return n
}

func IsHList(elt *list.Element) (*HList, bool) {
	hlist, ok := elt.Value.(*HList)
	return hlist, ok
}
