package math

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
)

// fuToSP converts a raw font-unit value (an int16 from the MATH table) to
// scaled points at the given style's body size. Uses int64 arithmetic so the
// intermediate product never overflows for sane fonts (UPEM ≤ 16384, size in
// ScaledPoint = ≈ 1e7 for 100pt → product ≈ 1.6e11, fits comfortably).
//
// The font's units-per-em is read via f.Face.UnitsPerEM (an int set at face
// construction). For style-scaled values, the caller passes the scaled body
// size from scaledSize().
func fuToSP(fu int16, size bag.ScaledPoint, upem int) bag.ScaledPoint {
	if upem == 0 || fu == 0 {
		return 0
	}
	return bag.ScaledPoint(int64(fu) * int64(size) / int64(upem))
}

// mu computes 1 math-unit in scaled points at the given body size: by TeX
// convention 1 mu = 1/18 em. The em is the unscaled body size, NOT the
// style-scaled size — TeX's `\thinmuskip` etc. use the surrounding text size,
// which for math means the math-list's nominal Size (= the font's Size in
// our setup, since the user picks one math font instance per list).
func mu(size bag.ScaledPoint) bag.ScaledPoint {
	return size / 18
}

// upemOf is a small accessor that hides the lookup chain
// f.Face.UnitsPerEM. Returns 0 for a nil font — callers treat the zero
// from fuToSP as "use 0 SP" without panicking.
func upemOf(f *font.Font) int {
	if f == nil || f.Face == nil {
		return 0
	}
	return int(f.Face.UnitsPerEM)
}
