package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// glyphRun appends one Glyph per rune of s with the given per-glyph width.
// Returns the new head and tail.
func glyphRun(head, cur Node, s string, width bag.ScaledPoint) (Node, Node) {
	for _, r := range s {
		g := NewGlyph()
		g.Width = width
		g.Components = string(r)
		head = InsertAfter(head, cur, g)
		cur = g
	}
	return head, cur
}

// countHLists returns the number of HList children in the VList.
func countHLists(v *VList) int {
	if v == nil {
		return 0
	}
	c := 0
	for n := v.List; n != nil; n = n.Next() {
		if _, ok := n.(*HList); ok {
			c++
		}
	}
	return c
}

// TestHardBreakProducesTwoLines builds "AB<HardBreak>CD" with a line width
// that easily fits "ABCD" on one line. The HardBreak must still split the
// content into two lines.
func TestHardBreakProducesTwoLines(t *testing.T) {
	const charWidth = bag.ScaledPoint(10 * bag.Factor)

	var head, cur Node
	head, cur = glyphRun(head, cur, "AB", charWidth)
	hb := NewHardBreak()
	head = InsertAfter(head, cur, hb)
	cur = hb
	head, cur = glyphRun(head, cur, "CD", charWidth)
	AppendLineEndAfter(head, cur)

	settings := NewLinebreakSettings()
	settings.HSize = 200 * bag.Factor // wide enough for the whole content
	settings.LineHeight = 12 * bag.Factor

	vlist, _ := Linebreak(head, settings)
	if got, want := countHLists(vlist), 2; got != want {
		t.Errorf("HardBreak yielded %d HLists, want %d", got, want)
	}
}

// TestConsecutiveHardBreaksMakeBlankLines verifies that N HardBreaks in a
// row produce N forced breaks, with N-1 empty HBoxes between them — that's
// how blank lines in plain-text source ("\n\n", "\n\n\n", …) materialise
// without a dedicated ParagraphBreak type.
func TestConsecutiveHardBreaksMakeBlankLines(t *testing.T) {
	const charWidth = bag.ScaledPoint(10 * bag.Factor)
	const lineHeight = bag.ScaledPoint(12 * bag.Factor)

	for _, n := range []int{2, 3, 4} {
		t.Run("", func(t *testing.T) {
			var head, cur Node
			head, cur = glyphRun(head, cur, "AB", charWidth)
			for k := 0; k < n; k++ {
				hb := NewHardBreak()
				head = InsertAfter(head, cur, hb)
				cur = hb
			}
			head, cur = glyphRun(head, cur, "CD", charWidth)
			AppendLineEndAfter(head, cur)

			settings := NewLinebreakSettings()
			settings.HSize = 200 * bag.Factor
			settings.LineHeight = lineHeight

			vlist, _ := Linebreak(head, settings)
			// n HardBreaks split the content into n+1 "lines", but the
			// final empty line collapses with the AppendLineEndAfter
			// trailer. We expect n+1 HLists in the resulting VList: AB,
			// (n-1) empty lines, CD. Total = n+1 HLists, with (n-1)
			// having effectively zero content height.
			gotHLists := countHLists(vlist)
			wantHLists := n + 1
			if gotHLists != wantHLists {
				t.Errorf("%d HardBreaks yielded %d HLists, want %d",
					n, gotHLists, wantHLists)
			}
		})
	}
}

// TestHardBreakNoExtraGap ensures that a single HardBreak between two text
// lines yields the regular lineskip gap (one baseline-skip of LineHeight),
// not more.
func TestHardBreakNoExtraGap(t *testing.T) {
	const charWidth = bag.ScaledPoint(10 * bag.Factor)
	const lineHeight = bag.ScaledPoint(12 * bag.Factor)

	var head, cur Node
	head, cur = glyphRun(head, cur, "AB", charWidth)
	hb := NewHardBreak()
	head = InsertAfter(head, cur, hb)
	cur = hb
	head, cur = glyphRun(head, cur, "CD", charWidth)
	AppendLineEndAfter(head, cur)

	settings := NewLinebreakSettings()
	settings.HSize = 200 * bag.Factor
	settings.LineHeight = lineHeight

	vlist, _ := Linebreak(head, settings)
	var lineskip *Glue
	for n := vlist.List; n != nil; n = n.Next() {
		g, ok := n.(*Glue)
		if !ok {
			continue
		}
		if origin, _ := g.Attributes["origin"].(string); origin == "lineskip" {
			lineskip = g
			break
		}
	}
	if lineskip == nil {
		t.Fatal("no lineskip glue found between two lines")
	}
	if lineskip.Width != lineHeight {
		t.Errorf("HardBreak lineskip = %s, want %s (no extra gap)",
			lineskip.Width, lineHeight)
	}
}

// TestHardBreakSuppressesJustify exercises the Justify-vs-HardBreak corner
// case: in Justify mode (no per-line LineStartGlue/LineEndGlue with fill
// stretch), a forced break in the middle of a paragraph would otherwise
// cause the line preceding the break to be justified — Word-Glues with
// SpaceStretch get stretched out so the line spans HSize. The fix is to
// detect Justify mode at the HardBreak boundary and substitute a
// fill-stretch LineEndGlue for that one line, which absorbs the slack and
// keeps Word-Glues at their natural width.
func TestHardBreakSuppressesJustify(t *testing.T) {
	const charWidth = bag.ScaledPoint(10 * bag.Factor)
	const spaceWidth = bag.ScaledPoint(5 * bag.Factor)
	const spaceStretch = bag.ScaledPoint(2 * bag.Factor)

	mkSpace := func() *Glue {
		g := NewGlue()
		g.Width = spaceWidth
		g.Stretch = spaceStretch
		return g
	}

	var head, cur Node
	head, cur = glyphRun(head, cur, "AA", charWidth)
	sp1 := mkSpace()
	head = InsertAfter(head, cur, sp1)
	cur = sp1
	head, cur = glyphRun(head, cur, "BB", charWidth)

	hb := NewHardBreak()
	head = InsertAfter(head, cur, hb)
	cur = hb

	head, cur = glyphRun(head, cur, "CC", charWidth)
	sp2 := mkSpace()
	head = InsertAfter(head, cur, sp2)
	cur = sp2
	head, cur = glyphRun(head, cur, "DD", charWidth)
	AppendLineEndAfter(head, cur)

	settings := NewLinebreakSettings()
	// Justify: LineEndGlue / LineStartGlue are NewGlue() defaults — no
	// fill stretch, no width. That's the configuration FormatParagraph
	// uses for HAlignJustified.
	settings.HSize = 200 * bag.Factor // wide enough so naturally < HSize
	settings.LineHeight = 12 * bag.Factor

	vlist, _ := Linebreak(head, settings)

	var firstHL *HList
	for n := vlist.List; n != nil; n = n.Next() {
		if hl, ok := n.(*HList); ok {
			firstHL = hl
			break
		}
	}
	if firstHL == nil {
		t.Fatal("no HList found")
	}
	// Hpack mutates the Glue.Width of the per-line word-glue to apply
	// stretch/shrink. After the fix the line carries a fill-stretch
	// LineEndGlue whose higher StretchOrder dominates the per-line glue
	// distribution, so the inline word-glues stay at their natural
	// width. Without the fix, only word-glues provide stretch — they
	// get spread out to span HSize, and their Width grows.
	var wordGlue *Glue
	for n := firstHL.List; n != nil; n = n.Next() {
		g, ok := n.(*Glue)
		if !ok {
			continue
		}
		// Skip the leftskip (copy of LineStartGlue) and the synthesised
		// lineend — we want the original inter-word glue, which carries
		// no "origin" attribute because we built it by hand in the test.
		if _, hasOrigin := g.Attributes["origin"]; hasOrigin {
			continue
		}
		wordGlue = g
		break
	}
	if wordGlue == nil {
		t.Fatal("no word glue found in first HList")
	}
	if wordGlue.Width != spaceWidth {
		t.Errorf("word glue width after Hpack = %s, want %s "+
			"— inline word-glue was stretched, the line before the "+
			"forced break is being justified", wordGlue.Width, spaceWidth)
	}
}

// TestForcedBreakNoStretchInLine verifies the core promise of the refactor:
// the HBox produced for a line that ends in a HardBreak does not contain a
// fill-stretch Glue that would compete with per-line alignment glue. We
// build "AB<HardBreak>CD", run Linebreak, walk the first HList's children,
// and assert that no inline node has StretchOrder >= StretchFill.
func TestForcedBreakNoStretchInLine(t *testing.T) {
	const charWidth = bag.ScaledPoint(10 * bag.Factor)

	var head, cur Node
	head, cur = glyphRun(head, cur, "AB", charWidth)
	hb := NewHardBreak()
	head = InsertAfter(head, cur, hb)
	cur = hb
	head, cur = glyphRun(head, cur, "CD", charWidth)
	AppendLineEndAfter(head, cur)

	settings := NewLinebreakSettings()
	settings.HSize = 200 * bag.Factor
	settings.LineHeight = 12 * bag.Factor

	vlist, _ := Linebreak(head, settings)

	var firstHList *HList
	for n := vlist.List; n != nil; n = n.Next() {
		if hl, ok := n.(*HList); ok {
			firstHList = hl
			break
		}
	}
	if firstHList == nil {
		t.Fatal("no HList found")
	}
	for n := firstHList.List; n != nil; n = n.Next() {
		if g, ok := n.(*Glue); ok {
			origin, _ := g.Attributes["origin"].(string)
			// LineEndGlue and lineend are inserted by Linebreak as
			// the per-line trailing glue — they are allowed to be
			// stretchy. Only inline glues from the source list
			// would carry stretch from the old Penalty/Glue/Penalty
			// pattern.
			if origin == "lineend" || origin == "leftskip" {
				continue
			}
			if g.StretchOrder >= StretchFill {
				t.Errorf("found inline glue with StretchOrder=%d (origin=%q) — "+
					"forced break should not emit fill stretch",
					g.StretchOrder, origin)
			}
		}
	}
}
