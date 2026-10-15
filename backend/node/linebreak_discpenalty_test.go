package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// buildHyphenatedWords builds words of 10pt glyphs with 6pt spaces and a Disc
// with a 10pt hyphen after the third glyph of every word. It returns the list
// and the number of Discs in it.
func buildHyphenatedWords(n int) (Node, int) {
	var head, cur Node
	for i := range n {
		if i > 0 {
			sp := NewGlue()
			sp.Width, sp.Stretch, sp.Shrink = 6*bag.Factor, 6*bag.Factor, 2*bag.Factor
			head = InsertAfter(head, cur, sp)
			cur = sp
		}
		head, cur = glyphRun(head, cur, "aaa", 10*bag.Factor)
		d := NewDisc()
		hyphen := NewGlyph()
		hyphen.Width = 10 * bag.Factor
		hyphen.Components = "-"
		d.Pre = hyphen
		head = InsertAfter(head, cur, d)
		cur = d
		head, cur = glyphRun(head, cur, "aaa", 10*bag.Factor)
	}
	head, _ = AppendLineEndAfter(head, cur)
	return head, n
}

func discBreaks(bps []*Breakpoint) int {
	c := 0
	for _, bp := range bps {
		if _, ok := bp.Position.(*Disc); ok {
			c++
		}
	}
	return c
}

// As at a penalty, a hyphenation point whose penalty is 10000 or more is no
// break, and one whose penalty is -10000 or less is a forced break.
func TestDiscPenaltyLimits(t *testing.T) {
	for _, tc := range []struct {
		hyphenpenalty int
		want          func(discs, breaks int) bool
		desc          string
	}{
		{10000, func(_, b int) bool { return b == 0 }, "no break at a Disc"},
		{-10000, func(d, b int) bool { return b == d }, "a break at every Disc"},
	} {
		// 85pt takes one word of 60pt, two of 126pt do not fit, so the
		// paragraph breaks between the words or inside them.
		head, discs := buildHyphenatedWords(6)
		s := NewLinebreakSettings()
		s.HSize = 85 * bag.Factor
		s.Hyphenpenalty = tc.hyphenpenalty
		_, bps := Linebreak(head, s)
		if got := discBreaks(bps); !tc.want(discs, got) {
			t.Errorf("Hyphenpenalty %d: %d breaks at %d Discs, want %s", tc.hyphenpenalty, got, discs, tc.desc)
		}
	}
}
