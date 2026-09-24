package node

import (
	"slices"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

const (
	tabCharWidth = bag.ScaledPoint(6 * bag.Factor)
	// The width a tab has without stops, and after the last one.
	tabNatural = bag.ScaledPoint(12 * bag.Factor)
)

func newTab() *Glue {
	g := NewGlue()
	g.Subtype = GlueTab
	g.Width = tabNatural
	return g
}

// buildTabbed lays out lines separated by forced breaks. Within a line, "\t"
// is a tab and " " interword glue that stretches and shrinks.
func buildTabbed(lines ...string) Node {
	var head, cur Node
	for i, line := range lines {
		if i > 0 {
			hb := NewHardBreak()
			head = InsertAfter(head, cur, hb)
			cur = hb
		}
		for j, seg := range strings.Split(line, "\t") {
			if j > 0 {
				tab := newTab()
				head = InsertAfter(head, cur, tab)
				cur = tab
			}
			for k, w := range strings.Split(seg, " ") {
				if k > 0 {
					sp := NewGlue()
					sp.Width = 3 * bag.Factor
					sp.Stretch = 3 * bag.Factor
					sp.Shrink = bag.Factor
					head = InsertAfter(head, cur, sp)
					cur = sp
				}
				head, cur = glyphRun(head, cur, w, tabCharWidth)
			}
		}
	}
	head, _ = AppendLineEndAfter(head, cur)
	return head
}

func tabSettings(stops ...TabStop) *LinebreakSettings {
	s := NewLinebreakSettings()
	s.HSize = 200 * bag.Factor
	s.LineHeight = 12 * bag.Factor
	// ragged right, as the frontend sets up left aligned text
	s.LineEndGlue = NewGlue()
	s.LineEndGlue.Stretch = bag.Factor
	s.LineEndGlue.StretchOrder = StretchFill
	s.TabStops = stops
	return s
}

// checkRatios fails for each line but the last that was not set at the ratio
// the first pass broke it at, which is what shows the first pass measured the
// line the way the second pass sets it.
func checkRatios(t *testing.T, ls []*HList, bps []*Breakpoint) {
	t.Helper()
	for i, hl := range ls[:len(ls)-1] {
		set := hl.GlueSet
		if _, lineend := edgeGlues(hl); lineend != nil && lineend.StretchOrder != StretchNormal {
			set = 0 // the fill at the line end takes the slack
		}
		if d := bps[i].R - set; d > 1e-9 || d < -1e-9 {
			t.Errorf("line %d: broken at ratio %.3f, set at %.3f", i, bps[i].R, set)
		}
	}
}

func lines(vlist *VList) []*HList {
	var out []*HList
	for n := vlist.List; n != nil; n = n.Next() {
		if hl, ok := n.(*HList); ok {
			out = append(out, hl)
		}
	}
	return out
}

// afterTabs returns where the text after each tab on the line starts,
// measured from the line's start edge: the left edge, or in a right to left
// line, whose content the frontend reverses after the break, the right edge.
//
// The positions are summed from the packed nodes, whose glue HpackTo has set
// to its final width.
func afterTabs(hl *HList) []bag.ScaledPoint {
	var out []bag.ScaledPoint
	var x bag.ScaledPoint
	for n := hl.List; n != nil; n = n.Next() {
		w, _, _ := n.Sizes(Horizontal)
		x += w
		if g, ok := n.(*Glue); ok && g.Subtype == GlueTab {
			out = append(out, x)
		}
	}
	if hl.TextDir == TextDirRTL {
		leftskip, lineend := edgeGlues(hl)
		for i := range out {
			out[i] += lineend.Width - leftskip.Width
		}
	}
	return out
}

// Label and value lines: whatever the width of the label, the value starts at
// the stop.
func TestTabStopAlignsValues(t *testing.T) {
	stop := bag.MustSP("40mm")
	vlist, _ := Linebreak(buildTabbed("ab\tvalue", "abcdefgh\tvalue", "abcd\tvalue"), tabSettings(TabStop{Position: stop}))
	ls := lines(vlist)
	if len(ls) != 3 {
		t.Fatalf("got %d lines, want 3", len(ls))
	}
	for i, hl := range ls {
		if got := afterTabs(hl); len(got) != 1 || got[0] != stop {
			t.Errorf("line %d: value starts at %v, want [%s]", i, got, stop)
		}
	}
}

// A tab after text that has reached a stop goes on to the next stop.
func TestTabStopPassedGoesToNext(t *testing.T) {
	first, second := bag.ScaledPoint(20*bag.Factor), bag.ScaledPoint(60*bag.Factor)
	vlist, _ := Linebreak(buildTabbed("ab\tx", "abcdef\tx"), tabSettings(TabStop{Position: first}, TabStop{Position: second}))
	ls := lines(vlist)
	if len(ls) != 2 {
		t.Fatalf("got %d lines, want 2", len(ls))
	}
	// "abcdef" is 36pt, past the first stop.
	for i, want := range []bag.ScaledPoint{first, second} {
		if got := afterTabs(ls[i]); len(got) != 1 || got[0] != want {
			t.Errorf("line %d: text after the tab starts at %v, want [%s]", i, got, want)
		}
	}
}

// When the stops run out a tab keeps its own width.
func TestTabStopsRunOut(t *testing.T) {
	stop := bag.ScaledPoint(20 * bag.Factor)
	vlist, _ := Linebreak(buildTabbed("a\tbcd\te"), tabSettings(TabStop{Position: stop}))
	got := afterTabs(lines(vlist)[0])
	want := []bag.ScaledPoint{stop, stop + 3*tabCharWidth + tabNatural}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("text after the tabs starts at %v, want %v", got, want)
	}
}

// Stops are measured from the paragraph's start edge, not from where an
// indented row starts: a hanging indent does not move them.
func TestTabStopWithHangingIndent(t *testing.T) {
	stop := bag.ScaledPoint(50 * bag.Factor)
	s := tabSettings(TabStop{Position: stop})
	s.Indent = 30 * bag.Factor
	s.IndentRows = -1 // all rows but the first
	vlist, _ := Linebreak(buildTabbed("ab\tx", "ab\tx"), s)
	ls := lines(vlist)
	if len(ls) != 2 {
		t.Fatalf("got %d lines, want 2", len(ls))
	}
	for i, hl := range ls {
		if got := afterTabs(hl); len(got) != 1 || got[0] != stop {
			t.Errorf("row %d: text after the tab starts at %v, want [%s]", i, got, stop)
		}
	}
	// The second row is indented, so its label ends further in; the tab there
	// is narrower by the indent.
	if leftskip, _ := edgeGlues(ls[1]); leftskip.Width != s.Indent {
		t.Fatalf("second row not indented (leftskip %s); the test proves nothing", leftskip.Width)
	}

	// The same on a row the paragraph wraps into, where the first pass has to
	// know the indent of the row to measure the line.
	s.HSize = 150 * bag.Factor
	s.LineEndGlue = NewGlue()
	stop = 100 * bag.Factor
	s.TabStops = []TabStop{{Position: stop}}
	vlist, bps := Linebreak(buildTabbed("aaaa bbbb cccc dddd eeee ffff gg\thh ii jj kk ll mm nn oo pp"), s)
	ls = lines(vlist)
	if len(ls) != 3 {
		t.Fatalf("wrapped: got %d lines, want 3", len(ls))
	}
	if got := afterTabs(ls[1]); len(got) != 1 || got[0] != stop {
		t.Errorf("wrapped: text after the tab on the second row starts at %v, want [%s]", got, stop)
	}
	checkRatios(t, ls, bps)
}

// A tab at the start of a line, after a forced break, stays on the line. Its
// first stop falls inside the row's indent, so it goes on to the next.
func TestTabStopAtLineStart(t *testing.T) {
	s := tabSettings(TabStop{Position: 25 * bag.Factor}, TabStop{Position: 60 * bag.Factor})
	s.HSize = 150 * bag.Factor
	s.LineEndGlue = NewGlue()
	s.Indent = 30 * bag.Factor
	s.IndentRows = -1
	vlist, bps := Linebreak(buildTabbed("aa", "\tbb cc dd ee ff gg hh ii jj kk ll mm nn"), s)
	ls := lines(vlist)
	if len(ls) != 3 {
		t.Fatalf("got %d lines, want 3", len(ls))
	}
	if got := afterTabs(ls[1]); len(got) != 1 || got[0] != 60*bag.Factor {
		t.Errorf("text after the tab starts at %v, want [60pt]", got)
	}
	checkRatios(t, ls, bps)
}

// In a right to left paragraph the stops are measured from the right edge,
// and the start inset there is the right one.
func TestTabStopRightToLeft(t *testing.T) {
	stop := bag.ScaledPoint(50 * bag.Factor)
	s := tabSettings(TabStop{Position: stop})
	s.TextDirection = TextDirRTL
	// start aligned: the slack goes to the left
	s.LineStartGlue, s.LineEndGlue = s.LineEndGlue, NewGlue()
	s.IndentRight = 10 * bag.Factor
	vlist, _ := Linebreak(buildTabbed("ab\tx", "abcd\tx"), s)
	for i, hl := range lines(vlist) {
		if got := afterTabs(hl); len(got) != 1 || got[0] != stop {
			t.Errorf("line %d: text after the tab starts %v from the right edge, want [%s]", i, got, stop)
		}
	}
}

// A line with a stop is set from its start edge whatever the alignment, or
// the stops would move with it.
func TestTabStopInCenteredParagraph(t *testing.T) {
	stop := bag.ScaledPoint(40 * bag.Factor)
	s := tabSettings(TabStop{Position: stop})
	s.LineStartGlue = s.LineEndGlue.Copy().(*Glue)
	vlist, _ := Linebreak(buildTabbed("ab\tx"), s)
	if got := afterTabs(lines(vlist)[0]); len(got) != 1 || got[0] != stop {
		t.Errorf("text after the tab starts at %v, want [%s]", got, stop)
	}
}

// A stop's leader fills the tab.
func TestTabStopLeader(t *testing.T) {
	var dot Node
	dot, _ = glyphRun(nil, nil, ".", 3*bag.Factor)
	pattern := Hpack(dot)
	stop := bag.ScaledPoint(40 * bag.Factor)
	vlist, _ := Linebreak(buildTabbed("ab\tx"), tabSettings(TabStop{Position: stop, Leader: pattern}))
	var tab *Glue
	for n := lines(vlist)[0].List; n != nil; n = n.Next() {
		if g, ok := n.(*Glue); ok && g.Subtype == GlueTab {
			tab = g
		}
	}
	if tab.Leader != pattern || tab.LeaderType != LeaderAligned {
		t.Errorf("tab leader %v (type %d), want the stop's pattern, aligned", tab.Leader, tab.LeaderType)
	}
	if want := stop - 2*tabCharWidth; tab.Width != want {
		t.Errorf("tab width %s, want %s", tab.Width, want)
	}
}

// Tabs inside a paragraph that breaks into several lines: the breaker has to
// measure a line with the width the tab will get, not its natural width, or
// it fills the line as if the tab were narrow and the line comes out
// overfull.
//
// Justified, so that a line stretches: the space before the tab must not, or
// the text after it misses the stop.
func TestTabStopInsideBrokenParagraph(t *testing.T) {
	stop := bag.ScaledPoint(150 * bag.Factor)
	s := tabSettings(TabStop{Position: stop})
	s.LineEndGlue = NewGlue()
	text := "a b\tccc ddd eee fff ggg hhh iii jjj kkk lll mmm nnn ooo ppp\tqqq rrr sss ttt uuu vvv www"
	vlist, bps := Linebreak(buildTabbed(text), s)
	ls := lines(vlist)
	if len(ls) < 3 {
		t.Fatalf("got %d lines, want at least 3", len(ls))
	}
	// The first pass counted the stretch after the stop only, and the tab at
	// the width it gets.
	checkRatios(t, ls, bps)
	for i, hl := range ls {
		var w bag.ScaledPoint
		for n := hl.List; n != nil; n = n.Next() {
			nw, _, _ := n.Sizes(Horizontal)
			w += nw
		}
		if w > s.HSize {
			t.Errorf("line %d is %s wide, overfull for the measure of %s", i, w, s.HSize)
		}
	}
	// The first tab reaches the stop with words after it on the same line.
	if got := afterTabs(ls[0]); len(got) != 1 || got[0] != stop {
		t.Fatalf("line 0: text after the tab starts at %v, want [%s]", got, stop)
	}
	// The second tab sits on a later line, after enough text to reach the stop.
	var found bool
	for _, hl := range ls[1:] {
		if got := afterTabs(hl); len(got) == 1 && got[0] == stop {
			found = true
		}
	}
	if !found {
		t.Error("the second tab does not reach the stop on its line")
	}
}

// A line that breaks at a tab drops it, like any glue it breaks at: the next
// line starts with the text after it, measured from the line start.
func TestTabStopBreakAtTab(t *testing.T) {
	s := tabSettings(TabStop{Position: 60 * bag.Factor})
	s.HSize = 100 * bag.Factor
	s.LineEndGlue = NewGlue()
	// "aaaa bbbb cccc" passes the stop, so the tab keeps its own width and
	// "dddd" no longer fits after it.
	vlist, bps := Linebreak(buildTabbed("aaaa bbbb cccc\tdddd eeee ffff\tgggg"), s)
	ls := lines(vlist)
	if len(ls) != 3 {
		t.Fatalf("got %d lines, want 3", len(ls))
	}
	if g, ok := ls[1].List.Next().(*Glyph); !ok || g.Components != "d" {
		t.Fatalf("second line does not start with the text after the tab")
	}
	checkRatios(t, ls, bps)
}

// A line that breaks at a discretionary right before a tab keeps the tab, as
// after any other break, and the first pass measured it with the tab.
func TestTabStopAfterDiscBreak(t *testing.T) {
	stop := bag.ScaledPoint(40 * bag.Factor)
	s := tabSettings(TabStop{Position: stop})
	s.HSize = 100 * bag.Factor
	s.LineEndGlue = NewGlue()
	head := buildTabbed("aaa bbb ccc eee\tff gg hh ii jj kk ll mm")
	for n := head; n != nil; n = n.Next() {
		if g, ok := n.(*Glue); ok && g.Subtype == GlueTab {
			d := NewDisc()
			d.Penalty = -5000 // break here
			h := NewGlyph()
			h.Components = "-"
			h.Width = 6 * bag.Factor
			d.Pre = h
			InsertBefore(head, g, d)
			break
		}
	}
	vlist, bps := Linebreak(head, s)
	ls := lines(vlist)
	if len(ls) != 3 {
		t.Fatalf("got %d lines, want 3", len(ls))
	}
	if got := afterTabs(ls[1]); len(got) != 1 || got[0] != stop {
		t.Fatalf("second line: text after the tab starts at %v, want [%s]", got, stop)
	}
	// Only the second line: the first ends in the discretionary, whose
	// hyphen the first pass counts and the second pass leaves out before
	// glue, tab or not.
	checkRatios(t, ls[1:], bps[1:])
}

// tabRuns returns, for each tab on the line, where the text after it starts
// and ends (the end of its last glyph before the next tab), measured from
// the left edge of the line.
func tabRuns(hl *HList) (starts, ends []bag.ScaledPoint) {
	var x bag.ScaledPoint
	in := false
	for n := hl.List; n != nil; n = n.Next() {
		w, _, _ := n.Sizes(Horizontal)
		x += w
		switch t := n.(type) {
		case *Glue:
			if t.Subtype == GlueTab {
				starts = append(starts, x)
				ends = append(ends, x)
				in = true
			}
		case *Glyph:
			if in {
				ends[len(ends)-1] = x
			}
		}
	}
	return starts, ends
}

// glyphAt returns where the first glyph c after the first tab starts.
func glyphAt(hl *HList, c string) (bag.ScaledPoint, bool) {
	var x bag.ScaledPoint
	seenTab := false
	for n := hl.List; n != nil; n = n.Next() {
		if g, ok := n.(*Glue); ok && g.Subtype == GlueTab {
			seenTab = true
		}
		if g, ok := n.(*Glyph); ok && seenTab && g.Components == c {
			return x, true
		}
		w, _, _ := n.Sizes(Horizontal)
		x += w
	}
	return 0, false
}

// A contents line: a right stop at the measure with a dot leader. The page
// numbers end at the right edge whatever their width, and the dots fill the
// space between title and number.
func TestTabStopRightWithLeader(t *testing.T) {
	var dot Node
	dot, _ = glyphRun(nil, nil, ".", 3*bag.Factor)
	pattern := Hpack(dot)
	s := tabSettings(TabStop{Position: 200 * bag.Factor, Align: TabAlignRight, Leader: pattern})
	vlist, _ := Linebreak(buildTabbed("Intro\t1", "A longer chapter title\t123"), s)
	ls := lines(vlist)
	if len(ls) != 2 {
		t.Fatalf("got %d lines, want 2", len(ls))
	}
	for i, hl := range ls {
		if _, ends := tabRuns(hl); len(ends) != 1 || ends[0] != s.HSize {
			t.Errorf("line %d: page number ends at %v, want [%s]", i, ends, s.HSize)
		}
		var tab *Glue
		for n := hl.List; n != nil; n = n.Next() {
			if g, ok := n.(*Glue); ok && g.Subtype == GlueTab {
				tab = g
			}
		}
		if tab.Leader != pattern {
			t.Errorf("line %d: no leader on the tab", i)
		}
	}
}

// A center stop centers the text after the tab on it.
func TestTabStopCenter(t *testing.T) {
	stop := bag.ScaledPoint(100 * bag.Factor)
	vlist, _ := Linebreak(buildTabbed("a\tbb", "a\tbbbbbb"), tabSettings(TabStop{Position: stop, Align: TabAlignCenter}))
	for i, hl := range lines(vlist) {
		starts, ends := tabRuns(hl)
		if len(starts) != 1 || starts[0]+ends[0] != 2*stop {
			t.Errorf("line %d: text after the tab from %v to %v, want it centered on %s", i, starts, ends, stop)
		}
	}
}

// A decimal stop lines figures up on their separator; a figure without one
// ends at the stop.
func TestTabStopDecimal(t *testing.T) {
	stop := bag.ScaledPoint(100 * bag.Factor)
	for _, sep := range []string{"", ","} {
		name := sep
		if name == "" {
			name = "default"
		}
		t.Run(name, func(t *testing.T) {
			c := sep
			if c == "" {
				c = "."
			}
			vlist, _ := Linebreak(buildTabbed("a\t1"+c+"5", "a\t1234"+c+"25", "a\t7"), tabSettings(TabStop{Position: stop, Align: TabAlignDecimal, Separator: sep}))
			ls := lines(vlist)
			if len(ls) != 3 {
				t.Fatalf("got %d lines, want 3", len(ls))
			}
			for i, hl := range ls[:2] {
				if x, ok := glyphAt(hl, c); !ok || x != stop {
					t.Errorf("line %d: separator at %s (found %v), want %s", i, x, ok, stop)
				}
			}
			if _, ends := tabRuns(ls[2]); len(ends) != 1 || ends[0] != stop {
				t.Errorf("line 2: figure without separator ends at %v, want [%s]", ends, stop)
			}
		})
	}
}

// The text after a right, center or decimal stop has no legal breakpoints,
// so a figure is not split even where that leaves the line overfull: not at
// glue, a penalty or a discretionary.
//
// Whether a run is kept together is decided before the break, for the line
// as it would be set without soft breaks, and only if the run fits then. The
// first line wraps here, so the second starts in the third row, which the
// right inset narrows: the kept figure no longer fits.
func TestTabStopKeepsAlignedRunTogether(t *testing.T) {
	breaks := map[string]func() Node{
		"glue": func() Node {
			g := NewGlue()
			g.Width, g.Stretch, g.Shrink = 3*bag.Factor, 3*bag.Factor, bag.Factor
			return g
		},
		"penalty": func() Node { return NewPenalty() },
		"disc": func() Node {
			d := NewDisc()
			d.Penalty = -1000 // preferred to the break at the tab
			d.Pre, _ = glyphRun(nil, nil, "-", tabCharWidth)
			return d
		},
	}
	for name, breakable := range breaks {
		t.Run(name, func(t *testing.T) {
			starts := func(align TabAlign) []string {
				s := tabSettings(TabStop{Position: 40 * bag.Factor, Align: align})
				s.HSize = 100 * bag.Factor
				s.IndentRight = 45 * bag.Factor
				s.IndentRightRows = -2
				head := buildTabbed("aaaa bbbb cccc dddd eeee ffff", "a\tbc.ddd")
				for n := head; n != nil; n = n.Next() {
					if g, ok := n.(*Glyph); ok && g.Components == "b" {
						if c, ok := g.Next().(*Glyph); !ok || c.Components != "c" {
							continue
						}
						InsertAfter(head, g, breakable())
						break
					}
				}
				vlist, _ := Linebreak(head, s)
				var out []string
				for _, hl := range lines(vlist) {
					for n := hl.List; n != nil; n = n.Next() {
						if g, ok := n.(*Glyph); ok {
							out = append(out, g.Components)
							break
						}
					}
				}
				return out
			}
			// With a left stop the run is breakable and does break.
			if got := starts(TabAlignLeft); !slices.Equal(got, []string{"a", "d", "a", "c"}) {
				t.Fatalf("left stop: line starts %v, want [a d a c]; the fixture does not break the run", got)
			}
			// The tab itself is still a legal breakpoint.
			if got := starts(TabAlignDecimal); slices.Contains(got, "c") {
				t.Errorf("decimal stop: line starts %v, the figure is split", got)
			}
		})
	}
}

// Figures at a decimal stop inside a paragraph that breaks: the breaker
// measures a line with the widths the tabs get, so no line is overfull and
// each break was chosen at the ratio the line is set with. The text after a
// figure runs on to the next tab and would not fit on a line, so it stays
// breakable; a line that ends in it is not justified but left ragged, as
// after a forced break.
func TestTabStopDecimalInsideBrokenParagraph(t *testing.T) {
	stop := bag.ScaledPoint(120 * bag.Factor)
	s := tabSettings(TabStop{Position: stop, Align: TabAlignDecimal})
	s.LineEndGlue = NewGlue()
	text := "aa bb cc\t12.5 dd ee ff gg hh ii jj kk ll\t1234.75 oo pp qq rr ss tt"
	vlist, bps := Linebreak(buildTabbed(text), s)
	ls := lines(vlist)
	if len(ls) < 3 {
		t.Fatalf("got %d lines, want at least 3", len(ls))
	}
	aligned := 0
	for i, hl := range ls {
		var w bag.ScaledPoint
		for n := hl.List; n != nil; n = n.Next() {
			nw, _, _ := n.Sizes(Horizontal)
			w += nw
		}
		if w > s.HSize {
			t.Errorf("line %d is %s wide, overfull for the measure of %s", i, w, s.HSize)
		}
		if x, ok := glyphAt(hl, "."); ok {
			if x != stop {
				t.Errorf("line %d: separator at %s, want %s", i, x, stop)
			}
			aligned++
		}
	}
	checkRatios(t, ls, bps)
	if aligned != 2 {
		t.Errorf("%d figures at the stop, want 2", aligned)
	}
}

// The text after a right stop that is too long to keep together stays
// breakable. A line that ends inside it gets its part of the text aligned to
// the stop, which is what the first pass measured the line with: justified
// or not, the rest of such a line is empty, so it was broken at ratio 0.
func TestTabStopRightRunTooLongToKeep(t *testing.T) {
	stop := bag.ScaledPoint(150 * bag.Factor)
	s := tabSettings(TabStop{Position: stop, Align: TabAlignRight})
	s.LineEndGlue = NewGlue()
	// Past "jj" the text would overshoot the stop, and "kkkkkkkkkkkk" is too
	// long to end the line with the tab at its own width instead.
	vlist, bps := Linebreak(buildTabbed("aa\tcc dd ee ff gg hh ii jj kkkkkkkkkkkk ll mm"), s)
	ls := lines(vlist)
	if len(ls) < 2 {
		t.Fatalf("got %d lines, want at least 2", len(ls))
	}
	if _, ends := tabRuns(ls[0]); len(ends) != 1 || ends[0] != stop {
		t.Errorf("line 0: text after the tab ends at %v, want [%s]", ends, stop)
	}
	for i := range ls[:len(ls)-1] {
		if bps[i].R != 0 {
			t.Errorf("line %d: broken at ratio %.3f, want 0: the line end takes the slack", i, bps[i].R)
		}
	}
}

// A figure at a decimal stop is set at its natural width even in a line
// that comes out overfull: its space does not shrink, so the separator stays
// on the stop. The line is overfull as in TestTabStopKeepsAlignedRunTogether,
// and here nothing on it is a legal breakpoint.
func TestTabStopAlignedRunDoesNotShrink(t *testing.T) {
	stop := bag.ScaledPoint(45 * bag.Factor)
	s := tabSettings(TabStop{Position: stop, Align: TabAlignDecimal})
	s.HSize = 100 * bag.Factor
	s.IndentRight = 45 * bag.Factor
	s.IndentRightRows = -2
	head := buildTabbed("aaaa bbbb cccc dddd eeee ffff", "a\tb cc.dd")
	var space *Glue
	for n := head; n != nil; n = n.Next() {
		if g, ok := n.(*Glue); ok && g.Subtype == GlueTab {
			p := NewPenalty()
			p.Penalty = 10000
			InsertBefore(head, g, p)
			// Enough shrink to take the overflow, were it allowed to.
			space = g.Next().Next().(*Glue)
			space.Shrink = 10 * bag.Factor
			break
		}
	}
	vlist, _ := Linebreak(head, s)
	ls := lines(vlist)
	if len(ls) != 3 {
		t.Fatalf("got %d lines, want 3", len(ls))
	}
	if w := lineContentWidths(vlist)[2]; w <= s.HSize-s.IndentRight {
		t.Fatalf("last line is %s wide, not overfull; the test proves nothing", w)
	}
	if x, ok := glyphAt(ls[2], "."); !ok || x != stop {
		t.Errorf("separator at %s, want %s", x, stop)
	}
	if space.Width != 3*bag.Factor {
		t.Errorf("space set to %s, want its natural 3pt", space.Width)
	}
}

// The first pass does not count the shrink of the text after a right stop
// either, as the second pass does not use it: a line that would only fit by
// shrinking the space in "ab cd" is no feasible break, and the paragraph
// breaks at the tab instead of coming out overfull.
func TestTabStopAlignedRunShrinkNotCounted(t *testing.T) {
	s := tabSettings(TabStop{Position: 100 * bag.Factor, Align: TabAlignRight})
	s.HSize = 100 * bag.Factor
	s.IndentRight = 10 * bag.Factor
	s.LineEndGlue = NewGlue()
	var head, cur Node
	add := func(n Node) { head = InsertAfter(head, cur, n); cur = n }
	head, cur = glyphRun(head, cur, "aaaa", tabCharWidth)
	sp := NewGlue()
	sp.Width, sp.Stretch = 10*bag.Factor, 20*bag.Factor // r = 1.6 on its own line
	add(sp)
	head, cur = glyphRun(head, cur, "bbbb", tabCharWidth)
	add(newTab())
	head, cur = glyphRun(head, cur, "ab", tabCharWidth)
	p := NewPenalty()
	p.Penalty = 10000
	add(p)
	shrink := NewGlue()
	shrink.Width, shrink.Shrink = 3*bag.Factor, 30*bag.Factor
	add(shrink)
	head, cur = glyphRun(head, cur, "cd", tabCharWidth)
	AppendLineEndAfter(head, cur)
	vlist, _ := Linebreak(head, s)
	ls := lines(vlist)
	if len(ls) != 2 {
		t.Fatalf("got %d lines, want 2, broken at the tab", len(ls))
	}
	for i, w := range lineContentWidths(vlist) {
		if w > s.HSize-s.IndentRight {
			t.Errorf("line %d is %s wide, overfull", i, w)
		}
	}
}

// Whether a run fits on the line, and so is kept together, is decided with
// both insets of the row: here an inset is what makes the figure too wide,
// so it stays breakable and does break.
func TestTabStopKeepFitCountsIndent(t *testing.T) {
	for _, tc := range []struct {
		name                string
		indent, indentRight bag.ScaledPoint
		stop                bag.ScaledPoint
	}{
		{"left", 45 * bag.Factor, 0, 85 * bag.Factor},
		{"right", 0, 45 * bag.Factor, 40 * bag.Factor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := tabSettings(TabStop{Position: tc.stop, Align: TabAlignDecimal})
			s.HSize = 100 * bag.Factor
			s.Indent, s.IndentRight = tc.indent, tc.indentRight
			vlist, _ := Linebreak(buildTabbed("a\tb c.ddd"), s)
			var starts []string
			for _, hl := range lines(vlist) {
				for n := hl.List; n != nil; n = n.Next() {
					if g, ok := n.(*Glyph); ok {
						starts = append(starts, g.Components)
						break
					}
				}
			}
			if !slices.Contains(starts, "c") {
				t.Errorf("line starts %v, want a break inside the figure", starts)
			}
			for i, w := range lineContentWidths(vlist) {
				if w > s.HSize-s.Indent-s.IndentRight {
					t.Errorf("line %d is %s wide, overfull", i, w)
				}
			}
		})
	}
}

// A line that ends at a discretionary inside the text after a right stop
// ends with the hyphen at the stop: the first pass counts the hyphen as part
// of the text it aligns.
func TestTabStopRightRunBrokenAtDisc(t *testing.T) {
	stop := bag.ScaledPoint(100 * bag.Factor)
	s := tabSettings(TabStop{Position: stop, Align: TabAlignRight})
	s.HSize = 100 * bag.Factor
	s.LineEndGlue = NewGlue()
	head := buildTabbed("aa\tbbbbbbbb cccccccc")
	var cs int
	for n := head; n != nil; n = n.Next() {
		if g, ok := n.(*Glyph); ok && g.Components == "c" {
			if cs++; cs == 3 {
				d := NewDisc()
				d.Penalty = -5000 // break here
				d.Pre, _ = glyphRun(nil, nil, "-", tabCharWidth)
				InsertAfter(head, g, d)
				break
			}
		}
	}
	vlist, _ := Linebreak(head, s)
	ls := lines(vlist)
	if len(ls) != 2 {
		t.Fatalf("got %d lines, want 2", len(ls))
	}
	if _, ends := tabRuns(ls[0]); len(ends) != 1 || ends[0] != stop {
		t.Errorf("first line: text after the tab ends at %v, want [%s]", ends, stop)
	}
	var last string
	for n := ls[0].List; n != nil; n = n.Next() {
		if g, ok := n.(*Glyph); ok {
			last = g.Components
		}
	}
	if last != "-" {
		t.Errorf("first line ends in %q, want the hyphen", last)
	}
}

// A right or center stop the text after the tab is too wide for, with the
// text before the tab in the way, keeps the tab at its own width, the width
// it has once the stops run out, rather than going on to the next stop or
// letting label and text touch. A run that leaves the tab more than that
// lines up as usual.
func TestTabStopAlignedRunTooWide(t *testing.T) {
	next := TabStop{Position: 150 * bag.Factor}
	for _, tc := range []struct {
		name  string
		stop  TabStop
		text  string
		start bag.ScaledPoint
	}{
		{"right", TabStop{Position: 30 * bag.Factor, Align: TabAlignRight}, "a\tbbbb", tabCharWidth + tabNatural},
		{"center", TabStop{Position: 20 * bag.Factor, Align: TabAlignCenter}, "a\tbbbbbb", tabCharWidth + tabNatural},
		{"right, room to spare", TabStop{Position: 60 * bag.Factor, Align: TabAlignRight}, "a\tbbbb", 36 * bag.Factor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vlist, _ := Linebreak(buildTabbed(tc.text), tabSettings(tc.stop, next))
			if got := afterTabs(lines(vlist)[0]); len(got) != 1 || got[0] != tc.start {
				t.Errorf("text after the tab starts at %v, want [%s]", got, tc.start)
			}
		})
	}
}

// With a right stop past the measure, a line ending in the text after the
// tab is as wide as the stop is far, whatever part of the text is on it, and
// so never fits: the first pass measures the part of the text on the line,
// not the whole of it, and breaks at the tab instead.
func TestTabStopRightPastTheMeasure(t *testing.T) {
	s := tabSettings(TabStop{Position: 110 * bag.Factor, Align: TabAlignRight})
	s.HSize = 100 * bag.Factor
	vlist, _ := Linebreak(buildTabbed("aa\tbb cc dd ee ff gg hh ii jj kk ll"), s)
	for i, w := range lineContentWidths(vlist) {
		if w > s.HSize {
			t.Errorf("line %d is %s wide, overfull", i, w)
		}
	}
}

// Both passes, and the pre-pass that decides which runs to keep, count the
// tab's own width: with it "bb cc dd ee ff gg" no longer fits after "a" and
// the tab, so it is not kept together and the line breaks inside it. The tab
// itself is no breakpoint here.
func TestTabStopTooWideRunBreaks(t *testing.T) {
	s := tabSettings(TabStop{Position: 30 * bag.Factor, Align: TabAlignRight})
	s.HSize = 100 * bag.Factor
	head := buildTabbed("a\tbb cc dd ee ff gg")
	for n := head; n != nil; n = n.Next() {
		if g, ok := n.(*Glue); ok && g.Subtype == GlueTab {
			p := NewPenalty()
			p.Penalty = 10000
			InsertBefore(head, g, p)
			break
		}
	}
	vlist, _ := Linebreak(head, s)
	if len(lines(vlist)) < 2 {
		t.Errorf("got %d line, want the run broken", len(lines(vlist)))
	}
	for i, w := range lineContentWidths(vlist) {
		if w > s.HSize {
			t.Errorf("line %d is %s wide, overfull", i, w)
		}
	}
}

// justified reports whether the line was set justified: its line-end glue
// has no fill, so the line's own glue took the slack.
func justified(hl *HList) bool {
	_, lineend := edgeGlues(hl)
	return lineend.StretchOrder == StretchNormal
}

// A figure at a decimal stop followed by prose too long to keep together:
// the part up to the separator is set at its natural width, and from the
// separator on the line is justified like any other, so the breaker fills
// it rather than taking any break in the prose at ratio 0. The space inside
// the integer part does not stretch.
func TestTabStopDecimalRunJustifies(t *testing.T) {
	stop := bag.ScaledPoint(120 * bag.Factor)
	s := tabSettings(TabStop{Position: stop, Align: TabAlignDecimal})
	s.LineEndGlue = NewGlue()
	head := buildTabbed("aa bb cc\t1 234.56 dd ee ff gg hh ii jj kk ll mm nn oo pp qq rr ss tt uu vv ww xx")
	// The space in "1 234" is a no-break space, as the frontend makes one.
	var inner *Glue
	for n := head; n != nil; n = n.Next() {
		if g, ok := n.(*Glyph); ok && g.Components == "1" {
			inner = g.Next().(*Glue)
			p := NewPenalty()
			p.Penalty = 10000
			InsertAfter(head, g, p)
			break
		}
	}
	vlist, bps := Linebreak(head, s)
	ls := lines(vlist)
	if len(ls) < 3 {
		t.Fatalf("got %d lines, want at least 3", len(ls))
	}
	if x, ok := glyphAt(ls[0], "."); !ok || x != stop {
		t.Errorf("separator at %s (found %v), want %s", x, ok, stop)
	}
	for i, hl := range ls[:len(ls)-1] {
		if !justified(hl) {
			t.Errorf("line %d is not justified", i)
		}
		if d := bps[i].R - hl.GlueSet; d > 1e-9 || d < -1e-9 {
			t.Errorf("line %d: broken at ratio %.3f, set at %.3f", i, bps[i].R, hl.GlueSet)
		}
		if bps[i].R == 0 {
			t.Errorf("line %d: broken at ratio 0, the fixture does not need the line to stretch", i)
		}
	}
	if inner.Width != 3*bag.Factor {
		t.Errorf("space in the integer part set to %s, want its natural 3pt", inner.Width)
	}
}

// A left to right number in a right to left paragraph runs back towards the
// start edge past its separator, so there is no part after the separator to
// justify: the line ends in the figure's run and takes no stretch, as before.
func TestTabStopDecimalRunRightToLeftNotJustified(t *testing.T) {
	stop := bag.ScaledPoint(120 * bag.Factor)
	s := tabSettings(TabStop{Position: stop, Align: TabAlignDecimal})
	s.TextDirection = TextDirRTL
	s.LineStartGlue, s.LineEndGlue = NewGlue(), NewGlue()
	head := buildTabbed("aa bb cc\t1234.56 dd ee ff gg hh ii jj kk ll mm nn oo pp qq rr")
	seen := false
	for n := head; n != nil; n = n.Next() {
		if g, ok := n.(*Glue); ok && g.Subtype == GlueTab {
			seen = true
		}
		if seen {
			n.SetBidiLevel(2)
		} else {
			n.SetBidiLevel(1)
		}
	}
	vlist, bps := Linebreak(head, s)
	ls := lines(vlist)
	if len(ls) < 2 {
		t.Fatalf("got %d lines, want at least 2", len(ls))
	}
	if bps[0].R != 0 {
		t.Errorf("line 0: broken at ratio %.3f, want 0", bps[0].R)
	}
	if leftskip, _ := edgeGlues(ls[0]); leftskip.StretchOrder == StretchNormal {
		t.Error("line 0: the line end (the left edge) takes no fill")
	}
}

// A figure too wide to keep that breaks before its separator: the part on
// the line has no separator, so it ends at the stop like a right-aligned run
// and the line end takes the slack. Only a line that holds the separator
// justifies from it.
func TestTabStopDecimalRunBrokenBeforeSeparator(t *testing.T) {
	stop := bag.ScaledPoint(120 * bag.Factor)
	s := tabSettings(TabStop{Position: stop, Align: TabAlignDecimal})
	s.LineEndGlue = NewGlue()
	vlist, bps := Linebreak(buildTabbed("aa bb cc\t1 234.56 dd ee ff gg hh ii jj kk ll mm nn oo pp qq rr ss tt uu vv ww xx"), s)
	ls := lines(vlist)
	if _, ends := tabRuns(ls[0]); len(ends) != 1 || ends[0] != stop {
		t.Fatalf("line 0: the part of the figure ends at %v, want [%s]", ends, stop)
	}
	if justified(ls[0]) {
		t.Error("line 0 is justified, want its line end to take the slack")
	}
	checkRatios(t, ls, bps)
}

// A figure kept together stays at its natural width after the separator
// too, up to the next tab.
func TestTabStopKeptDecimalRunNotJustified(t *testing.T) {
	s := tabSettings(TabStop{Position: 60 * bag.Factor, Align: TabAlignDecimal})
	s.LineEndGlue = NewGlue()
	head := buildTabbed("aa\t1.5 bb\tcc dd ee ff gg hh ii jj kk ll mm nn oo pp qq rr")
	var space *Glue
	for n := head; n != nil; n = n.Next() {
		if g, ok := n.(*Glyph); ok && g.Components == "5" {
			space = g.Next().(*Glue)
			break
		}
	}
	vlist, bps := Linebreak(head, s)
	if space.Width != 3*bag.Factor {
		t.Errorf("space after the figure set to %s, want its natural 3pt", space.Width)
	}
	checkRatios(t, lines(vlist), bps)
}
