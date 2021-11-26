package node

import (
	"math"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/lang"
)

// LinebreakSettings contains all information about the final paragraph
type LinebreakSettings struct {
	HSize                bag.ScaledPoint
	LineHeight           bag.ScaledPoint
	DemeritsFitness      int
	DoublehyphenDemerits int
	Tolerance            float64
}

// NewLinebreakSettings returns a settings struct with defaults initialized.
func NewLinebreakSettings() *LinebreakSettings {
	ls := &LinebreakSettings{
		DoublehyphenDemerits: 3000,
		DemeritsFitness:      100,
		Tolerance:            positiveInf,
	}

	return ls
}

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
	if head == nil {
		return insert
	}
	if cur == nil || cur == head {
		insert.SetNext(head)
		head.SetPrev(insert)
		return insert
	}

	curPrev := cur.Prev()
	if curPrev != nil {
		curPrev.SetNext(insert)
		insert.SetPrev(curPrev)
	}
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

// CopyList makes a deep copy of the list starting at nl.
func CopyList(nl Node) Node {
	var copied, tail Node
	copied = nl.Copy()
	tail = copied
	for e := nl.Next(); e != nil; e = e.Next() {
		c := e.Copy()
		tail.SetNext(c)
		c.SetPrev(tail)
		tail = c
	}
	return copied
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
		case *Rule:
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

// HpackTo returns a HList node with the node list as its list and the badness.
// The width is the desired width.
func HpackTo(firstNode Node, width bag.ScaledPoint) (*HList, int) {
	return HPackToWithEnd(firstNode, Tail(firstNode), width)
}

// HPackToWithEnd returns a HList node with nl as its list. The width is the
// desired width. The list stops at lastNode (including lastNode).
func HPackToWithEnd(firstNode Node, lastNode Node, width bag.ScaledPoint) (*HList, int) {
	glues := []*Glue{}

	sumwd := bag.ScaledPoint(0)
	nonGlueSumWd := bag.ScaledPoint(0) // used for real width calculation

	totalStretchability := [4]bag.ScaledPoint{0, 0, 0, 0}
	totalShrinkability := [4]bag.ScaledPoint{0, 0, 0, 0}

	for e := firstNode; e != nil; e = e.Next() {
		switch v := e.(type) {
		case *Glue:
			sumwd += v.Width
			totalStretchability[v.StretchOrder] += v.Stretch
			totalShrinkability[v.StretchOrder] += v.Shrink
			glues = append(glues, v)
		default:
			nonGlueSumWd += getWidth(v)
		}

		if e == lastNode {
			if e.Next() != nil {
				e.Next().SetPrev(nil)
				e.SetNext(nil)
			}
			break
		}
	}
	sumwd += nonGlueSumWd

	var highestOrderStretch, highestOrderShrink GlueOrder
	stretchability, shrinkability := totalStretchability[0], totalShrinkability[0]

	for i := GlueOrder(3); i > 0; i-- {
		if totalStretchability[i] != 0 && highestOrderStretch < i {
			highestOrderStretch = i
			stretchability = totalStretchability[i]
		}
		if totalShrinkability[i] != 0 && highestOrderShrink < i {
			highestOrderShrink = i
			shrinkability = totalShrinkability[i]
		}
	}

	var r float64
	if width == sumwd {
		r = 1
	} else if sumwd < width {
		// a short line
		r = float64(width-sumwd) / float64(stretchability)
	} else {
		// a long line
		r = float64(width-sumwd) / float64(shrinkability)
	}

	badness := 10000
	if r >= -1 {
		badness = int(math.Round(math.Pow(math.Abs(r), 3) * 100.0))
		if badness > 10000 {
			badness = 10000
		}
	}
	if highestOrderShrink > 0 || highestOrderStretch > 0 {
		badness = 0
	}
	// calculate the real width: non-glue widths + new glue widths
	sumwd = nonGlueSumWd
	for _, g := range glues {
		if r >= 0 && highestOrderStretch == g.StretchOrder {
			if g.StretchOrder == 0 {
				g.Width += bag.ScaledPoint(r * float64(g.Stretch))
			} else {
				g.Width += bag.ScaledPoint(r * float64(g.Stretch))
			}
		} else if r <= 0 && highestOrderShrink == g.ShrinkOrder {
			if g.ShrinkOrder == 0 {
				g.Width += bag.ScaledPoint(r * float64(g.Shrink))
			} else {
				g.Width += bag.ScaledPoint(r * float64(g.Shrink))
			}
		}
		sumwd += g.Width
		g.Stretch = 0
		g.Shrink = 0
	}

	hl := NewHList()
	hl.List = firstNode
	hl.Width = sumwd
	hl.GlueSet = r
	return hl, badness
}
