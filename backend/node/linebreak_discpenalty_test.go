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

// After a line that ends at a discretionary, a line ending at another one
// costs DoublehyphenDemerits, and the last line of the paragraph
// FinalHyphenDemerits (TeX: The Program §859).
func TestHyphenDemerits(t *testing.T) {
	// inText puts n before a glyph, as a break inside a paragraph is.
	inText := func(n Node) Node {
		InsertAfter(n, n, NewGlyph())
		return n
	}
	disc := func() Node { return inText(NewDisc()) }
	glue := func() Node { return inText(NewGlue()) }
	end := func() Node {
		_, tail := AppendLineEndAfter(NewGlyph(), NewGlyph())
		return tail
	}
	for _, tc := range []struct {
		desc      string
		prev, cur func() Node
		want      int
	}{
		{"hyphen after hyphen", disc, disc, 3000},
		{"paragraph end after hyphen", disc, end, 5000},
		{"glue after hyphen", disc, glue, 0},
		{"hyphen after glue", glue, disc, 0},
		{"paragraph end after glue", glue, end, 0},
	} {
		s := NewLinebreakSettings()
		s.Hyphenpenalty = 0
		lb := newLinebreaker(s)
		// r 0 is decent, as the previous line, so only the hyphens count
		// beyond (LinePenalty + 0)².
		_, got := lb.calculateDemerits(&Breakpoint{Position: tc.prev(), Fitness: 1}, 0, tc.cur())
		if got -= s.LinePenalty * s.LinePenalty; got != tc.want {
			t.Errorf("%s: %d demerits beyond the line's own, want %d", tc.desc, got, tc.want)
		}
	}
}
