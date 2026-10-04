package node

import (
	"maps"
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

// TextDirection is the inline direction a line box is laid out in: the
// side its text starts on. It is the paragraph's base direction from UAX#9,
// not the direction of a run inside the line. LuaTeX keeps the same thing
// as the dir field of an hlist.
type TextDirection uint8

const (
	// TextDirLTR is left to right, the zero value.
	TextDirLTR TextDirection = iota
	// TextDirRTL is right to left: the line starts at its right edge.
	TextDirRTL
)

func (d TextDirection) String() string {
	if d == TextDirRTL {
		return "rtl"
	}
	return "ltr"
}

// LinebreakSettings controls the line breaking algorithm. Start from
// NewLinebreakSettings: it sets the defaults named below, and Linebreak needs
// the LineStartGlue and LineEndGlue it creates.
type LinebreakSettings struct {
	// LineEndGlue is put at the right end of every line, TeX's \rightskip.
	// Stretch of order fil or higher sets the lines ragged right, and
	// centered together with the same stretch in LineStartGlue. IndentRight
	// is added to its width. It must not be nil; the default is a glue of
	// zero width.
	LineEndGlue *Glue
	// LineStartGlue is put at the left end of every line, TeX's \leftskip.
	// Stretch of order fil or higher sets the lines ragged left. Indent is
	// added to its width. It must not be nil; the default is a glue of zero
	// width.
	LineStartGlue *Glue
	// DemeritsFitness is added to the demerits of a line whose fitness
	// class (tight, decent, loose or very loose) is more than one class
	// away from the line before, TeX's \adjdemerits. Default 100.
	DemeritsFitness int
	// DoublehyphenDemerits is added when two lines in a row end at a
	// discretionary, TeX's \doublehyphendemerits. Default 3000.
	DoublehyphenDemerits int
	// EmergencyStretch is stretch the line breaker adds to every line it
	// measures, TeX's \emergencystretch. Unlike TeX, which uses it in a
	// last pass only, it counts for every line. Packing a line does not
	// know it, so a line that needs it comes out loose. Default 0.
	EmergencyStretch bag.ScaledPoint
	// FontExpansion is the fraction of its width by which a glyph may be
	// narrowed or widened, 0.02 for 2 percent (hz, microtype's expansion).
	// The line breaker counts it as stretch and shrink; packing a line uses
	// it only to narrow the glyphs of a line that is still too wide when
	// its glue has shrunk all it can. Default 0, off.
	FontExpansion float64
	// HSize is the width of every line, TeX's \hsize. Indent and IndentRight
	// are taken off it.
	HSize bag.ScaledPoint
	// Hyphenpenalty is added to a discretionary's own penalty when a line
	// breaks there, TeX's \hyphenpenalty. Default 50.
	Hyphenpenalty int
	// Indent is the left inset of the rows IndentRows selects. It moves their
	// content to the right and shortens them, as a paragraph indent or a
	// hanging indent does.
	Indent bag.ScaledPoint
	// IndentRows selects the rows Indent applies to: 0 all rows, a positive
	// n the first n rows, a negative n every row but the first n. That is
	// TeX's \hangafter with the sign turned around. IndentForRow returns
	// the inset of a row.
	IndentRows int
	// IndentRight is the mirror of Indent: it narrows a line from the right
	// without moving where the line starts. IndentRightRows selects the rows it
	// applies to with the same sign convention as IndentRows (positive: the
	// first n rows; negative: every row but the first n; 0: all rows).
	//
	// The two are separate because they are not the same operation. Indent
	// shifts the line's content to the right as well as shortening it, which is
	// what a paragraph indent means; IndentRight only takes width away from the
	// end, which is what is needed to lay text beside something on the right:
	// the line box still spans the full HSize, so alignment inside the narrowed
	// measure keeps working.
	IndentRight     bag.ScaledPoint
	IndentRightRows int
	// LineHeight is the room a line takes up, its box and the glue below it
	// together: Linebreak pads each line to LineHeight with glue. Lines of
	// the same height are therefore LineHeight apart from baseline to
	// baseline. A line taller than LineHeight gets no glue. HalfLeading and
	// LineModel change how the room is made.
	LineHeight bag.ScaledPoint
	// Tolerance is the largest adjustment ratio a line may have, the amount
	// its glue stretches as a multiple of its stretchability. A break that
	// needs more is not feasible; if no break is, Linebreak still breaks
	// and leaves a line overfull. Unlike TeX's \tolerance it is a ratio,
	// not a badness: the default 4.0 is a badness of 6400, and TeX's 200
	// is a ratio of about 1.26.
	Tolerance float64
	// SqueezeOverfullBoxes lets the glue of a line that is still too wide
	// shrink past its shrinkability, so the line keeps to HSize with spaces
	// narrower than allowed. It has no effect with FontExpansion, which
	// narrows the glyphs instead. Default false.
	SqueezeOverfullBoxes bool
	// HangingPunctuationEnd lets punctuation at the end of a line, and the
	// hyphen of a hyphenated line, hang into the margin: it takes no width
	// there. Default false.
	HangingPunctuationEnd bool
	// OmitLastLeading leaves out the glue below the last line (with a
	// LineModel, its Leading there), so the paragraph ends at the last
	// line's depth. Default false.
	OmitLastLeading bool
	// HalfLeading distributes the leading (LineHeight minus the line's
	// natural Height+Depth) into the line box itself, half above and half
	// below (CSS 2.1 section 10.8.1), instead of emitting lineskip glue
	// after each line (the TeX-flavored default). Baseline-to-baseline
	// distances and the total paragraph height stay the same; only the
	// box edges and the first baseline (down by half the leading) move.
	// Lines taller than LineHeight keep their natural size. In this mode
	// OmitLastLeading has no meaning: half the leading sits in the last
	// line's depth by construction.
	HalfLeading bool
	// LineModel, when set, places each line in its line box and decides the
	// glue between lines, in place of HalfLeading and the lineskip glue
	// (see LineModel). Nil keeps the built-in behavior.
	LineModel LineModel
	// Breaker, when set, chooses where the paragraph breaks among its legal
	// breakpoints, in place of Knuth-Plass (see Breaker). Linebreak still
	// measures, packs and sets the lines. Nil keeps Knuth-Plass.
	Breaker Breaker
	// TextDirection is the paragraph's base direction. The insets and the
	// edge glues are physical whatever it says; what it decides is which
	// side is the line end: where a forced break leaves its slack, and
	// where hanging punctuation protrudes. Every line and the paragraph
	// box carry it as TextDir.
	TextDirection TextDirection
	// TabStops are the positions a tab (a Glue of subtype GlueTab) advances
	// to, measured from the paragraph's start edge: the left edge of a left
	// to right paragraph, the right edge of a right to left one. A tab moves
	// to the first stop past the text before it on the line; after the last
	// stop it keeps its own width. Without stops tabs are ordinary glue.
	TabStops []TabStop
}

// NewLinebreakSettings returns a settings struct with defaults initialized.
func NewLinebreakSettings() *LinebreakSettings {
	ls := &LinebreakSettings{
		DoublehyphenDemerits: 3000,
		DemeritsFitness:      100,
		Hyphenpenalty:        50,
		Tolerance:            4.0,
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

// Walk visits every node reachable from start, descending into container
// sub-lists: HList.List, VList.List, Disc.Pre/Post/Replace, and the list
// inside Glue.Leader. The visitor receives each node in source order and
// returns true to continue or false to abort the traversal. Walk itself
// returns false if the visitor aborted, true otherwise.
func Walk(start Node, fn func(Node) bool) bool {
	for e := start; e != nil; e = e.Next() {
		if !fn(e) {
			return false
		}
		switch v := e.(type) {
		case *HList:
			if !Walk(v.List, fn) {
				return false
			}
		case *VList:
			if !Walk(v.List, fn) {
				return false
			}
		case *Disc:
			if !Walk(v.Pre, fn) {
				return false
			}
			if !Walk(v.Post, fn) {
				return false
			}
			if !Walk(v.Replace, fn) {
				return false
			}
		case *Glue:
			if v.Leader != nil {
				if !Walk(v.Leader.List, fn) {
					return false
				}
			}
		}
	}
	return true
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

// cloneAttributes returns a shallow clone of an Attributes map, or nil if
// there is nothing to copy. Every Copy() method uses this so a copied node
// carries the same metadata (origin markers, hyperlink/dest info, structure
// tags) as its source — the Attributes field lives in the embedded basenode
// and would otherwise be silently dropped.
func cloneAttributes(a H) H {
	if a == nil {
		return nil
	}
	return maps.Clone(a)
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
		wd, ht, dp := e.Sizes(dir)
		sumwd += wd
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
		wd, ht, dp := e.Sizes(Horizontal)
		sumwd += wd
		if ht > maxht {
			maxht = ht
		}
		if dp > maxdp {
			maxdp = dp
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
		default:
			wd, ht, dp := e.Sizes(Horizontal)
			sumwd += wd
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
	switch {
	case width == sumwd:
		// an exact fit, no glue is stretched or shrunk
		r = 0
	case sumwd < width:
		// a short line
		r = float64(width-sumwd) / float64(stretchability)
	default:
		// a long line
		r = float64(width-sumwd) / float64(shrinkability)
	}
	badness := 10000
	if r < -1 {
		// Badness 1000000 for overfull boxes
		badness = 1000000
	} else if r >= -1 {
		badness = int(min(math.Round(math.Pow(math.Abs(r), 3)*100.0), 10000))
	}
	useExpand := false
	if hs.fontexpansion != 0 {
		if r < -1 {
			r = -1
			useExpand = true
		}
	}
	if math.IsInf(r, 0) {
		// Nothing can stretch or shrink, so as in TeX the glue keeps its
		// width. Inf × 0 is NaN, which converts to 0 on arm64 but to
		// math.MinInt64 on amd64.
		r = 0
	}
	for _, g := range glues {
		switch {
		case r >= 0 && highestOrderStretch == g.StretchOrder:
			g.Width += bag.ScaledPoint(r * float64(g.Stretch))
		case r >= -1 && r <= 0 && highestOrderShrink == g.ShrinkOrder:
			g.Width += bag.ScaledPoint(r * float64(g.Shrink))
		case r < -1 && highestOrderShrink == g.ShrinkOrder:
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
		// Clamp at the configured fontexpansion ceiling (microtype/hz
		// limit). Beyond that, glyphs shrink visibly and the cell looks
		// like it uses a smaller font size, which is exactly the
		// "Quanti-ty rendered narrower than Unit Cost" symptom. The
		// remaining overflow stays in the HList as box-overfull — much
		// better than secretly squeezing the glyph widths beyond the
		// hz limit.
		if a > hs.fontexpansion {
			a = hs.fontexpansion
		}
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
		wd, ht, dp := e.Sizes(Vertical)
		sumht += ht + dp
		if wd > maxwd {
			maxwd = wd
		}
		lastNode = e
	}
	vl := NewVList()
	vl.List = firstNode
	vl.Width = maxwd
	if lastNode != nil {
		_, _, lastDepth := lastNode.Sizes(Vertical)
		vl.Depth = lastDepth
		vl.Height = sumht - lastDepth
	}
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
