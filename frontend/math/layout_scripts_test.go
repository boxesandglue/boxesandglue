package math

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/node"
)

// TestScriptsLimitsOnOrdBase — an Ord atom whose nucleus is a sublist
// ("lim") stacks its subscript below when ScriptsLimits is requested
// (MathML munder semantics), instead of rendering a side subscript.
func TestScriptsLimitsOnOrdBase(t *testing.T) {
	fnt := loadMathFont(t)
	lGid := glyphFor(t, fnt, 'l')
	iGid := glyphFor(t, fnt, 'i')
	mGid := glyphFor(t, fnt, 'm')
	nGid := glyphFor(t, fnt, 'n')

	base := &MathAtom{
		Class:   ClassOrd,
		Scripts: ScriptsLimits,
		Nucleus: MathField{Sublist: []MathItem{Ord(lGid), Ord(iGid), Ord(mGid)}},
		Sub:     MathField{Glyph: nGid},
	}
	hl, err := DisplayMath(fnt, base)
	if err != nil {
		t.Fatalf("DisplayMath: %v", err)
	}
	hasVList := false
	walkAll(hl.List, func(n node.Node) {
		if _, ok := n.(*node.VList); ok {
			hasVList = true
		}
	})
	if !hasVList {
		t.Errorf("ScriptsLimits on an Ord base must stack the script in a VList")
	}

	// The stacked box must be deeper than the plain nucleus: the limit
	// hangs below the baseline.
	plain, err := DisplayMath(fnt, Ord(lGid), Ord(iGid), Ord(mGid))
	if err != nil {
		t.Fatalf("DisplayMath (plain): %v", err)
	}
	if hl.Depth <= plain.Depth {
		t.Errorf("stacked limit should increase depth: got %v, plain %v", hl.Depth, plain.Depth)
	}
}

// TestScriptsSideOnDisplayBigOp — an integral in display style with
// ScriptsSide renders its scripts to the right (no limit VList), while
// still using the enlarged display variant of the ∫ glyph.
func TestScriptsSideOnDisplayBigOp(t *testing.T) {
	fnt := loadMathFont(t)
	intGid := glyphFor(t, fnt, '∫')
	aGid := glyphFor(t, fnt, 'a')
	bGid := glyphFor(t, fnt, 'b')

	side, err := DisplayMath(fnt,
		Op(intGid).WithScripts(ScriptsSide).WithSubGlyph(aGid).WithSupGlyph(bGid))
	if err != nil {
		t.Fatalf("DisplayMath: %v", err)
	}
	walkAll(side.List, func(n node.Node) {
		if _, ok := n.(*node.VList); ok {
			t.Errorf("ScriptsSide must not stack limits in a VList")
		}
	})

	// Auto placement stacks; the side variant must be wider (scripts sit
	// to the right of the operator instead of centered above/below).
	auto, err := DisplayMath(fnt,
		Op(intGid).WithSubGlyph(aGid).WithSupGlyph(bGid))
	if err != nil {
		t.Fatalf("DisplayMath (auto): %v", err)
	}
	if side.Width <= auto.Width {
		t.Errorf("side scripts should be wider than stacked limits: side %v, auto %v", side.Width, auto.Width)
	}

	// The display variant of the operator must still be used: the side
	// layout is as tall as the auto (stacked) nucleus core.
	plain, err := InlineMath(fnt, Op(intGid))
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}
	if side.Height+side.Depth <= plain.Height+plain.Depth {
		t.Errorf("display ∫ with side scripts should keep the enlarged variant: side span %v, inline span %v",
			side.Height+side.Depth, plain.Height+plain.Depth)
	}
}
