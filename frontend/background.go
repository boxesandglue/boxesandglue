package frontend

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/boxesandglue/frontend/pdfdraw"
)

// attrInlineBackground marks the StartStop pair Mknodes puts around a child
// Text with its own background colour. The opener carries the *color.Color,
// the closer carries nil and points back to the opener via StartNode.
const attrInlineBackground = "inlinebackground"

// inlineBackground is one background box that is open while walking a line.
type inlineBackground struct {
	col *color.Color
	// start is the opener on the current line, or nil when the box was opened
	// on an earlier line and continues from the start of this one.
	start node.Node
}

// backgroundColorOf resolves a SettingBackgroundColor value, which is either
// a colour name (CSS keyword or a colour defined on the document) or a
// *color.Color. Anything else, including an unknown name, yields nil.
func (fe *Document) backgroundColorOf(v any) *color.Color {
	switch t := v.(type) {
	case *color.Color:
		return t
	case string:
		return fe.GetColor(t)
	}
	return nil
}

// sameColor reports whether two colours paint the same. Inherited settings
// share the pointer, an explicitly repeated colour compares equal by value.
func sameColor(a, b *color.Color) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// firstInked walks forward from start to the first node that puts ink on the
// line, so a background continued from the previous line does not start at the
// margin: a centred or right-aligned line begins with the alignment glue.
func firstInked(start, stop node.Node) node.Node {
	for e := start; e != nil; e = e.Next() {
		switch e.(type) {
		case *node.Glue, *node.Penalty:
			if e == stop {
				return start
			}
		default:
			return e
		}
	}
	return start
}

// bgSegment is one filled rectangle of an inline background: a run of nodes
// whose glyphs share the same font box.
type bgSegment struct {
	start, stop node.Node
	wd, ht, dp  bag.ScaledPoint
	// hasGlyph is false while the box is still the fallback taken from the
	// dimensions of non-glyph nodes.
	hasGlyph bool
}

// drawBackground fills the box behind the nodes from start to stop with col
// and returns the list head, which changes when start is the head. The box is
// the CSS content area: the font's ascender above and descender below the
// baseline, not the ink of the glyphs, so "ace" and "Alpha" in the same span
// get the same box. Glyphs raised or lowered by vertical-align carry the box
// with them. Where the font box changes within the run, a nested Text in a
// larger size say, the box is split so that each part fits its own text; the
// same text therefore gets the same box on every line it lands on. Spaces,
// kerns and markers belong to the part before them, or to the first part when
// nothing precedes them. A run without any glyph, an inline image say, falls
// back to the dimensions of its nodes.
func drawBackground(head, start, stop node.Node, col *color.Color) node.Node {
	var segs []*bgSegment
	var cur *bgSegment
	for e := start; e != nil; e = e.Next() {
		wd, ht, dp := e.Sizes(node.Horizontal)
		if g, ok := e.(*node.Glyph); ok && g.Font != nil {
			ht = g.YOffset + g.Font.Size - g.Font.Depth
			dp = g.Font.Depth - g.YOffset
			switch {
			case cur == nil:
				cur = &bgSegment{start: e}
				segs = append(segs, cur)
			case !cur.hasGlyph:
				// The leading non-glyph nodes join this glyph's part.
			case cur.ht != ht || cur.dp != dp:
				cur = &bgSegment{start: e}
				segs = append(segs, cur)
			}
			cur.hasGlyph = true
			cur.ht, cur.dp = ht, dp
		} else {
			if cur == nil {
				cur = &bgSegment{start: e}
				segs = append(segs, cur)
			}
			if !cur.hasGlyph {
				cur.ht = max(cur.ht, ht)
				cur.dp = max(cur.dp, dp)
			}
		}
		cur.stop = e
		cur.wd += wd
		if e == stop {
			break
		}
	}
	for _, seg := range segs {
		if seg.wd <= 0 || seg.ht+seg.dp <= 0 {
			continue
		}
		pd := pdfdraw.NewStandalone().ColorNonstroking(*col).Rect(0, -seg.dp, seg.wd, seg.ht+seg.dp).Fill()
		r := node.NewRule()
		r.Hide = true
		r.Pre = pd.String()
		r.Attributes = node.H{"origin": "inline background"}
		head = node.InsertBefore(head, seg.start, r)
	}
	return head
}

// postLinebreakBackground paints the inline backgrounds of a paragraph. Boxes
// open at the end of a line continue on the next one, so the stack of open
// boxes lives in one styles value shared by all lines of the paragraph.
func postLinebreakBackground(vl *node.VList) {
	st := &styles{}
	for e := vl.List; e != nil; e = e.Next() {
		if hl, ok := e.(*node.HList); ok {
			hl.List = postLinebreakBackgroundLine(hl.List, st)
		}
	}
}

// postLinebreakBackgroundLine paints the inline backgrounds on one line, the
// node list n, and returns its new head. Inner boxes are drawn after outer
// ones and therefore on top of them. A box nested in the line (an inline
// block, a leader pattern) is a line of its own with its own stack: a box
// open around it is drawn across it, never into it.
func postLinebreakBackgroundLine(n node.Node, st *styles) node.Node {
	head := n
	var tail node.Node
	for e := n; e != nil; e = e.Next() {
		tail = e
		switch t := e.(type) {
		case *node.HList:
			t.List = postLinebreakBackgroundLine(t.List, &styles{})
		case *node.VList:
			postLinebreakBackground(t)
		case *node.StartStop:
			v, ok := t.GetAttribute(attrInlineBackground)
			if !ok {
				continue
			}
			if col, ok := v.(*color.Color); ok && col != nil {
				st.backgrounds = append(st.backgrounds, &inlineBackground{col: col, start: t})
				continue
			}
			// The closer. Boxes are properly nested, so it closes the
			// innermost open one.
			if len(st.backgrounds) == 0 {
				continue
			}
			bg := st.backgrounds[len(st.backgrounds)-1]
			st.backgrounds = st.backgrounds[:len(st.backgrounds)-1]
			start := bg.start
			if start == nil {
				start = firstInked(head, t)
			}
			head = drawBackground(head, start, t, bg.col)
		}
	}
	// Whatever is still open runs to the end of the line and continues on the
	// next one. Outer boxes first, so that inner ones are painted on top.
	for _, bg := range st.backgrounds {
		start := bg.start
		if start == nil {
			start = firstInked(head, tail)
		}
		head = drawBackground(head, start, lastInked(start, tail), bg.col)
		bg.start = nil
	}
	return head
}
