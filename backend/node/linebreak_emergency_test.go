package node

import (
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// buildWordsHardBreak is buildWords with a HardBreak in place of the interword
// glue before word hb.
func buildWordsHardBreak(words []string, hb int, charWidth, spaceWidth bag.ScaledPoint) Node {
	var head, cur Node
	for i, w := range words {
		switch {
		case i == hb:
			hb := NewHardBreak()
			head = InsertAfter(head, cur, hb)
			cur = hb
		case i > 0:
			sp := NewGlue()
			sp.Width = spaceWidth
			sp.Stretch = spaceWidth
			sp.Shrink = spaceWidth / 3
			head = InsertAfter(head, cur, sp)
			cur = sp
		}
		head, cur = glyphRun(head, cur, w, charWidth)
	}
	head, _ = AppendLineEndAfter(head, cur)
	return head
}

// firstLineGlyphs counts the glyphs of the paragraph's first line.
func firstLineGlyphs(t *testing.T, vl *VList) int {
	t.Helper()
	for n := vl.List; n != nil; n = n.Next() {
		if hl, ok := n.(*HList); ok {
			c := 0
			for m := hl.List; m != nil; m = m.Next() {
				if _, ok := m.(*Glyph); ok {
					c++
				}
			}
			return c
		}
	}
	t.Fatal("no line")
	return 0
}

// emergencySettings is a justified 100pt measure with tolerance 1. With
// seven-glyph words of 7pt and 3pt spaces (stretch 3pt, shrink 1pt), the
// first line can end after word 9 (r=0.54), 10 (r=0.11) or 11 (r=-0.7), and
// after word 10 it has the fewest demerits. No line from one of those breaks
// to a HardBreak two to four words later is feasible.
func emergencySettings() *LinebreakSettings {
	s := NewLinebreakSettings()
	s.HSize = 100 * bag.Factor
	s.LineHeight = 12 * bag.Factor
	s.Tolerance = 1
	return s
}

func sevenGlyphWords(n int) []string {
	words := make([]string, n)
	for i := range words {
		words[i] = strings.Repeat("a", 7)
	}
	return words
}

// TestEmergencyAnchorFewestDemerits: before a HardBreak that no active
// breakpoint reaches with a feasible line, the emergency line starts at the
// deactivated breakpoint with the fewest demerits, not at the one deactivated
// last. The three candidates end line 1 in the same class, where the oldest
// (after word 9) is the last in the active list (bag#74).
func TestEmergencyAnchorFewestDemerits(t *testing.T) {
	// Word 12 ends with a glue at which the line from the start is overfull,
	// so the start is gone before the HardBreak after word 13.
	words := sevenGlyphWords(15)
	head := buildWordsHardBreak(words, 13, bag.Factor, 3*bag.Factor)
	vl, _ := Linebreak(head, emergencySettings())
	if got, want := firstLineGlyphs(t, vl), 10*7; got != want {
		t.Errorf("first line has %d glyphs, want %d (ten words)", got, want)
	}
}

// TestEmergencyAnchorPrefersALineThatFits: when a HardBreak finds the line
// from one breakpoint overfull and the lines from others only too loose, the
// emergency line starts at one of the loose ones. Anchoring at the overfull
// one would set the twelve words up to the HardBreak as one line of 117pt in
// a 100pt measure.
func TestEmergencyAnchorPrefersALineThatFits(t *testing.T) {
	words := sevenGlyphWords(14)
	head := buildWordsHardBreak(words, 12, bag.Factor, 3*bag.Factor)
	vl, _ := Linebreak(head, emergencySettings())
	if got, want := firstLineGlyphs(t, vl), 10*7; got != want {
		t.Errorf("first line has %d glyphs, want %d (ten words)", got, want)
	}
	for i, w := range lineContentWidths(vl) {
		if w > 100*bag.Factor {
			t.Errorf("line %d is %s wide, past the 100pt measure", i+1, w)
		}
	}
}
