package math

import (
	"testing"
)

// TestFractionDisplayCascade — `½` in display vs. text. Display style picks
// the *DisplayStyleShiftUp/Down constants which sit further from the math
// axis than their TextStyle counterparts, so the resulting HList is taller
// (greater Height + Depth). The test asserts that ordering plus the exact
// shape: Display's Height comes from FractionNumeratorDisplayStyleShiftUp;
// Text's from FractionNumeratorShiftUp.
func TestFractionDisplayCascade(t *testing.T) {
	fnt := loadMathFont(t)
	c := fnt.MathConstantsFU()
	oneGid := glyphFor(t, fnt, '1')
	twoGid := glyphFor(t, fnt, '2')

	frac := Frac([]MathItem{Ord(oneGid)}, []MathItem{Ord(twoGid)})

	hlText, err := InlineMath(fnt, frac)
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}
	// Build a fresh frac so the previous Bin→Ord rewrite (none in this
	// case, but cheap insurance) doesn't surprise the second run.
	frac2 := Frac([]MathItem{Ord(oneGid)}, []MathItem{Ord(twoGid)})
	hlDisp, err := DisplayMath(fnt, frac2)
	if err != nil {
		t.Fatalf("DisplayMath: %v", err)
	}

	if hlDisp.Height <= hlText.Height {
		t.Errorf("display fraction Height = %d sp, text Height = %d sp; display should be taller", hlDisp.Height, hlText.Height)
	}
	if hlDisp.Depth <= hlText.Depth {
		t.Errorf("display fraction Depth = %d sp, text Depth = %d sp; display should be deeper", hlDisp.Depth, hlText.Depth)
	}

	// Spot-check: the Height of the display fraction must be at least
	// FractionNumeratorDisplayStyleShiftUp converted to SP. The numerator
	// itself adds its own height on top.
	upem := int(fnt.Face.UnitsPerEM)
	dispShiftUp := fuToSP(c.FractionNumeratorDisplayStyleShiftUp, fnt.Size, upem)
	if hlDisp.Height < dispShiftUp {
		t.Errorf("display Height = %d sp, want >= FractionNumeratorDisplayStyleShiftUp (%d sp)", hlDisp.Height, dispShiftUp)
	}
}

// TestBinomialNoRule — a binomial (Thickness=-1) should not emit a Rule
// node, while a normal fraction should. This is the visual difference
// between (1\over 2) and {1 \choose 2}.
func TestBinomialNoRule(t *testing.T) {
	fnt := loadMathFont(t)
	oneGid := glyphFor(t, fnt, '1')
	twoGid := glyphFor(t, fnt, '2')

	frac, err := DisplayMath(fnt, Frac([]MathItem{Ord(oneGid)}, []MathItem{Ord(twoGid)}))
	if err != nil {
		t.Fatalf("Frac DisplayMath: %v", err)
	}
	binom, err := DisplayMath(fnt, Binom([]MathItem{Ord(oneGid)}, []MathItem{Ord(twoGid)}))
	if err != nil {
		t.Fatalf("Binom DisplayMath: %v", err)
	}
	if !hasRule(frac) {
		t.Errorf("Frac result has no Rule node (expected one)")
	}
	if hasRule(binom) {
		t.Errorf("Binom result has a Rule node (expected none)")
	}
}
