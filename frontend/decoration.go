package frontend

import (
	"slices"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/boxesandglue/frontend/pdfdraw"
)

type styles struct {
	// decorations are the decoration lines open at this point, in the order
	// they opened. Several can be open at once: an underlined and struck run,
	// or a struck span inside an underlined one.
	decorations []*openDecoration
	// backgrounds is the stack of inline background boxes open at this point
	// of the line, outermost first. See postLinebreakBackgroundLine.
	backgrounds []*inlineBackground
}

// openDecoration is a decoration line whose start marker the walk has passed
// but not its stop marker.
type openDecoration struct {
	// start is the marker that opened the line; its stop points back to it.
	start *node.StartStop
	// style is how the line is drawn.
	style TextDecorationStyle
	// col overrides the text colour; nil means CSS's `currentColor`.
	col *color.Color
	// pos is the line's offset from the baseline, negative below it.
	pos       bag.ScaledPoint
	linewidth bag.ScaledPoint
}

// newOpenDecoration reads the line a decoration start marker describes.
func newOpenDecoration(ss *node.StartStop) *openDecoration {
	d := &openDecoration{start: ss, style: TextDecorationStyleSolid}
	if v, ok := ss.GetAttribute("decorationpos"); ok {
		if pos, ok := v.(bag.ScaledPoint); ok {
			d.pos = pos
		}
	}
	if v, ok := ss.GetAttribute("decorationlw"); ok {
		if lw, ok := v.(bag.ScaledPoint); ok {
			d.linewidth = lw
		}
	}
	if v, ok := ss.GetAttribute("decorationstyle"); ok {
		if style, ok := v.(TextDecorationStyle); ok {
			d.style = style
		}
	}
	if v, ok := ss.GetAttribute("decorationcolor"); ok {
		if col, ok := v.(*color.Color); ok {
			d.col = col
		}
	}
	return d
}

// DecorationOffset returns where a text-decoration line sits relative to the
// baseline, as a fraction of the font size. The fractions match the underline
// offset this package has always used; no font exposes its post/OS-2
// underline or strikeout metrics here yet, so all three lines are derived the
// same way. line is a single line, not a set of them.
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

func drawDecoration(head, start, stop node.Node, st *openDecoration) node.Node {
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
	// from is where each line drawn in this list begins. A line already open
	// when the list starts begins at its head, recorded as nil because drawing
	// in front of the head moves it. A line that opens in a nested list and
	// is still open after it has no entry: the nested list drew it.
	from := make(map[*openDecoration]node.Node, len(st.decorations))
	for _, d := range st.decorations {
		from[d] = nil
	}
	var head, tail node.Node
	head = n
	for e := n; e != nil; e = e.Next() {
		tail = e
		switch t := e.(type) {
		case *node.HList:
			t.List = postLinebreakHL(t.List, st)
		case *node.VList:
			t.List = postLinebreakHL(t.List, st)
		case *node.StartStop:
			val, ok := t.GetAttribute("decoration")
			if !ok {
				continue
			}
			if line, _ := val.(TextDecorationLine); line != TextDecorationLineNone {
				d := newOpenDecoration(t)
				st.decorations = append(st.decorations, d)
				from[d] = t
				continue
			}
			i := st.openDecoration(t.StartNode)
			if i < 0 {
				continue
			}
			d := st.decorations[i]
			st.decorations = slices.Delete(st.decorations, i, i+1)
			if begin, ok := from[d]; ok {
				if begin == nil {
					begin = head
				}
				head = drawDecoration(head, begin, e, d)
			}
		}
	}
	// Lines still open run on into the next line; here they end at the text.
	for _, d := range st.decorations {
		begin, ok := from[d]
		if !ok {
			continue
		}
		if begin == nil {
			begin = head
		}
		head = drawDecoration(head, begin, lastInked(begin, tail), d)
	}
	return head
}

// openDecoration returns the index of the open line that start opened. A stop
// marker without a start closes the line opened last. -1 means none is open.
func (st *styles) openDecoration(start *node.StartStop) int {
	if start == nil {
		return len(st.decorations) - 1
	}
	for i, d := range st.decorations {
		if d.start == start {
			return i
		}
	}
	return -1
}

func postLinebreak(vl *node.VList) *node.VList {
	// Backgrounds first: their rules go in front of the decoration rules, so
	// an underline is painted on top of the background, not under it.
	postLinebreakBackground(vl)
	// One walk over all lines. postLinebreakHL follows the Next chain from
	// its argument, so starting it once at the first line covers the whole
	// paragraph; starting it again at every line drew the decorations of
	// the later lines once per preceding line, on top of each other.
	vl.List = postLinebreakHL(vl.List, &styles{})
	return vl
}
