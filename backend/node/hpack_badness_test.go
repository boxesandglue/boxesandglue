package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// boxAndGlue packs content, a node of width wd, and a glue g to 100pt.
func boxAndGlue(content Node, g *Glue, opts ...HpackOption) *HList {
	InsertAfter(content, content, g)
	return HpackToWithEnd(content, g, bag.MustSP("100pt"), opts...)
}

// ruleOf is a rule of width wd.
func ruleOf(wd string) *Rule {
	r := NewRule()
	r.Width = bag.MustSP(wd)
	return r
}

// glueOf is a glue of width wd that stretches or shrinks by 1pt in order.
func glueOf(wd string, stretch, shrink GlueOrder) *Glue {
	g := NewGlue()
	g.Width = bag.MustSP(wd)
	g.Stretch, g.StretchOrder = bag.Factor, stretch
	g.Shrink, g.ShrinkOrder = bag.Factor, shrink
	return g
}

// A box set with infinite stretch has badness 0, as in TeX, where only glue
// of order normal has a badness: the last line of a paragraph, whose
// parfillskip stretches infinitely, is not rated 10000 (boxesandglue#94).
func TestHpackInfiniteStretchHasNoBadness(t *testing.T) {
	g := glueOf("0pt", StretchFil, StretchNormal)
	hl := boxAndGlue(ruleOf("50pt"), g)
	if hl.Badness != 0 {
		t.Errorf("badness %d, want 0", hl.Badness)
	}
	if want := bag.MustSP("50pt"); g.Width != want {
		t.Errorf("the glue is %s wide, want %s", g.Width, want)
	}
}

// Infinite shrink takes up any excess, so the box is neither overfull nor
// rated, with or without font expansion: the glue shrinks by the whole
// excess, and no glyph is narrowed (boxesandglue#94).
func TestHpackInfiniteShrinkTakesUpAnyExcess(t *testing.T) {
	for _, opts := range [][]HpackOption{nil, {FontExpansion(0.05)}} {
		g := glueOf("10pt", StretchNormal, StretchFil)
		hl := boxAndGlue(ruleOf("150pt"), g, opts...)
		if hl.Badness != 0 {
			t.Errorf("expansion %t: badness %d, want 0", opts != nil, hl.Badness)
		}
		if want := bag.MustSP("-50pt"); g.Width != want {
			t.Errorf("expansion %t: the glue is %s wide, want %s", opts != nil, g.Width, want)
		}
		if hl.GlueSet != -60 || hl.GlueOrder != StretchFil {
			t.Errorf("expansion %t: glue set %v of order %d, want -60 of order %d", opts != nil, hl.GlueSet, hl.GlueOrder, StretchFil)
		}
		if _, ok := hl.Attributes["expand"]; ok {
			t.Errorf("expansion %t: the box narrows its glyphs", opts != nil)
		}
	}
}

// Glue of order normal is rated as before: an underfull box 10000, an
// overfull one 1000000, whose glue keeps its width unless the box may be
// squeezed.
func TestHpackNormalGlueKeepsItsBadness(t *testing.T) {
	if hl := boxAndGlue(ruleOf("50pt"), glueOf("0pt", StretchNormal, StretchNormal)); hl.Badness != 10000 {
		t.Errorf("underfull: badness %d, want 10000", hl.Badness)
	}
	for _, squeeze := range []bool{false, true} {
		g := glueOf("10pt", StretchNormal, StretchNormal)
		hl := boxAndGlue(ruleOf("150pt"), g, SqueezeOverfullBoxes(squeeze))
		if hl.Badness != 1000000 {
			t.Errorf("overfull, squeeze %t: badness %d, want 1000000", squeeze, hl.Badness)
		}
		want := bag.MustSP("10pt")
		if squeeze {
			want = bag.MustSP("-50pt")
		}
		if g.Width != want {
			t.Errorf("overfull, squeeze %t: the glue is %s wide, want %s", squeeze, g.Width, want)
		}
	}
}

// With font expansion an overfull box shrinks its glue by all it can and
// narrows the glyphs for the rest.
func TestHpackExpansionShrinksTheGlueFirst(t *testing.T) {
	gl := NewGlyph()
	gl.Width = bag.MustSP("100pt")
	g := NewGlue()
	g.Width = bag.MustSP("10pt")
	g.Shrink = bag.MustSP("4pt")
	hl := boxAndGlue(gl, g, FontExpansion(0.05))
	if want := bag.MustSP("6pt"); g.Width != want {
		t.Errorf("the glue is %s wide, want %s", g.Width, want)
	}
	if _, ok := hl.Attributes["expand"]; !ok {
		t.Errorf("the box does not narrow its glyphs")
	}
}
