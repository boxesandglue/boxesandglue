package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// TeX takes a line whose badness is at most \tolerance, in both directions:
// with a ratio for a tolerance, a line may stretch and shrink by at most the
// tolerance, and the tolerance itself is still feasible.
func TestFeasibleIsSymmetricAndInclusive(t *testing.T) {
	for _, tc := range []struct {
		tolerance, r float64
		want         bool
	}{
		{4, 4, true},
		{4, 4.01, false},
		{4, -1, true},
		{4, -1.01, false},
		{1.26, 1.26, true},
		{1.26, -1, true},
		{0.5, 0.5, true},
		{0.5, -0.5, true},
		{0.5, -0.75, false},
		{0, 0, true},
		{0, 0.01, false},
		{0, -0.01, false},
	} {
		lb := newLinebreaker(&LinebreakSettings{Tolerance: tc.tolerance})
		if got := lb.feasible(tc.r); got != tc.want {
			t.Errorf("Tolerance %g, r %g: feasible %t, want %t", tc.tolerance, tc.r, got, tc.want)
		}
	}
}

// A line that fits its measure exactly is feasible at Tolerance 0, it does not
// become an emergency line.
func TestExactFitAtToleranceZero(t *testing.T) {
	// Three words of 20pt with 6pt spaces are 72pt, exactly the measure.
	head := buildWords([]string{"aa", "aa", "aa", "aa"}, 10*bag.Factor, 6*bag.Factor)
	s := NewLinebreakSettings()
	s.HSize = 72 * bag.Factor
	s.Tolerance = 0
	_, bps := Linebreak(head, s)
	if len(bps) != 2 {
		t.Fatalf("got %d lines, want 2", len(bps))
	}
	// An emergency line costs 1000 demerits.
	if bps[0].R != 0 || bps[0].Demerits >= 1000 {
		t.Errorf("first line: r %g, demerits %d, want an exact fit that is not an emergency line", bps[0].R, bps[0].Demerits)
	}
}
