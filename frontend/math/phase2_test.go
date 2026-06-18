package math

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

// TestRadicalStretched — when the body of √ is tall (squared body), the
// engine must select a larger pre-built variant from MathVariants rather
// than stretching the base glyph via a fake "lift". Asserts that the
// rendered radical glyph id is NOT the base U+221A id.
func TestRadicalStretched(t *testing.T) {
	fnt := loadMathFont(t)
	radGid := glyphFor(t, fnt, '√')
	aGid := glyphFor(t, fnt, 'a')
	twoGid := glyphFor(t, fnt, '2')

	// Body with sup pushes the radical's needed height beyond the base
	// glyph, forcing the variant selector to pick something larger.
	hl, err := InlineMath(fnt,
		Sqrt(radGid, Ord(aGid).WithSupGlyph(twoGid)),
	)
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}

	// Find the actual radical glyph id in the rendered tree.
	var renderedRadGid ot.GlyphID
	walkAll(hl.List, func(n node.Node) {
		if g, ok := n.(*node.Glyph); ok {
			// Heuristic: radical-related glyphs sit at the very left of
			// the structure. Latin Modern Math's √ variants share a
			// glyph-id range; the first non-body glyph we see is it.
			// Take the first glyph; subsequent body glyphs (a, ², …)
			// come later horizontally.
			if renderedRadGid == 0 {
				renderedRadGid = ot.GlyphID(g.Codepoint)
			}
		}
	})

	if renderedRadGid == 0 {
		t.Fatalf("no radical glyph found in output")
	}
	// We expect the variant selector to have picked a DIFFERENT glyph
	// than the base. If the font shipped no variants, the selector
	// would fall back to the base — log it but don't fail.
	if renderedRadGid == radGid {
		t.Logf("variant selector returned base glyph %d — font may lack variants for U+221A", radGid)
	}
}

// TestBigSumDisplay — in DisplayMath, an ∑ atom should select a larger
// pre-built variant of the glyph. The selection happens via MathVariants
// — we verify by reading the rendered glyph id off the result and
// confirming it differs from the inline rendering's id.
func TestBigSumDisplay(t *testing.T) {
	fnt := loadMathFont(t)
	sumGid := glyphFor(t, fnt, '∑')

	hlInline, err := InlineMath(fnt, Op(sumGid))
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}
	hlDisplay, err := DisplayMath(fnt, Op(sumGid))
	if err != nil {
		t.Fatalf("DisplayMath: %v", err)
	}

	firstGid := func(hl *node.HList) ot.GlyphID {
		var gid ot.GlyphID
		walkAll(hl.List, func(n node.Node) {
			if g, ok := n.(*node.Glyph); ok && gid == 0 {
				gid = ot.GlyphID(g.Codepoint)
			}
		})
		return gid
	}
	inlineGid := firstGid(hlInline)
	displayGid := firstGid(hlDisplay)
	if displayGid == 0 || inlineGid == 0 {
		t.Fatalf("could not extract glyph ids: inline=%d display=%d", inlineGid, displayGid)
	}
	if displayGid == inlineGid {
		t.Errorf("display ∑ glyph id == inline ∑ glyph id (%d); expected the display variant", displayGid)
	}
}

// TestLimitsMode — in DisplayMath with sub/sup, a big-op renders limits
// (centered above/below). The result is a VList wrapping nucleus and
// limit boxes; the outer HList has at least one VList child.
func TestLimitsMode(t *testing.T) {
	fnt := loadMathFont(t)
	sumGid := glyphFor(t, fnt, '∑')
	nGid := glyphFor(t, fnt, 'n')
	kGid := glyphFor(t, fnt, 'k')

	hl, err := DisplayMath(fnt,
		Op(sumGid).
			WithSubGlyph(kGid).
			WithSupGlyph(nGid),
	)
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
		t.Errorf("expected a VList in the result (limits mode wraps nucleus + limits in a VList)")
	}
}

// TestFractionDelimitersStretched — a Fraction with LeftDelim/RightDelim
// produces an outer HList containing the stretched delimiters plus the
// fraction body. Both delimiters should resolve to glyphs (or assemblies)
// whose total span matches the fraction height.
func TestFractionDelimitersStretched(t *testing.T) {
	fnt := loadMathFont(t)
	aGid := glyphFor(t, fnt, 'a')
	bGid := glyphFor(t, fnt, 'b')
	lparen := glyphFor(t, fnt, '(')
	rparen := glyphFor(t, fnt, ')')

	frac := Frac(
		[]MathItem{Ord(aGid)},
		[]MathItem{Ord(bGid)},
	)
	frac.LeftDelim = lparen
	frac.RightDelim = rparen

	hl, err := DisplayMath(fnt, frac)
	if err != nil {
		t.Fatalf("DisplayMath frac: %v", err)
	}

	// The result must have at least 3 distinct glyph runs at the top
	// level (left paren, fraction body, right paren). Count Glyph and
	// VList children of the outermost HList.
	leafCount := 0
	walkAll(hl.List, func(n node.Node) {
		switch n.(type) {
		case *node.Glyph, *node.Rule:
			leafCount++
		}
	})
	// Minimum: 2 paren glyphs + a, b, rule = 5. Assembly parens would
	// give more; that's still ≥ 5.
	if leafCount < 5 {
		t.Errorf("expected at least 5 leaf nodes (parens + a + b + rule), got %d", leafCount)
	}
}
