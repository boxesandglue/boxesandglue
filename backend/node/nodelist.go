package node

import (
	"fmt"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/lang"
)

// InsertAfter inserts the node insert right after cur. If cur is nil then insert is the new head. This method retuns the head node.
func InsertAfter(head, cur, insert Node) Node {
	if cur == nil {
		return insert
	}
	curNext := cur.Next()
	if curNext != nil {
		insert.SetNext(curNext)
		curNext.SetPrev(insert)
	}
	cur.SetNext(insert)
	insert.SetPrev(cur)
	if head == nil {
		return cur
	}
	return head
}

// InsertBefore inserts the node insert before the current not cur. It returns the (perhaps) new head node.
func InsertBefore(head, cur, insert Node) Node {
	curPrev := cur.Prev()
	curPrev.SetNext(insert)
	insert.SetPrev(curPrev)
	cur.SetPrev(insert)
	insert.SetNext(cur)
	return head
}

// Tail returns the last node of a node list.
func Tail(nl Node) Node {
	if nl == nil {
		return nl
	}
	if nl.Next() == nil {
		return nl
	}
	var e Node

	for e = nl; e.Next() != nil; e = e.Next() {
	}
	return e
}

func insertBreakpoints(l *lang.Lang, word *strings.Builder, wordstart Node) {
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
			InsertBefore(wordstart, cur, disc)
			cur = cur.Next()
		}
	}
}

// Hyphenate inserts hyphenation points in to the list
func Hyphenate(nodelist Node) {
	// hyphenation points should be inserted when a language changes or when
	// the word ends (with a comma or a space for example).
	var curlang *lang.Lang
	var wordboundary bool
	var prevhyphenate bool
	var wordstart Node
	var b strings.Builder
	e := nodelist
	for {
		switch v := e.(type) {
		case *Glyph:
			if prevhyphenate != v.Hyphenate {
				wordboundary = true
			}

			if wordboundary {
				insertBreakpoints(curlang, &b, wordstart)
				wordstart = e
			}

			prevhyphenate = v.Hyphenate
			wordboundary = false
			if v.Hyphenate {
				if v.Components != "" {
					b.WriteString(v.Components)
				} else {
					b.WriteRune(rune(v.Codepoint))
				}
			}
		case *Lang:
			fmt.Println("lang")
			curlang = v.Lang
			wordboundary = true
		default:
			fmt.Println("other")
			wordboundary = true

		}
		e = e.Next()
		if e == nil {
			break
		}
	}
	insertBreakpoints(curlang, &b, wordstart)
}

// Hpack returns a HList node with the node list as its list
func Hpack(firstNode Node) *HList {
	sumwd := bag.ScaledPoint(0)
	glues := []Node{}
	sumglue := 0

	for e := firstNode; e != nil; e = e.Next() {
		if e == nil {
			break
		}
		switch v := e.(type) {
		case *Glyph:
			sumwd = sumwd + v.Width
		case *Glue:
			sumwd = sumwd + v.Width
			sumglue = sumglue + int(v.Width)
			glues = append(glues, e)
		case *Lang:
		case *Penalty:
			sumwd += v.Width
		default:
			bag.Logger.DPanic(v)
		}
	}
	hl := NewHList()
	hl.List = firstNode
	hl.Width = sumwd
	return hl
}

// HpackTo returns a HList node with the node list as its list.
// The width is supposed to be width, but can be different if not
// enough glue is found in the list
func HpackTo(firstNode Node, width bag.ScaledPoint) *HList {
	return HPackToWithEnd(firstNode, Tail(firstNode), width)
}

// HPackToWithEnd returns a HList node with nl as its list.
// The width is supposed to be width, but can be different if not
// enough glue is found in the list. The list stops at lastNode (including lastNode).
func HPackToWithEnd(firstNode Node, lastNode Node, width bag.ScaledPoint) *HList {
	sumwd := bag.ScaledPoint(0)
	glues := []Node{}

	sumglue := 0

	for e := firstNode; e != nil; e = e.Next() {
		switch v := e.(type) {
		case *Glyph:
			sumwd = sumwd + v.Width
		case *Glue:
			sumwd = sumwd + v.Width
			sumglue = sumglue + int(v.Width)
			glues = append(glues, e)
		}

		if e == lastNode {
			if e.Next() != nil {
				e.Next().SetPrev(nil)
			}
			e.SetNext(nil)
			break
		}
	}

	delta := width - sumwd
	stretch := float64(delta) / float64(sumglue)

	for _, elt := range glues {
		g := elt.(*Glue)
		g.Width += bag.ScaledPoint(float64(g.Width) * stretch)
	}

	// re-calculate sum to get an exact result
	sumwd = 0
	for e := firstNode; e != nil; e = e.Next() {
		switch v := e.(type) {
		case *Glyph:
			sumwd = sumwd + v.Width
		case *Glue:
			sumwd = sumwd + v.Width
			glues = append(glues, e)
		}
	}

	hl := NewHList()
	hl.List = firstNode
	hl.Width = sumwd
	return hl
}
