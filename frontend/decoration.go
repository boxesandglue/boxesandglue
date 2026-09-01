package frontend

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/boxesandglue/frontend/pdfdraw"
)

type styles struct {
	// decoration is the line currently open, or TextDecorationLineNone.
	decoration TextDecorationLine
	// style is how that line is drawn.
	style TextDecorationStyle
	// col overrides the text colour; nil means CSS's `currentColor`.
	col *color.Color
	// pos is the line's offset from the baseline, negative below it.
	pos       bag.ScaledPoint
	linewidth bag.ScaledPoint
}

// DecorationOffset returns where a text-decoration line sits relative to the
// baseline, as a fraction of the font size. The fractions match the underline
// offset this package has always used; no font exposes its post/OS-2
// underline or strikeout metrics here yet, so all three lines are derived the
// same way.
func DecorationOffset(line TextDecorationLine, fontsize bag.ScaledPoint) bag.ScaledPoint {
	switch line {
	case TextDecorationUnderline:
		return -fontsize / 6
	case TextDecorationLineThrough:
		// Roughly a quarter of the em above the baseline, which puts the line
		// through the middle of lowercase letters.
		return fontsize / 4
	case TextDecorationOverline:
		// Just clear of the ascender.
		return fontsize * 4 / 5
	}
	return 0
}

// lastInked walks back from stop to the last node that puts ink on the line, so
// a decoration still open at a line break stops at the text. The nodes at the
// end of a broken line are the break's own glue and the alignment filler, and
// including them drew the underline out to the margin: a wrapped underlined
// heading was underlined across the gap after its last word.
func lastInked(start, stop node.Node) node.Node {
	for e := stop; e != nil; e = e.Prev() {
		switch e.(type) {
		case *node.Glue, *node.Penalty:
			// Not ink; keep walking back, unless there is nothing before it.
			if e == start {
				return stop
			}
		default:
			return e
		}
	}
	return stop
}

func drawDecoration(head, start, stop node.Node, st *styles) node.Node {
	wd, _, _ := node.Dimensions(start, stop, node.Horizontal)
	pd := pdfdraw.NewStandalone().LineWidth(st.linewidth)
	if st.col != nil {
		// CSS text-decoration-color; without it the line inherits the text
		// colour from the graphics state, which is `currentColor`.
		pd = pd.ColorStroking(*st.col)
	}
	lw := st.linewidth
	switch st.style {
	case TextDecorationStyleDouble:
		// Two lines, the pair centred on where the single line would sit.
		gap := lw * 2
		pd = pd.Moveto(0, st.pos+gap).Lineto(wd, st.pos+gap).
			Moveto(0, st.pos-gap).Lineto(wd, st.pos-gap)
	case TextDecorationStyleDotted:
		// SetDash takes whole PDF units, so the pattern cannot be finer than a
		// point however thin the line is.
		u := dashUnit(lw)
		pd = pd.SetDash([]uint{u, 2 * u}, 0).Moveto(0, st.pos).Lineto(wd, st.pos)
	case TextDecorationStyleDashed:
		u := dashUnit(lw)
		pd = pd.SetDash([]uint{3 * u, 2 * u}, 0).Moveto(0, st.pos).Lineto(wd, st.pos)
	case TextDecorationStyleWavy:
		pd = wave(pd, wd, st.pos, lw)
	default:
		pd = pd.Moveto(0, st.pos).Lineto(wd, st.pos)
	}
	r := node.NewRule()
	r.Hide = true
	r.Pre = pd.Stroke().String()
	head = node.InsertBefore(head, start, r)
	return head
}

// dashUnit scales the dash pattern with the line width, with a floor of one
// PDF unit because SetDash takes integers.
func dashUnit(lw bag.ScaledPoint) uint {
	u := uint(lw.ToPT() * 2)
	if u < 1 {
		u = 1
	}
	return u
}

// wave draws the wavy decoration as a run of half-period Bézier arcs,
// alternating above and below the line position.
func wave(pd *pdfdraw.Object, wd, pos, lw bag.ScaledPoint) *pdfdraw.Object {
	period := lw * 6
	if period <= 0 {
		return pd.Moveto(0, pos).Lineto(wd, pos)
	}
	amp := lw * 2
	pd = pd.Moveto(0, pos)
	up := true
	for x := bag.ScaledPoint(0); x < wd; x += period {
		next := x + period
		if next > wd {
			next = wd
		}
		y := pos + amp
		if !up {
			y = pos - amp
		}
		// One half period: rise to the crest and return to the line.
		pd = pd.Curveto(x+(next-x)/3, y, x+2*(next-x)/3, y, next, pos)
		up = !up
	}
	return pd
}

func postLinebreakHL(n node.Node, st *styles) node.Node {
	decoratedFromStart := st.decoration != TextDecorationLineNone
	var decorationStart, decorationStop node.Node
	var head, tail node.Node
	head = n
	for e := n; e != nil; e = e.Next() {
		tail = e
		if hl, ok := e.(*node.HList); ok {
			hl.List = postLinebreakHL(hl.List, st)
		} else if vl, ok := e.(*node.VList); ok {
			vl.List = postLinebreakHL(vl.List, st)
		} else if ss, ok := e.(*node.StartStop); ok {
			if val, ok := ss.GetAttribute("decoration"); ok {
				if line, _ := val.(TextDecorationLine); line != TextDecorationLineNone {
					st.decoration = line
					decorationStart = ss
					if pos, ok := ss.GetAttribute("decorationpos"); ok {
						if pos != nil {
							st.pos = pos.(bag.ScaledPoint)
						}
					}
					if lw, ok := ss.GetAttribute("decorationlw"); ok {
						if lw != nil {
							st.linewidth = lw.(bag.ScaledPoint)
						}
					}
					st.style = TextDecorationStyleSolid
					if v, ok := ss.GetAttribute("decorationstyle"); ok {
						if style, ok := v.(TextDecorationStyle); ok {
							st.style = style
						}
					}
					st.col = nil
					if v, ok := ss.GetAttribute("decorationcolor"); ok {
						if col, ok := v.(*color.Color); ok {
							st.col = col
						}
					}
				} else {
					if decoratedFromStart {
						head = drawDecoration(head, head, e, st)
					}
					st.decoration = TextDecorationLineNone
					decorationStop = ss
				}
			}
		}
		if decorationStart != nil && decorationStop != nil {
			head = drawDecoration(head, decorationStart, decorationStop, st)
			decorationStart = nil
			decorationStop = nil
		}
	}
	if st.decoration != TextDecorationLineNone {
		if decorationStart == nil && decorationStop == nil {
			// whole line
			head = drawDecoration(head, head, lastInked(head, tail), st)
		}
		if decorationStart != nil {
			// up to the end
			head = drawDecoration(head, decorationStart, lastInked(decorationStart, tail), st)
		}
	}
	return head
}

func postLinebreak(vl *node.VList) *node.VList {
	var e node.Node
	for e = vl.List; e != nil; e = e.Next() {
		if hl, ok := e.(*node.HList); ok {
			postLinebreakHL(hl, &styles{})
		}
	}
	return vl
}
