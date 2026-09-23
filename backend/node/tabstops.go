package node

import (
	"cmp"
	"slices"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// TabAlign is how the text after a tab lines up with the stop.
type TabAlign int

const (
	// TabAlignLeft starts the text at the stop.
	TabAlignLeft TabAlign = iota
	// TabAlignRight ends the text at the stop.
	TabAlignRight
	// TabAlignCenter centers the text on the stop.
	TabAlignCenter
	// TabAlignDecimal puts the stop at the text's first decimal separator,
	// or ends the text there if it has none.
	TabAlignDecimal
)

// TabStop is a position a tab advances to, see LinebreakSettings.TabStops.
type TabStop struct {
	// Position is the distance of the stop from the paragraph's start edge.
	Position bag.ScaledPoint
	// Align is how the text after the tab lines up with the stop. The text
	// up to the next tab, forced break or paragraph end has no legal
	// breakpoints after a right, center or decimal stop if it fits on the
	// line. Where it is too wide to line up and still leave the tab its own
	// width, the tab keeps that width and the text starts after it.
	Align TabAlign
	// Separator is the decimal separator a TabAlignDecimal stop aligns on;
	// empty means ".".
	Separator string
	// Leader is a pattern repeated across the tab, like the pattern of a
	// leader glue. Nil leaves the tab blank.
	Leader *HList
}

// lineOrigin is the point a candidate line is measured from in the first
// pass: the position x reached on the line, relative to where the line's
// content starts, and the running sums of the breaker at that point. Without
// a tab stop on the line that is the line's start breakpoint with x = 0.
// Past a stop it is the tab: the text after it starts at the stop whatever
// came before, and only its own glue stretches or shrinks.
type lineOrigin struct {
	x bag.ScaledPoint
	lineSums
	// aligned is set when the tab reached a right, center or decimal stop.
	// The rest of the line after its run is empty, as after a forced break.
	aligned bool
}

// tabMark is a tab of the paragraph with the running sums of the first pass
// before and after it.
type tabMark struct {
	wBefore bag.ScaledPoint
	after   lineSums
	run     *tabRun
}

// tabRun is the text after a tab up to the next tab, forced break or the end
// of the paragraph: the nodes from first up to (not including) end.
type tabRun struct {
	first, end Node
	width      bag.ScaledPoint
	// keep is set when the run has no legal breakpoints, because the tab
	// reaches a right, center or decimal stop.
	keep    bool
	decimal map[string]bag.ScaledPoint
	// fromSep caches, per separator, the running sums from the start of the
	// run up to the separator, where a run that is not kept starts to
	// justify.
	fromSep map[string]*sepSums
}

type sepSums struct {
	found bool
	sums  lineSums
}

func stopSeparator(s *TabStop) string {
	if s.Separator == "" {
		return "."
	}
	return s.Separator
}

// justifiesFrom returns the separator in the nodes from first up to end at
// which the text after a decimal tab starts to justify: the first glyph sep,
// unless the number is a left to right one in a right to left paragraph,
// which runs back towards the start edge past the separator. Nil if there
// is none.
func (lb *linebreaker) justifiesFrom(first, end Node, sep string) Node {
	for n := first; n != nil && n != end; n = n.Next() {
		if g, ok := n.(*Glyph); ok && g.Components == sep {
			if lb.settings.TextDirection == TextDirRTL && g.BidiLevel()%2 == 0 {
				return nil
			}
			return g
		}
	}
	return nil
}

// sumsTo adds the nodes from first up to (not including) end to the running
// sums, the way the first pass does.
func (lb *linebreaker) sumsTo(sums lineSums, first, end Node) lineSums {
	for n := first; n != nil && n != end; n = n.Next() {
		switch t := n.(type) {
		case *Glue:
			sums.sumW += t.Width
			sums.sumZ += t.Shrink
			switch t.StretchOrder {
			case StretchFil:
				sums.stretchFil += t.Stretch
			case StretchFill:
				sums.stretchFill += t.Stretch
			case StretchFilll:
				sums.stretchFilll += t.Stretch
			default:
				sums.sumY += t.Stretch
			}
		case *Glyph:
			sums.sumW += t.Width
			if lb.settings.FontExpansion != 0 {
				sums.sumExpand += bag.MultiplyFloat(t.Width, lb.settings.FontExpansion)
			}
		case *Penalty, *Disc, *HardBreak:
		default:
			w, _, _ := n.Sizes(Horizontal)
			sums.sumW += w
		}
	}
	return sums
}

func sortedTabStops(stops []TabStop) []TabStop {
	return slices.SortedStableFunc(slices.Values(stops), func(a, b TabStop) int {
		return cmp.Compare(a.Position, b.Position)
	})
}

func (lb *linebreaker) isTab(n Node) bool {
	if len(lb.stops) == 0 {
		return false
	}
	g, ok := n.(*Glue)
	return ok && g.Subtype == GlueTab
}

// startInset is the inset of the row at the edge the stops are measured
// from, which is the right edge in a right to left paragraph, as for
// SettingIndentStart.
func (lb *linebreaker) startInset(row int) bag.ScaledPoint {
	if lb.settings.TextDirection == TextDirRTL {
		return lb.getIndentRight(row)
	}
	return lb.getIndent(row)
}

// nextStop returns the stop a tab at x reaches, x and the returned target
// both measured from the start of the line's content in a row with the given
// start inset. The target is where the tab ends, before the stop by off: how
// much of the text after the tab the stop's alignment puts before it. At a
// right, center or decimal stop the tab is at least its own width wide, tabW,
// the width it has once the stops run out. Text that has reached or passed a
// stop goes on to the next; ok is false when the stops have run out.
func (lb *linebreaker) nextStop(inset, x, tabW bag.ScaledPoint, off func(*TabStop) bag.ScaledPoint) (target bag.ScaledPoint, stop *TabStop, ok bool) {
	for i := range lb.stops {
		s := &lb.stops[i]
		if pos := s.Position - inset; pos > x {
			if s.Align == TabAlignLeft {
				return pos, s, true
			}
			return max(x+tabW, pos-off(s)), s, true
		}
	}
	return 0, nil, false
}

// alignOffset is how much of the text after a tab, w wide, the stop s puts
// before itself.
func alignOffset(s *TabStop, w bag.ScaledPoint, decimal func(sep string) bag.ScaledPoint) bag.ScaledPoint {
	switch s.Align {
	case TabAlignRight:
		return w
	case TabAlignCenter:
		return w / 2
	case TabAlignDecimal:
		return min(decimal(stopSeparator(s)), w)
	}
	return 0
}

// toSeparator is the width of the nodes from first up to end that a decimal
// stop puts before itself: those before the first glyph sep, or all of them
// if there is none. A left to right number in a right to left paragraph
// runs away from the start edge in the other direction, so there it is the
// width after the separator.
func (lb *linebreaker) toSeparator(first, end Node, sep string) bag.ScaledPoint {
	var w bag.ScaledPoint
	for n := first; n != nil && n != end; n = n.Next() {
		if g, ok := n.(*Glyph); ok && g.Components == sep {
			if lb.settings.TextDirection != TextDirRTL || g.BidiLevel()%2 == 1 {
				return w
			}
			w = 0
			for m := n.Next(); m != nil && m != end; m = m.Next() {
				mw, _, _ := m.Sizes(Horizontal)
				w += mw
			}
			return w
		}
		nw, _, _ := n.Sizes(Horizontal)
		w += nw
	}
	return w
}

// measureTabRuns is the pre-pass over the paragraph: it measures the text
// after each tab and decides which runs are kept together. That depends on
// the stop a tab reaches, and so on where its line starts, which is not
// known before the break; it is decided for the line as it would be set
// without soft breaks, from the paragraph start or the last forced break. A
// run that would not fit on the line even so stays breakable.
func (lb *linebreaker) measureTabRuns(head Node) map[*Glue]*tabRun {
	runs := map[*Glue]*tabRun{}
	row := 0
	var x bag.ScaledPoint
	for n := head; n != nil; n = n.Next() {
		if isForcedBreak(n) {
			row++
			x = 0
			continue
		}
		g, ok := n.(*Glue)
		if !ok || !lb.isTab(g) {
			w, _, _ := n.Sizes(Horizontal)
			x += w
			continue
		}
		r := &tabRun{first: g.Next()}
		for e := r.first; e != nil; e = e.Next() {
			if isForcedBreak(e) || lb.isTab(e) {
				r.end = e
				break
			}
			w, _, _ := e.Sizes(Horizontal)
			r.width += w
		}
		runs[g] = r
		if target, stop, ok := lb.nextStop(lb.startInset(row), x, g.Width, lb.offset(r, r.width)); ok {
			x = target
			measure := lb.settings.HSize - lb.getIndent(row) - lb.getIndentRight(row)
			r.keep = stop.Align != TabAlignLeft && target+r.width <= measure
		} else {
			x += g.Width
		}
	}
	return runs
}

// offset returns the alignment offset function for the first w of the run.
func (lb *linebreaker) offset(r *tabRun, w bag.ScaledPoint) func(*TabStop) bag.ScaledPoint {
	return func(s *TabStop) bag.ScaledPoint {
		return alignOffset(s, w, func(sep string) bag.ScaledPoint {
			d, ok := r.decimal[sep]
			if !ok {
				d = lb.toSeparator(r.first, r.end, sep)
				if r.decimal == nil {
					r.decimal = map[string]bag.ScaledPoint{}
				}
				r.decimal[sep] = d
			}
			return d
		})
	}
}

// origin is the lineOrigin of the candidate line starting at a and ending at
// the current position, where the line is curW wide in the running sum, or
// nil if no tab on the line reaches a stop and the line is measured from a as
// usual. The result is only valid until the next call.
func (lb *linebreaker) origin(a *Breakpoint, curW bag.ScaledPoint) *lineOrigin {
	o := &lb.org
	*o = lineOrigin{lineSums: lineSums{sumW: a.sumW}}
	ok := false
	inset := lb.startInset(a.Line)
	first := lb.firstTab[a.Position]
	for i, t := range lb.tabs[first:] {
		// The last tab's run may go on past the end of this line.
		runW := t.run.width
		if first+i == len(lb.tabs)-1 {
			runW = min(runW, curW-t.after.sumW)
		}
		target, stop, reached := lb.nextStop(inset, o.x+t.wBefore-o.sumW, t.after.sumW-t.wBefore, lb.offset(t.run, runW))
		if !reached {
			continue
		}
		*o = lineOrigin{x: target, lineSums: t.after, aligned: stop.Align != TabAlignLeft}
		if stop.Align == TabAlignDecimal && !t.run.keep {
			// A figure too wide to keep together: from its separator on, the
			// line is measured and justified as after a left stop.
			if ss := lb.sepSumsOf(t, stopSeparator(stop)); ss.found && ss.sums.sumW-t.after.sumW < runW {
				*o = lineOrigin{x: target + ss.sums.sumW - t.after.sumW, lineSums: ss.sums}
				ok = true
				continue
			}
		}
		if o.aligned {
			// The run is placed by its natural width, so its glue does not
			// stretch or shrink either, except for the paragraph's fill at
			// its end.
			o.sumY, o.sumZ, o.sumExpand = lb.sumY, lb.sumZ, lb.sumExpand
		}
		ok = true
	}
	if !ok {
		return nil
	}
	return o
}

// sepSumsOf returns the running sums at the separator of the run after the
// tab t, see justifiesFrom.
func (lb *linebreaker) sepSumsOf(t tabMark, sep string) *sepSums {
	r := t.run
	if ss, ok := r.fromSep[sep]; ok {
		return ss
	}
	ss := &sepSums{}
	if g := lb.justifiesFrom(r.first, r.end, sep); g != nil {
		ss.found = true
		ss.sums = lb.sumsTo(t.after, r.first, g)
	}
	if r.fromSep == nil {
		r.fromSep = map[string]*sepSums{}
	}
	r.fromSep[sep] = ss
	return ss
}

// markFirstTab records the first tab on a line that starts at a break at n.
// A tab the line breaks at is dropped with the break.
func (lb *linebreaker) markFirstTab(n Node) {
	if lb.firstTab == nil {
		return
	}
	i := len(lb.tabs)
	if lb.isTab(n) {
		i++
	}
	lb.firstTab[n] = i
}

// setTabs gives each tab on the line from start up to (not including) end
// the width that takes the text after it to its stop, and reports whether
// any tab reached one, and whether the last one to do so is a right, center
// or decimal stop. The glue before the last stop reached loses its stretch
// and shrink: text before a stop is set at its natural width, which is how
// the first pass measured it. So does the finite glue in the text after a
// right, center or decimal stop, up to the separator where a decimal figure
// too wide to keep together starts to justify.
func (lb *linebreaker) setTabs(start, end Node, row int) (tabbed, aligned bool) {
	inset := lb.startInset(row)
	var x bag.ScaledPoint
	// keepEnd is where the run after the last stop reached ends, or where it
	// starts to justify, if that stop is not a left one.
	var last, keepEnd Node
	for n := start; n != nil && n != end; n = n.Next() {
		g, ok := n.(*Glue)
		if ok && g.Subtype == GlueTab {
			first := g.Next()
			runEnd := end
			var runW bag.ScaledPoint
			for e := first; e != nil && e != end; e = e.Next() {
				if eg, ok := e.(*Glue); ok && eg.Subtype == GlueTab {
					runEnd = e
					break
				}
				w, _, _ := e.Sizes(Horizontal)
				runW += w
			}
			off := func(s *TabStop) bag.ScaledPoint {
				return alignOffset(s, runW, func(sep string) bag.ScaledPoint {
					return lb.toSeparator(first, runEnd, sep)
				})
			}
			if target, stop, ok := lb.nextStop(inset, x, g.Width, off); ok {
				g.Width = target - x
				g.Stretch, g.Shrink = 0, 0
				if stop.Leader != nil {
					g.Leader = stop.Leader
					g.LeaderType = LeaderAligned
				}
				last = g
				keepEnd, aligned = nil, false
				if stop.Align != TabAlignLeft {
					keepEnd, aligned = runEnd, true
				}
				if stop.Align == TabAlignDecimal && !lb.runs[g].keep {
					if sep := lb.justifiesFrom(first, runEnd, stopSeparator(stop)); sep != nil {
						keepEnd, aligned = sep, false
					}
				}
			}
		}
		w, _, _ := n.Sizes(Horizontal)
		x += w
	}
	if last == nil {
		return false, false
	}
	for n := start; n != last; n = n.Next() {
		if g, ok := n.(*Glue); ok {
			g.Stretch, g.Shrink = 0, 0
		}
	}
	for n := last; n != nil && keepEnd != nil && n != keepEnd; n = n.Next() {
		if g, ok := n.(*Glue); ok && g.StretchOrder == StretchNormal {
			g.Stretch, g.Shrink = 0, 0
		}
	}
	return true, aligned
}

// setFromStart moves the stretch of the edge glue at the line's start edge
// to the one at its end, so that a line with a tab stop starts at the start
// edge whatever the alignment and the stops keep their positions. With fill
// set the end takes the slack of the line as after a forced break.
func (lb *linebreaker) setFromStart(leftskip, lineEnd *Glue, fill bool) {
	start, end := leftskip, lineEnd
	if lb.settings.TextDirection == TextDirRTL {
		start, end = lineEnd, leftskip
	}
	if fill && end.StretchOrder < StretchFil {
		end.Stretch, end.StretchOrder = bag.Factor, StretchFill
	}
	if start.Stretch == 0 {
		return
	}
	if end.Stretch == 0 || end.StretchOrder < start.StretchOrder {
		end.Stretch, end.StretchOrder = start.Stretch, start.StretchOrder
	}
	start.Stretch = 0
}
