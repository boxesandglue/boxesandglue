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
