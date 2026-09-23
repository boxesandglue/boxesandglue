package node

import (
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
		// HpackToWithEnd reports 1 for an exact fit, where the breaker has 0.
		if bps[i].R == 0 && set == 1 {
			continue
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
