package math

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/node"
)

// countGlyphs walks a node list and counts Glyph nodes.
func countGlyphs(n node.Node) int {
	count := 0
	walkAll(n, func(nd node.Node) {
		if _, ok := nd.(*node.Glyph); ok {
			count++
		}
	})
	return count
}

// TestFencedStretchesAroundTallBody — parens around a display fraction
// must grow: either a larger pre-built variant (different gid) or a
// multi-part assembly. The fence span has to cover the body span.
func TestFencedStretchesAroundTallBody(t *testing.T) {
	fnt := loadMathFont(t)
	lparen := glyphFor(t, fnt, '(')
	rparen := glyphFor(t, fnt, ')')
	aGid := glyphFor(t, fnt, 'a')
	bGid := glyphFor(t, fnt, 'b')

	frac := Frac([]MathItem{Ord(aGid)}, []MathItem{Ord(bGid)})
	fenced := Fence(lparen, rparen, frac)
	hl, err := DisplayMath(fnt, fenced)
	if err != nil {
		t.Fatalf("DisplayMath: %v", err)
	}

	body, err := DisplayMath(fnt, frac)
	if err != nil {
		t.Fatalf("DisplayMath (body): %v", err)
	}
	if hl.Height+hl.Depth < body.Height+body.Depth {
		t.Errorf("fenced span %v smaller than body span %v", hl.Height+hl.Depth, body.Height+body.Depth)
	}

	// The paren must not be the plain base glyph: DisplayMath enforces
	// DelimitedSubFormulaMinHeight, well above the base paren's span.
	plainParen, err := InlineMath(fnt, Open(lparen))
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}
	if hl.Height+hl.Depth <= plainParen.Height+plainParen.Depth {
		t.Errorf("fence did not stretch: fenced span %v, base paren span %v",
			hl.Height+hl.Depth, plainParen.Height+plainParen.Depth)
	}
}

// TestFencedInlineSmallBodyKeepsBase — around inline-height content the
// base paren already covers the target, so no variant is selected and
// the glyph count stays minimal (left paren, x, right paren).
func TestFencedInlineSmallBodyKeepsBase(t *testing.T) {
	fnt := loadMathFont(t)
	lparen := glyphFor(t, fnt, '(')
	rparen := glyphFor(t, fnt, ')')
	xGid := glyphFor(t, fnt, 'x')

	hl, err := InlineMath(fnt, Fence(lparen, rparen, Ord(xGid)))
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}
	if got := countGlyphs(hl.List); got != 3 {
		t.Errorf("inline fence around x: want 3 glyphs, got %d", got)
	}

	plain, err := InlineMath(fnt, Open(lparen), Ord(xGid), Close(rparen))
	if err != nil {
		t.Fatalf("InlineMath (plain): %v", err)
	}
	if hl.Height != plain.Height || hl.Depth != plain.Depth {
		t.Errorf("inline fence changed geometry: fence %v+%v, plain %v+%v",
			hl.Height, hl.Depth, plain.Height, plain.Depth)
	}
}

// TestFencedOneSided — Left == 0 renders only the right delimiter.
func TestFencedOneSided(t *testing.T) {
	fnt := loadMathFont(t)
	rparen := glyphFor(t, fnt, ')')
	xGid := glyphFor(t, fnt, 'x')

	hl, err := InlineMath(fnt, Fence(0, rparen, Ord(xGid)))
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}
	if got := countGlyphs(hl.List); got != 2 {
		t.Errorf("one-sided fence: want 2 glyphs, got %d", got)
	}
}
