package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// rigidRow is three glues of 10pt, 20pt and 30pt that can neither stretch
// nor shrink, like the columns of a table with fixed widths.
func rigidRow() (Node, Node, []*Glue) {
	var head, tail Node
	var glues []*Glue
	for _, wd := range []string{"10pt", "20pt", "30pt"} {
		g := NewGlue()
		g.Width = bag.MustSP(wd)
		glues = append(glues, g)
		head = InsertAfter(head, tail, g)
		tail = g
	}
	return head, tail, glues
}

// A list packed to a width its glue cannot reach keeps its glue as it is.
// The ratio was ±Inf, which only showed on amd64, so this checks the ratio
// and the badness too.
func TestHpackRigidGlue(t *testing.T) {
	for _, tc := range []struct {
		name    string
		delta   bag.ScaledPoint
		badness int
	}{
		{"underfull", 1, 10000},
		{"overfull", -1, 1000000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			head, tail, glues := rigidRow()
			hl := HpackToWithEnd(head, tail, bag.MustSP("60pt")+tc.delta, SqueezeOverfullBoxes(true))
			for i, want := range []string{"10pt", "20pt", "30pt"} {
				if glues[i].Width != bag.MustSP(want) {
					t.Errorf("glue %d is %s wide, want %s", i, glues[i].Width, want)
				}
			}
			if hl.GlueSet != 0 {
				t.Errorf("glue set %v, want 0", hl.GlueSet)
			}
			if hl.Badness != tc.badness {
				t.Errorf("badness %d, want %d", hl.Badness, tc.badness)
			}
		})
	}
}
