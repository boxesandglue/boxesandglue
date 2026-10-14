package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// A line that needs its glyphs 2.5 percent narrower gets an expansion of
// -2.5, not -2: a whole percent left the glyphs too wide and the line
// past the right margin (boxesandglue#86).
func TestHpackExpandIsAFraction(t *testing.T) {
	var head, tail Node
	for i := range 10 {
		if i == 5 {
			g := NewGlue()
			g.Width = bag.MustSP("5pt")
			g.Shrink = bag.MustSP("1pt")
			head = InsertAfter(head, tail, g)
			tail = g
		}
		gl := NewGlyph()
		gl.Width = bag.MustSP("10pt")
		head = InsertAfter(head, tail, gl)
		tail = gl
	}
	// 100pt of glyphs and 5pt of glue that shrinks by 1pt: at 101.5pt the
	// glyphs shrink by the remaining 2.5pt, 2.5 percent of their width.
	hl := HpackToWithEnd(head, tail, bag.MustSP("101.5pt"), FontExpansion(0.05))
	got, ok := hl.Attributes["expand"].(float64)
	if !ok {
		t.Fatalf("expand is %#v, want a float64", hl.Attributes["expand"])
	}
	if got != -2.5 {
		t.Errorf("expand %v, want -2.5", got)
	}
}
