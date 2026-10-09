package node

import (
	"math/rand"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// TestLineAfterAPenaltyIsSetAsPlanned checks that every line is set with the
// natural width the first pass planned it with, in paragraphs where some
// spaces carry a penalty before the glue. A break at such a penalty discards
// the glue after it in the first pass, so the second pass must not put it at
// the start of the next line.
func TestLineAfterAPenaltyIsSetAsPlanned(t *testing.T) {
	rng := rand.New(rand.NewSource(80))
	penalties := []int{-10000, -50, 0, 50}
	for c := 0; c < 300; c++ {
		var head, cur Node
		// space holds the paragraph's own glues: Hpack sets their width,
		// so the natural width of a line counts 3pt for each.
		space := map[Node]bool{}
		add := func(n Node) {
			head = InsertAfter(head, cur, n)
			cur = n
		}
		words := 4 + rng.Intn(30)
		for i := 0; i < words; i++ {
			if i > 0 {
				if rng.Intn(3) == 0 {
					p := NewPenalty()
					p.Penalty = penalties[rng.Intn(len(penalties))]
					add(p)
				}
				sp := NewGlue()
				sp.Width, sp.Stretch, sp.Shrink = 3*bag.Factor, 2*bag.Factor, bag.Factor
				add(sp)
				space[sp] = true
			}
			for j := 0; j < 1+rng.Intn(6); j++ {
				g := NewGlyph()
				g.Width = bag.ScaledPoint(4+rng.Intn(3)) * bag.Factor
				g.Components = "a"
				add(g)
			}
		}
		head, _ = AppendLineEndAfter(head, cur)
		s := NewLinebreakSettings()
		s.HSize = bag.ScaledPoint(50+rng.Intn(100)) * bag.Factor
		vl, bps := Linebreak(head, s)

		planned := map[int]bag.ScaledPoint{}
		for _, bp := range bps {
			planned[bp.Line] = bp.Width
		}
		line := 0
		for n := vl.List; n != nil; n = n.Next() {
			hl, ok := n.(*HList)
			if !ok {
				continue
			}
			line++
			var natural bag.ScaledPoint
			for e := hl.List; e != nil; e = e.Next() {
				switch {
				case space[e]:
					natural += 3 * bag.Factor
				case e.Type() == TypeGlyph:
					natural += e.(*Glyph).Width
				}
			}
			if want := planned[line]; natural != want {
				t.Errorf("case %d (%s): line %d is set %s wide, planned %s", c, s.HSize, line, natural, want)
			}
		}
	}
}

// TestIndentAfterAHardBreakStays checks that the white space after a forced
// break starts the next line, as the indent of a line in preformatted text
// does, while the space after a break at a penalty is discarded, also behind
// a penalty that forbids a break there.
func TestIndentAfterAHardBreakStays(t *testing.T) {
	penalty := func(p int) func() Node {
		return func() Node { n := NewPenalty(); n.Penalty = p; return n }
	}
	for _, tc := range []struct {
		name    string
		brk     func() Node
		nobreak bool
		indent  bool
	}{
		{"HardBreak", func() Node { return NewHardBreak() }, false, true},
		{"forced penalty", forcedPenalty, false, true},
		{"penalty", penalty(-50), false, false},
		{"penalty and penalty 10000", penalty(-50), true, false},
	} {
		var head, cur Node
		head, cur = glyphRun(head, cur, "aaaa", 5*bag.Factor)
		brk := tc.brk()
		head = InsertAfter(head, cur, brk)
		cur = brk
		if tc.nobreak {
			nb := penalty(10000)()
			head = InsertAfter(head, cur, nb)
			cur = nb
		}
		sp := NewGlue()
		sp.Width = 6 * bag.Factor
		head = InsertAfter(head, cur, sp)
		cur = sp
		head, cur = glyphRun(head, cur, "bbbb", 5*bag.Factor)
		head, _ = AppendLineEndAfter(head, cur)
		// Both words fill the line exactly, neither line needs to stretch.
		s := NewLinebreakSettings()
		s.HSize = 20 * bag.Factor
		vl, _ := Linebreak(head, s)
		var lines []*HList
		for n := vl.List; n != nil; n = n.Next() {
			if hl, ok := n.(*HList); ok {
				lines = append(lines, hl)
			}
		}
		if len(lines) != 2 {
			t.Fatalf("%s: %d lines, want 2", tc.name, len(lines))
		}
		kept := false
		for e := lines[1].List; e != nil; e = e.Next() {
			if e == sp {
				kept = true
			}
		}
		if kept != tc.indent {
			t.Errorf("%s: space at the start of line 2 kept %t, want %t", tc.name, kept, tc.indent)
		}
	}
}
