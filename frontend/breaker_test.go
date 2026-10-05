package frontend

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// everyBreak breaks at every legal breakpoint and counts its calls.
type everyBreak struct{ calls int }

func (b *everyBreak) Breaks(p *node.BreakProblem) []int {
	b.calls++
	breaks := make([]int, len(p.Candidates))
	for i := range breaks {
		breaks[i] = i
	}
	return breaks
}

// SettingBreaker reaches Linebreak through FormatParagraph: a breaker that
// breaks at every space sets one word a line, where Knuth-Plass sets the
// paragraph on one line of the same width.
func TestSettingBreaker(t *testing.T) {
	fe, ff := lineModelDocument(t)
	format := func(b node.Breaker) int {
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		if b != nil {
			te.Settings[SettingBreaker] = b
		}
		te.Items = append(te.Items, "one two three four")
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("200pt"))
		if err != nil {
			t.Fatal(err)
		}
		lines := 0
		for n := vl.List; n != nil; n = n.Next() {
			if _, ok := n.(*node.HList); ok {
				lines++
			}
		}
		return lines
	}
	if n := format(nil); n != 1 {
		t.Fatalf("Knuth-Plass sets %d lines, want 1", n)
	}
	b := &everyBreak{}
	if n := format(b); n != 4 || b.calls != 1 {
		t.Errorf("with the breaker: %d lines and %d calls, want 4 lines and 1 call", n, b.calls)
	}
}
