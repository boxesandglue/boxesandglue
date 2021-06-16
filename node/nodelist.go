package node

import (
	"strings"

	"github.com/speedata/texperiments/bag"
	"github.com/speedata/texperiments/lang"
)

// Nodelist contains a list of nodes
// type Nodelist *list.List

// NewNodelist creates an empty node list

// AppendNode appends the node val to the end of the node list nl.
func (nl *Nodelist) AppendNode(val interface{}) *Node {
	return nl.PushBack(val)
}

func insertBreakpoints(l *lang.Lang, word *strings.Builder, nodelist *Nodelist, wordstart *Node) {
	cur := wordstart
	if word.Len() > 0 {
		str := word.String()
		word.Reset()
		bp := l.Hyphenate(str)
		for _, step := range bp {
			for i := 0; i <= step-1; i++ {
				cur = cur.Next()
			}
			disc := NewDiscWithContents(&Disc{})
			cur = nodelist.InsertBefore(disc, cur)
			cur = cur.Next()
		}
	}
}

// Hyphenate inserts hyphenation points in to the list
func Hyphenate(nodelist *Nodelist) {
	// hyphenation points should be inserted when a language changes or when
	// the word ends (with a comma or a space for example).
	nl := nodelist
	var curlang *lang.Lang
	var wordboundary bool
	var prevhyphenate bool
	var wordstart *Node
	var b strings.Builder
	e := nl.Front()
	for {
		switch v := e.Value.(type) {
		case *Glyph:
			if prevhyphenate != v.Hyphenate {
				wordboundary = true
			}

			if wordboundary {
				insertBreakpoints(curlang, &b, nodelist, wordstart)
				wordstart = e
			}

			prevhyphenate = v.Hyphenate
			wordboundary = false
			if v.Hyphenate {
				if v.Components != "" {
					b.WriteString(v.Components)
				} else {
					b.WriteRune(rune(v.GlyphID))
				}
			}
		case *Lang:
			curlang = v.Lang
			wordboundary = true
		default:
			wordboundary = true

		}
		e = e.Next()
		if e == nil {
			break
		}
	}
	insertBreakpoints(curlang, &b, nodelist, wordstart)
}

// Hpack returns a HList node with the node list as its list
func Hpack(nl *Nodelist) *HList {
	sumwd := bag.ScaledPoint(0)
	for e := nl.Front(); e != nil; e = e.Next() {
		switch v := e.Value.(type) {
		case *Glyph:
			sumwd = sumwd + v.Width
		case *Glue:
			sumwd = sumwd + v.Width
		}
	}

	hl := NewHList()
	hl.List = nl
	hl.Width = sumwd
	return hl
}

// HpackTo returns a HList node with the node list as its list.
// The width is supposed to be width, but can be different if not
// enough glue is found in the list
func HpackTo(firstNode *Node, width bag.ScaledPoint) *HList {
	return HPackToWithEnd(firstNode, firstNode.list.Back(), width)
}

// HPackToWithEnd returns a HList node with nl as its list.
// The width is supposed to be width, but can be different if not
// enough glue is found in the list. The list stops at lastNode (including lastNode).
func HPackToWithEnd(firstNode *Node, lastNode *Node, width bag.ScaledPoint) *HList {
	sumwd := bag.ScaledPoint(0)
	nl := firstNode.list
	glues := []*Node{}
	sumglue := 0

	newlist := NewNodelist()

	e := firstNode
	for {
		if e == nil {
			break
		}
		nextE := e.Next()
		switch v := e.Value.(type) {
		case *Glyph:
			sumwd = sumwd + v.Width
		case *Glue:
			sumwd = sumwd + v.Width
			sumglue = sumglue + int(v.Width)
			glues = append(glues, e)
		}

		val := nl.Remove(e)
		newlist.PushBack(val)
		if e == lastNode {
			break
		}
		e = nextE
	}

	delta := width - sumwd
	stretch := float64(delta) / float64(sumglue)

	for _, elt := range glues {
		g := elt.Value.(*Glue)
		g.Width += bag.ScaledPoint(float64(g.Width) * stretch)
	}

	// re-calculate sum to get an exact result
	sumwd = 0
	for e := newlist.Front(); e != nil; e = e.Next() {
		switch v := e.Value.(type) {
		case *Glyph:
			sumwd = sumwd + v.Width
		case *Glue:
			sumwd = sumwd + v.Width
			glues = append(glues, e)
		}
	}

	hl := NewHList()
	hl.List = newlist
	hl.Width = sumwd
	return hl
}
