package node

import (
	"fmt"
	"math"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/frontend/pdfdraw"
)

// Direction represents the direction of the node list. This can be horizontal or vertical.
type Direction bool

const (
	// Horizontal is the direction from left to right or from right to left.
	Horizontal Direction = true
	// Vertical is the direction from top to bottom or from bottom to top.
	Vertical Direction = false
)

// LinebreakSettings controls the line breaking algorithm.
type LinebreakSettings struct {
	SqueezeOverfullBoxes  bool
	DemeritsFitness       int
	DoublehyphenDemerits  int
	HangingPunctuationEnd bool
	FontExpansion         float64
	HSize                 bag.ScaledPoint
	Hyphenpenalty         int
	Indent                bag.ScaledPoint
	IndentRows            int
	LineEndGlue           *Glue
	LineHeight            bag.ScaledPoint
	LineStartGlue         *Glue
	OmitLastLeading       bool
	Tolerance             float64
}

// NewLinebreakSettings returns a settings struct with defaults initialized.
func NewLinebreakSettings() *LinebreakSettings {
	ls := &LinebreakSettings{
		DoublehyphenDemerits: 3000,
		DemeritsFitness:      100,
		Hyphenpenalty:        50,
		Tolerance:            positiveInf,
		LineStartGlue:        NewGlue(),
		LineEndGlue:          NewGlue(),
	}

	return ls
}

// DeleteFromList removes the node cur from the list starting at head. The
// possible new head is returned.
func DeleteFromList(head, cur Node) Node {
	if cur == nil {
		return head
	}
	p := cur.Prev()
	n := cur.Next()
	if head == cur {
		head = n
	}
	if p != nil {
		p.SetNext(n)
	}
	if n != nil {
		n.SetPrev(p)
	}
	return head
}

// InsertAfter inserts the node insert right after cur. If cur is nil then
// insert is the new head. This method returns the head node.
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

// InsertBefore inserts the node insert before the current not cur. It returns
// the (perhaps) new head node.
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
		return nil
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
	if nl == nil {
		return nil
	}
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

// Dimensions returns the width, height and the depth of the node list starting
// at n and ending with the stop node or at the end if stop is nil. If dir is
// Horizontal, then calculate in horizontal mode, otherwise in vertical mode.
func Dimensions(start Node, stop Node, dir Direction) (bag.ScaledPoint, bag.ScaledPoint, bag.ScaledPoint) {
	var sumwd, sumht, sumdp bag.ScaledPoint

	for e := start; e != nil; e = e.Next() {
		wd := getWidth(e, dir)
		sumwd += wd

		ht, dp := getHeight(e, dir)
		if ht > sumht {
			sumht = ht
		}
		if dp > sumdp {
			sumdp = dp
		}
		if e == stop {
			break
		}
	}
	return sumwd, sumht, sumdp
}

type hpackSetting struct {
	fontexpansion        float64
	squeezeOverfullBoxes bool
}

// HpackOption controls the packaging of the box.
type HpackOption func(*hpackSetting)

// FontExpansion sets the allowed font expansion (0-1).
func FontExpansion(amount float64) HpackOption {
	return func(p *hpackSetting) {
		p.fontexpansion = amount
	}
}

// SqueezeOverfullBoxes avoids overfull boxes by shrinking more than allowed.
func SqueezeOverfullBoxes(avoid bool) HpackOption {
	return func(p *hpackSetting) {
		p.squeezeOverfullBoxes = avoid
	}
}

// Hpack returns a HList node with the node list as its list
func Hpack(firstNode Node) *HList {
	sumwd := bag.ScaledPoint(0)
	maxht := bag.ScaledPoint(0)
	maxdp := bag.ScaledPoint(0)

	for e := firstNode; e != nil; e = e.Next() {
		switch v := e.(type) {
		case *Glyph:
			sumwd = sumwd + v.Width
			if v.Height > maxht {
				maxht = v.Height
			}
			if v.Depth > maxdp {
				maxdp = v.Depth
			}
		case *Glue:
			sumwd = sumwd + v.Width
		case *HList:
			sumwd = sumwd + v.Width
			if v.Height > maxht {
				maxht = v.Height
			}
			if v.Depth > maxdp {
				maxdp = v.Depth
			}
		case *Kern:
			sumwd += v.Kern
		case *Lang:
		case *Penalty:
			sumwd += v.Width
		case *VList:
			sumwd += v.Width
			if v.Height > maxht {
				maxht = v.Height
			}
			if v.Depth > maxdp {
				maxdp = v.Depth
			}
		case *Rule:
			sumwd += v.Width
			if v.Height > maxht {
				maxht = v.Height
			}
			if v.Depth > maxdp {
				maxdp = v.Depth
			}
		case *Image:
			sumwd += v.Width
			if v.Height > maxht {
				maxht = v.Height
			}
		default:
			bag.Logger.Error(fmt.Sprintf("Hpack: unknown node %v", v))
		}
	}
	hl := NewHList()
	hl.List = firstNode
	hl.Width = sumwd
	hl.Height = maxht
	hl.Depth = maxdp
	return hl
}

// HpackTo returns a HList node with the node list as its list.
// The width is the desired width.
func HpackTo(firstNode Node, width bag.ScaledPoint) *HList {
	return HpackToWithEnd(firstNode, Tail(firstNode), width)
}

// HpackToWithEnd returns a HList node with nl as its list. The width is the
// desired width. The list stops at lastNode (including lastNode).
func HpackToWithEnd(firstNode Node, lastNode Node, width bag.ScaledPoint, opts ...HpackOption) *HList {
	hs := &hpackSetting{}
	for _, opt := range opts {
		opt(hs)
	}
	glues := []*Glue{}

	sumwd := bag.ScaledPoint(0)
	sumGlyph := bag.ScaledPoint(0)
	maxht := bag.ScaledPoint(0)
	maxdp := bag.ScaledPoint(0)

	totalStretchability := [4]bag.ScaledPoint{0, 0, 0, 0}
	totalShrinkability := [4]bag.ScaledPoint{0, 0, 0, 0}
	totalExtend := bag.ScaledPoint(0)

	for e := firstNode; e != nil; e = e.Next() {
		switch v := e.(type) {
		case *Glue:
			sumwd += v.Width
			totalStretchability[v.StretchOrder] += v.Stretch
			totalShrinkability[v.StretchOrder] += v.Shrink
			glues = append(glues, v)
		case *Glyph:
			sumwd += v.Width
			if v.Height > maxht {
				maxht = v.Height
			}
			if v.Depth > maxdp {
				maxdp = v.Depth
			}
			if hs.fontexpansion != 0 {
				extend := bag.MultiplyFloat(v.Width, hs.fontexpansion)
				totalExtend += extend
			}
			sumGlyph += v.Width
		case *Rule:
			sumwd += v.Width
			if v.Height > maxht {
				maxht = v.Height
			}
			if v.Depth > maxdp {
				maxdp = v.Depth
			}
		case *VList:
			sumwd += getWidth(v, Vertical)
			ht, dp := getHeight(v, Vertical)
			if ht > maxht {
				maxht = ht
			}
			if dp > maxdp {
				maxdp = dp
			}

		default:
			sumwd += getWidth(v, Horizontal)
			ht, dp := getHeight(v, Vertical)
			if ht > maxht {
				maxht = ht
			}
			if dp > maxdp {
				maxdp = dp
			}
		}

		if e == lastNode {
			if e.Next() != nil {
				e.Next().SetPrev(nil)
				e.SetNext(nil)
			}
			break
		}
	}

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
	if r < -1 {
		// Badness 1000000 for overfull boxes
		badness = 1000000
	} else if r >= -1 {
		badness = int(math.Round(math.Pow(math.Abs(r), 3) * 100.0))
		if badness > 10000 {
			badness = 10000
		}
	}
	useExpand := false
	if hs.fontexpansion != 0 {
		if r < -1 {
			r = -1
			useExpand = true
		}
	}
	for _, g := range glues {
		if r >= 0 && highestOrderStretch == g.StretchOrder {
			g.Width += bag.ScaledPoint(r * float64(g.Stretch))
		} else if r >= -1 && r <= 0 && highestOrderShrink == g.ShrinkOrder {
			g.Width += bag.ScaledPoint(r * float64(g.Shrink))
		} else if r < -1 && highestOrderShrink == g.ShrinkOrder {
			if hs.squeezeOverfullBoxes {
				g.Width += bag.ScaledPoint(r * float64(g.Shrink))
			}
		}
	}
	hl := NewHList()
	hl.List = firstNode
	hl.Width = width
	hl.Depth = maxdp
	hl.Height = maxht
	hl.GlueSet = r
	hl.Badness = badness
	if useExpand {
		a := (sumwd - width - shrinkability).ToPT() / sumGlyph.ToPT()
		hl.Attributes = H{"expand": int(-1 * a * 100)}
	}
	return hl
}

// Vpack creates a list
func Vpack(firstNode Node) *VList {
	sumht := bag.ScaledPoint(0)
	maxwd := bag.ScaledPoint(0)

	var lastNode Node
	for e := firstNode; e != nil; e = e.Next() {
		ht, dp := getHeight(e, Vertical)
		sumht += ht + dp
		if wd := getWidth(e, Vertical); wd > maxwd {
			maxwd = wd
		}
		lastNode = e
	}
	vl := NewVList()
	vl.List = firstNode
	vl.Depth = getDepth(lastNode)
	vl.Height = sumht - getDepth(lastNode)
	vl.Width = maxwd
	return vl
}

// Boxit draws a thin rectangle around the box.
func Boxit(n Node) Node {
	r := NewRule()
	r.Hide = true
	switch t := n.(type) {
	case *VList:
		p := pdfdraw.NewStandalone().LineWidth(bag.MustSP("0.4pt")).Rect(0, 0, t.Width, -t.Height+t.Depth).Stroke()
		r.Pre = p.String()
		t.List = InsertBefore(t.List, t.List, r)
	case *HList:
		p := pdfdraw.NewStandalone().LineWidth(bag.MustSP("0.4pt")).Rect(0, 0, t.Width, -t.Height+t.Depth).Stroke()
		r.Pre = p.String()
		t.List = InsertBefore(t.List, t.List, r)
	}
	return n
}

func getWidth(n Node, dir Direction) bag.ScaledPoint {
	switch t := n.(type) {
	case *Glue:
		if dir == Horizontal {
			return t.Width
		}
		return 0
	case *Glyph:
		return t.Width
	case *Penalty:
		return t.Width
	case *Rule:
		return t.Width
	case *HList:
		return t.Width
	case *Image:
		return t.Width
	case *Kern:
		return t.Kern
	case *VList:
		return t.Width
	case *StartStop, *Disc, *Lang:
		return 0
	default:
		bag.Logger.Error(fmt.Sprintf("getWidth: unknown node type %T", n))
	}
	return 0
}

// getHeight returns the height and the depth of the node list starting at n.
// Depending on the progressing direction, the height of a glue is either 0 or
// the glue width.
func getHeight(n Node, dir Direction) (bag.ScaledPoint, bag.ScaledPoint) {
	switch t := n.(type) {
	case *HList:
		return t.Height, t.Depth
	case *Glyph:
		return t.Height, t.Depth
	case *VList:
		return t.Height, t.Depth
	case *Rule:
		return t.Height, t.Depth
	case *Image:
		return t.Height, 0
	case *Glue:
		if dir == Vertical {
			return t.Width, 0
		}
		return 0, 0
	case *StartStop, *Disc, *Lang, *Penalty, *Kern:
		return 0, 0
	default:
		bag.Logger.Error("getHeight: unknown node type", "type", fmt.Sprintf("%T", n))
	}
	return 0, 0
}

func getDepth(n Node) bag.ScaledPoint {
	switch t := n.(type) {
	case *HList:
		return t.Depth
	case *Glyph:
		return t.Depth
	case *Rule:
		return t.Depth
	case *StartStop, *Disc, *Lang, *Glue, *Penalty, *Kern:
		return 0
	case *VList:
		return t.Depth
	default:
		bag.Logger.Error("getDepth: unknown node type", "type", fmt.Sprintf("%T", n))
	}
	return 0
}
