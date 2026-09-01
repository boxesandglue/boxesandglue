package frontend

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// filGlue is the alignment filler a line carries: rightskip for ragged right,
// leftskip for text-align:right, one of each for center. Its defining property
// is infinite stretch, which is what marks it as not-content.
func filGlue() *node.Glue {
	g := node.NewGlue()
	g.Stretch = bag.MustSP("1pt")
	g.StretchOrder = node.StretchFil
	return g
}

func fixedGlue(wd bag.ScaledPoint) *node.Glue {
	g := node.NewGlue()
	g.Width = wd
	return g
}

func ruleOfWidth(wd bag.ScaledPoint) *node.Rule {
	r := node.NewRule()
	r.Width = wd
	r.Height = bag.MustSP("10pt")
	return r
}

func line(items ...node.Node) *node.HList {
	var head node.Node
	for _, item := range items {
		head = node.InsertAfter(head, node.Tail(head), item)
	}
	return node.Hpack(head)
}

// TestNaturalWidthIgnoresAlignmentFiller checks that a line's natural width is
// the width of its content wherever the alignment filler happens to sit. The
// scan used to stop at the first infinitely stretchable glue, which is only
// the right thing to do when that glue trails the content. Lead with it, as
// text-align:right and :center do, and the line measured zero.
func TestNaturalWidthIgnoresAlignmentFiller(t *testing.T) {
	want := bag.MustSP("50pt")

	cases := []struct {
		name string
		hl   *node.HList
	}{
		{"ragged right", line(ruleOfWidth(want), filGlue())},
		{"text-align:right", line(filGlue(), ruleOfWidth(want))},
		{"text-align:center", line(filGlue(), ruleOfWidth(want), filGlue())},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getMaxWidthHlistWithoutStretch(tc.hl); got != want {
				t.Errorf("max width = %s, want %s", got, want)
			}
			if got := getMinWidthHlistWithoutStretch(tc.hl); got != want {
				t.Errorf("min width = %s, want %s", got, want)
			}
		})
	}
}

// TestNaturalWidthKeepsFiniteGlue guards the other half of the rule: only
// infinite stretch is filler. An ordinary interword space is content and is
// measured at its natural width.
func TestNaturalWidthKeepsFiniteGlue(t *testing.T) {
	hl := line(ruleOfWidth(bag.MustSP("30pt")), fixedGlue(bag.MustSP("5pt")), ruleOfWidth(bag.MustSP("20pt")))
	want := bag.MustSP("55pt")
	if got := getMaxWidthHlistWithoutStretch(hl); got != want {
		t.Errorf("max width = %s, want %s", got, want)
	}
}
