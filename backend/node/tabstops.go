package node

import (
	"cmp"
	"slices"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// TabStop is a position a tab advances to, see LinebreakSettings.TabStops.
type TabStop struct {
	// Position is the distance of the stop from the paragraph's start edge.
	Position bag.ScaledPoint
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
}

// tabMark is a tab of the paragraph with the running sums of the first pass
// before and after it.
type tabMark struct {
	wBefore bag.ScaledPoint
	after   lineSums
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
// start inset. Text that has reached or passed a stop goes on to the next;
// ok is false when the stops have run out.
func (lb *linebreaker) nextStop(inset, x bag.ScaledPoint) (target bag.ScaledPoint, stop *TabStop, ok bool) {
	for i := range lb.stops {
		s := &lb.stops[i]
		if t := s.Position - inset; t > x {
			return t, s, true
		}
	}
	return 0, nil, false
}

// origin is the lineOrigin of the candidate line starting at a and ending at
// the current position, or nil if no tab on the line reaches a stop and the
// line is measured from a as usual. The result is only valid until the next
// call.
func (lb *linebreaker) origin(a *Breakpoint) *lineOrigin {
	o := &lb.org
	*o = lineOrigin{lineSums: lineSums{sumW: a.sumW}}
	ok := false
	inset := lb.startInset(a.Line)
	for _, t := range lb.tabs[lb.firstTab[a.Position]:] {
		if target, _, reached := lb.nextStop(inset, o.x+t.wBefore-o.sumW); reached {
			*o = lineOrigin{x: target, lineSums: t.after}
			ok = true
		}
	}
	if !ok {
		return nil
	}
	return o
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
// any tab reached one. The glue before the last stop reached loses its
// stretch and shrink: text before a stop is set at its natural width, which
// is how the first pass measured it.
func (lb *linebreaker) setTabs(start, end Node, row int) bool {
	inset := lb.startInset(row)
	var x bag.ScaledPoint
	var last Node
	for n := start; n != nil && n != end; n = n.Next() {
		if g, ok := n.(*Glue); ok && g.Subtype == GlueTab {
			if target, stop, ok := lb.nextStop(inset, x); ok {
				g.Width = target - x
				g.Stretch, g.Shrink = 0, 0
				if stop.Leader != nil {
					g.Leader = stop.Leader
					g.LeaderType = LeaderAligned
				}
				last = g
			}
		}
		w, _, _ := n.Sizes(Horizontal)
		x += w
	}
	if last == nil {
		return false
	}
	for n := start; n != last; n = n.Next() {
		if g, ok := n.(*Glue); ok {
			g.Stretch, g.Shrink = 0, 0
		}
	}
	return true
}

// setFromStart moves the stretch of the edge glue at the line's start edge
// to the one at its end, so that a line with a tab stop starts at the start
// edge whatever the alignment and the stops keep their positions.
func (lb *linebreaker) setFromStart(leftskip, lineEnd *Glue) {
	start, end := leftskip, lineEnd
	if lb.settings.TextDirection == TextDirRTL {
		start, end = lineEnd, leftskip
	}
	if start.Stretch == 0 {
		return
	}
	if end.Stretch == 0 || end.StretchOrder < start.StretchOrder {
		end.Stretch, end.StretchOrder = start.Stretch, start.StretchOrder
	}
	start.Stretch = 0
}
