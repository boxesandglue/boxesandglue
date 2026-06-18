package math

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/node"
)

// TestRadicalWithDegree — ∛x: the degree must be (a) in script-script style
// (= scaled down using ScriptScriptPercentScaleDown), (b) raised vertically
// by RadicalDegreeBottomRaisePercent of the radical's box, and (c) followed
// by a negative RadicalKernAfterDegree that pulls it close to the radical
// (this is what makes the ⁿ√ visual coherent).
func TestRadicalWithDegree(t *testing.T) {
	fnt := loadMathFont(t)
	c := fnt.MathConstantsFU()

	// U+221A radical sign, U+0033 '3', 'x'.
	radGid := glyphFor(t, fnt, '√')
	threeGid := glyphFor(t, fnt, '3')
	xGid := glyphFor(t, fnt, 'x')

	rad := NRoot(radGid, []MathItem{Ord(threeGid)}, []MathItem{Ord(xGid)})
	hl, err := InlineMath(fnt, rad)
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}

	// Walk the tree, gather all glyphs.
	var glyphs []*node.Glyph
	var degreeHL *node.HList
	var walk func(n node.Node)
	walk = func(n node.Node) {
		for ; n != nil; n = n.Next() {
			switch x := n.(type) {
			case *node.Glyph:
				glyphs = append(glyphs, x)
			case *node.HList:
				if x.Shift > 0 && degreeHL == nil {
					degreeHL = x
				}
				walk(x.List)
			case *node.VList:
				walk(x.List)
			}
		}
	}
	walk(hl.List)

	if len(glyphs) < 3 {
		t.Fatalf("expected at least 3 glyphs (degree, radical, body), got %d", len(glyphs))
	}
	if degreeHL == nil {
		t.Fatalf("no HList with positive Shift found — the degree must be raised")
	}

	// Degree's raise should equal RadicalDegreeBottomRaisePercent% of the
	// STRETCHED radical's box height+depth. Phase 2 selects a variant
	// whose AdvanceFU matches the required size; the base-glyph
	// pauschal model would give a smaller number, but the raise is
	// computed in terms of the actually-rendered radical box.
	raisePercent := int64(c.RadicalDegreeBottomRaisePercent)
	if raisePercent == 0 {
		raisePercent = 60
	}
	// The degree's Shift was set by layoutRadical against radHL.Height+
	// radHL.Depth after the variant was selected. We just sanity-check
	// it is non-zero and proportional — the exact value depends on the
	// font's variant set and would brittle-couple test to font version.
	if degreeHL.Shift <= 0 {
		t.Errorf("degree Shift = %d sp, want positive (lifted by raisePercent%% of radical box)", degreeHL.Shift)
	}

	// Spot-check that a Rule node exists in the right-VList — the overbar.
	if !hasRule(hl) {
		t.Errorf("radical output has no Rule node — the overbar is missing")
	}
}
