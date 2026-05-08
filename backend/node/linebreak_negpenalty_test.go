package node

import (
	"math"
	"testing"
)

// TestNegativeHyphenPenaltyProducesFiniteDemerits verifies the bugfix
// for the naive `if demerits < 0 { demerits = math.MaxInt }` overflow
// guard in calculateDemerits.
//
// Knuth-Plass §4 makes negative penalties (e.g. Hyphenpenalty < 0 to
// favour hyphenation) lower demerits via `(1+b)² − p²`, which can
// legitimately go negative. The previous code treated *every* negative
// result as an overflow and clamped to MaxInt, silently inverting the
// intended bonus into the worst possible penalty. After the fix only
// true sign-flipping additions are clamped.
//
// We exercise calculateDemerits directly with a synthetic active node so
// the test is independent of any Linebreak feasibility concerns. We do
// require a Disc node to flip the curpenalty branch onto the
// Hyphenpenalty path.
func TestNegativeHyphenPenaltyProducesFiniteDemerits(t *testing.T) {
	disc := NewDisc()

	makeLB := func(hp int) (*linebreaker, *Breakpoint) {
		ls := NewLinebreakSettings()
		ls.Hyphenpenalty = hp
		lb := newLinebreaker(ls)
		// Synthetic active node with non-zero accumulated demerits so we
		// can also exercise the addition step.
		active := &Breakpoint{
			Fitness:  1,
			Demerits: 5000,
		}
		return lb, active
	}

	// r = 0.5 → finite badness ≈ 12.5, (1+b)² ≈ 182. With p² subtracted
	// at hp = -1000, raw demerits ≈ 182 − 1_000_000 ≈ -999_818, then
	// + active.Demerits (5000) ≈ -994_818. Pre-fix: clamped to MaxInt.
	const r = 0.5

	lbPos, aPos := makeLB(10000)
	_, dPos := lbPos.calculateDemerits(aPos, r, disc)

	lbNeg, aNeg := makeLB(-1000)
	_, dNeg := lbNeg.calculateDemerits(aNeg, r, disc)

	// Sanity: positive penalty produces large positive demerits.
	if dPos <= 0 {
		t.Fatalf("expected positive demerits with hp=+10000, got %d", dPos)
	}
	// Sanity: negative penalty produces *smaller* demerits than positive.
	if dNeg >= dPos {
		t.Errorf("expected hp=-1000 demerits < hp=+10000 demerits, got dNeg=%d, dPos=%d", dNeg, dPos)
	}
	// Pre-fix the result was always clamped to MaxInt for negative hp.
	// Make that the canonical regression assertion.
	if dNeg == math.MaxInt {
		t.Errorf("demerits=%d == MaxInt — looks like the pre-fix clamping returned. Negative hyphenpenalty should produce a finite (possibly negative) bonus, not the worst-possible MaxInt sentinel.", dNeg)
	}
	// Specifically: the bonus should be large enough that demerits go
	// well into the negative range. (1+b)² for r=0.5 is small; -1000
	// squared (1_000_000) dominates by orders of magnitude.
	if dNeg > 0 {
		t.Errorf("expected negative total demerits with hp=-1000 dominating (1+b)², got dNeg=%d", dNeg)
	}
}

// TestPositiveOverflowStillClampsToMaxInt verifies the signed-overflow
// rule did not regress the original protection: if both the candidate
// demerits and the active path's accumulated demerits are positive but
// their int sum overflows (wraps negative), we must still clamp to
// MaxInt rather than let a corrupted negative result leak through.
//
// We synthesise the inputs directly because going through a real
// linebreak run cannot reliably push through MaxInt.
func TestPositiveOverflowStillClampsToMaxInt(t *testing.T) {
	ls := NewLinebreakSettings()
	lb := newLinebreaker(ls)

	// candidate demerits at this break (small positive — comes back
	// from (1+b)² for a r=0.5 line).
	active := &Breakpoint{
		Fitness:  1,
		Demerits: math.MaxInt - 100,
	}
	// A non-Disc, non-Penalty trigger so curpenalty=0 and demerits stays
	// at (1+b)². Using a Glue keeps the demerits formula simple and
	// positive; the active.Demerits-near-MaxInt is what causes the
	// addition to overflow.
	g := NewGlue()
	_, d := lb.calculateDemerits(active, 0.5, g)
	if d != math.MaxInt {
		t.Errorf("expected demerits clamped to MaxInt on overflow, got %d (active was MaxInt-100, candidate was small positive)", d)
	}
}
