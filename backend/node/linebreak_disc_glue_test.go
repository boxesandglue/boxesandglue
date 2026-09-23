package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// TestLinebreakDiscBeforeGlue tests that when a Disc (hyphenation point) is
// immediately followed by a Glue (space), the Glue is still considered as a
// valid breakpoint. This tests a bug where Disc nodes incorrectly reset
// prevItemBox, preventing the following Glue from being a breakpoint.
func TestLinebreakDiscBeforeGlue(t *testing.T) {
	// We construct: "word1| word2" where | is a Disc (hyphenation point)
	// right before the space. The line width is set so:
	// - "word1 word2" doesn't fit on one line
	// - The break should happen at the space (Glue), not at the Disc
	//
	// Bug behavior: Disc sets prevItemBox=false, so Glue is not considered
	// as a breakpoint. The algorithm either:
	// - Breaks at Disc (inserting unwanted hyphen), or
	// - Can't break at all

	// Simple character widths
	charWidth := bag.ScaledPoint(10 * bag.Factor)
	spaceWidth := bag.ScaledPoint(6 * bag.Factor)
	hyphenWidth := bag.ScaledPoint(6 * bag.Factor)

	hyphenchar := NewGlyph()
	hyphenchar.Width = hyphenWidth
	hyphenchar.Components = "-"

	var cur, head Node

	// Build "word1" (5 chars = 50pt)
	for _, r := range "word1" {
		g := NewGlyph()
		g.Width = charWidth
		g.Components = string(r)
		head = InsertAfter(head, cur, g)
		cur = g
	}

	// Disc at end of word1 (hyphenation point just before space)
	disc := NewDisc()
	disc.Pre = hyphenchar.Copy()
	head = InsertAfter(head, cur, disc)
	cur = disc

	// Space (Glue) - THIS is where the break should happen
	space := NewGlue()
	space.Width = spaceWidth
	space.Stretch = 3 * bag.Factor
	space.Shrink = 2 * bag.Factor
	head = InsertAfter(head, cur, space)
	cur = space

	// Build "word2" (5 chars = 50pt)
	for _, r := range "word2" {
		g := NewGlyph()
		g.Width = charWidth
		g.Components = string(r)
		head = InsertAfter(head, cur, g)
		cur = g
	}

	AppendLineEndAfter(head, cur)

	settings := NewLinebreakSettings()
	// Total width: 50 + 6 + 50 = 106pt
	// Set HSize = 50pt, exactly word1's width
	// When breaking at Glue, line 1 = word1 = 50pt (perfect fit)
	settings.HSize = 50 * bag.Factor
	settings.LineHeight = 12 * bag.Factor
	settings.Hyphenpenalty = 50
	settings.Tolerance = 4.0

	vlist, bps := Linebreak(head, settings)

	t.Logf("Number of breakpoints: %d", len(bps))
	for i, bp := range bps {
		t.Logf("BP %d: Line=%d, Position=%T, R=%f, Demerits=%d, Pre=%v",
			i, bp.Line, bp.Position, bp.R, bp.Demerits, bp.Pre != nil)
	}

	if vlist == nil {
		t.Fatal("vlist is nil")
	}

	// We expect 2 lines
	if len(bps) < 2 {
		t.Errorf("expected at least 2 breakpoints (2 lines), got %d", len(bps))
		t.Log("This likely means the Glue after Disc is not being considered as a breakpoint")
		return
	}

	// Check that the break after "word1" does NOT have a hyphen
	// (it should break at the Glue/space, not at the Disc)
	if len(bps) >= 2 {
		// bps[0] is the start, bps[1] is the first break
		firstBreak := bps[1]
		if firstBreak.Pre != nil {
			t.Errorf("first line break has Pre (hyphen) set, but should be nil when breaking at space")
			t.Log("Break position type:", firstBreak.Position)
		}
	}
}

// TestDiscBeforeGlueHyphenWidth checks that the first pass measures a break
// at a Disc followed by glue the way the second pass sets it: without the
// hyphen, and without the glue at the start of the next line (issue #37).
func TestDiscBeforeGlueHyphenWidth(t *testing.T) {
	var head, cur Node
	add := func(n Node) { head = InsertAfter(head, cur, n); cur = n }
	word := func(s string) { head, cur = glyphRun(head, cur, s, 6*bag.Factor) }
	space := func() {
		g := NewGlue()
		g.Width, g.Stretch, g.Shrink = 3*bag.Factor, 3*bag.Factor, bag.Factor
		add(g)
	}
	word("aaa")
	space()
	word("bbb")
	space()
	word("ccc")
	d := NewDisc()
	d.Penalty = -5000
	d.Pre, _ = glyphRun(nil, nil, "-", 6*bag.Factor)
	add(d)
	space()
	word("ddd")
	space()
	word("eee")
	space()
	word("fff")
	space()
	word("ggg")
	AppendLineEndAfter(head, cur)
	s := NewLinebreakSettings()
	s.HSize = 75 * bag.Factor
	s.LineHeight = 12 * bag.Factor
	vlist, bps := Linebreak(head, s)
	i := 0
	for n := vlist.List; n != nil; n = n.Next() {
		hl, ok := n.(*HList)
		if !ok {
			continue
		}
		// the last line is set with the fill at the line end, not at its ratio
		if i < len(bps)-1 && bps[i].R != hl.GlueSet {
			t.Errorf("line %d broken at ratio %.3f, set at %.3f", i, bps[i].R, hl.GlueSet)
		}
		if i == 0 {
			if g, ok := hl.List.Next().(*Glyph); !ok || g.Components != "a" {
				t.Errorf("line 0 starts with %v, want the glyph a", hl.List.Next())
			}
			for m := hl.List; m != nil; m = m.Next() {
				if g, ok := m.(*Glyph); ok && g.Components == "-" {
					t.Errorf("line 0 has a hyphen at a word boundary")
				}
			}
		}
		if i == 1 {
			if g, ok := hl.List.Next().(*Glyph); !ok || g.Components != "d" {
				t.Errorf("line 1 starts with %v, want the glyph d after the discarded space", hl.List.Next())
			}
		}
		i++
	}
	if i != 3 {
		t.Errorf("got %d lines, want 3", i)
	}
}
