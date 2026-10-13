package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// glueRow is two glues of 10pt that shrink by 4pt each. The first one also
// stretches infinitely, as a line-end glue that may shrink would.
func glueRow() (Node, Node, []*Glue) {
	var glues []*Glue
	var head, tail Node
	for i := range 2 {
		g := NewGlue()
		g.Width = bag.MustSP("10pt")
		g.Shrink = bag.MustSP("4pt")
		if i == 0 {
			g.Stretch = bag.Factor
			g.StretchOrder = StretchFil
		}
		glues = append(glues, g)
		head = InsertAfter(head, tail, g)
		tail = g
	}
	return head, tail, glues
}

// A glue's shrink counts in the order of its shrink, not of its stretch:
// a long line shrinks both glues, the one that also stretches infinitely
// too.
func TestHpackShrinksByTheShrinkOrder(t *testing.T) {
	head, tail, glues := glueRow()
	hl := HpackToWithEnd(head, tail, bag.MustSP("16pt"))
	for i, g := range glues {
		if want := bag.MustSP("8pt"); g.Width != want {
			t.Errorf("glue %d is %s wide, want %s", i, g.Width, want)
		}
	}
	if hl.GlueSet != -0.5 || hl.GlueOrder != StretchNormal {
		t.Errorf("glue set %v of order %d, want -0.5 of order 0", hl.GlueSet, hl.GlueOrder)
	}
}

// The packed list records the order of the glue it set, as TeX's
// glue_order: the infinite stretch of a short line.
func TestHpackRecordsTheGlueOrder(t *testing.T) {
	head, tail, _ := glueRow()
	hl := HpackToWithEnd(head, tail, bag.MustSP("30pt"))
	if hl.GlueOrder != StretchFil {
		t.Errorf("glue order %d, want %d (fil)", hl.GlueOrder, StretchFil)
	}
}
