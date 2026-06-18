package math

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

// layoutFraction lowers a Fraction (numerator stacked over denominator with
// optional rule) to an HList. The HList carries a single VList child whose
// declared Height / Depth lift the rule to math-axis height in the outer
// baseline coordinate system — so a fraction lines up cleanly between two
// surrounding atoms with no further adjustment from the caller.
//
// Display vs. text style chooses different OT-MATH constants (deeper shifts
// and bigger gaps in display). Thickness < 0 means "binomial" / \atop —
// the rule is omitted and Stack-* constants are used instead of Fraction-*.
//
// Centering: numerator and denominator are wrapped in side-padding HLists
// so each one is exactly the common width — TeX traditionally uses
// HpackTo, but a kern-padding wrapper costs no allocation extras here and
// keeps glyph-walking simple (no glue settings to inspect).
//
// LeftDelim / RightDelim are phase-1-limited: if set, the base glyph is
// emitted at natural size with a warning. Stretching to body height is a
// phase-2 task (MathVariants).
func layoutFraction(f *Fraction, style MathStyle, ctx *engineCtx) *node.HList {
	fnt := ctx.at(style)
	c := ctx.cons
	size := fnt.Size
	upem := upemOf(fnt)

	// Lay out the numerator (one style down, cramping inherited) and the
	// denominator (one style down, always cramped).
	num := mlistToHlist(f.Num, numStyle(style), ctx)
	den := mlistToHlist(f.Den, denStyle(style), ctx)

	// Choose the constant set: display style triggers the bigger sibling.
	// TeX convention: a fraction wrapped in \left ... \right delimiters
	// uses the text-style shift constants even when the surrounding math
	// is in display style. The delimiters still stretch to the bigger
	// fraction-body height, but the numerator and denominator sit
	// closer to the rule — the visual difference between `\frac{a}{b}`
	// (loose, lots of space above 'a') and `\left(\frac{a}{b}\right)`
	// (tight, 'a' nearly touches the rule), which is what LaTeX users
	// expect.
	useDisplayShifts := style.IsDisplay() && f.LeftDelim == 0 && f.RightDelim == 0
	var shiftUpConst, shiftDownConst, numGapConst, denomGapConst int16
	if useDisplayShifts {
		shiftUpConst = c.FractionNumeratorDisplayStyleShiftUp
		shiftDownConst = c.FractionDenominatorDisplayStyleShiftDown
		numGapConst = c.FractionNumDisplayStyleGapMin
		denomGapConst = c.FractionDenomDisplayStyleGapMin
	} else {
		shiftUpConst = c.FractionNumeratorShiftUp
		shiftDownConst = c.FractionDenominatorShiftDown
		numGapConst = c.FractionNumeratorGapMin
		denomGapConst = c.FractionDenominatorGapMin
	}

	shiftUp := fuToSP(shiftUpConst, size, upem)
	shiftDown := fuToSP(shiftDownConst, size, upem)
	numGapMin := fuToSP(numGapConst, size, upem)
	denomGapMin := fuToSP(denomGapConst, size, upem)
	axisHeight := fuToSP(c.AxisHeight, size, upem)

	// Rule thickness: caller override > font default > 0 for binomial.
	binomial := f.Thickness < 0
	var thickness bag.ScaledPoint
	if !binomial {
		thickness = fuToSP(c.FractionRuleThickness, size, upem)
		if f.Thickness > 0 {
			thickness = bag.ScaledPoint(f.Thickness)
		}
	}
	halfRule := thickness / 2

	if binomial {
		// Stack constants instead of Fraction constants. No rule.
		var stackShiftUp, stackShiftDown, stackGap int16
		if style.IsDisplay() {
			stackShiftUp = c.StackTopDisplayStyleShiftUp
			stackShiftDown = c.StackBottomDisplayStyleShiftDown
			stackGap = c.StackDisplayStyleGapMin
		} else {
			stackShiftUp = c.StackTopShiftUp
			stackShiftDown = c.StackBottomShiftDown
			stackGap = c.StackGapMin
		}
		shiftUp = fuToSP(stackShiftUp, size, upem)
		shiftDown = fuToSP(stackShiftDown, size, upem)
		// Enforce minimum stack gap.
		gap := (shiftUp - num.Depth) - (-shiftDown + den.Height)
		gapMin := fuToSP(stackGap, size, upem)
		if gap < gapMin {
			missing := gapMin - gap
			shiftUp += missing / 2
			shiftDown += missing - missing/2
		}
	} else {
		// Numerator gap: distance from num.bottom to rule.top, both
		// measured above the baseline.
		gap1 := (shiftUp - num.Depth) - (axisHeight + halfRule)
		if gap1 < numGapMin {
			shiftUp += numGapMin - gap1
		}
		// Denominator gap: distance from rule.bottom to den.top.
		gap2 := (axisHeight - halfRule) - (-shiftDown + den.Height)
		if gap2 < denomGapMin {
			shiftDown += denomGapMin - gap2
		}
	}

	// Common width — pad each side so both boxes are exactly W wide.
	W := num.Width
	if den.Width > W {
		W = den.Width
	}
	numCentered := centerInHList(num, W)
	denCentered := centerInHList(den, W)

	// Build the VList top-to-bottom. Internal gaps:
	//   gap_above_rule = (shift_up - num.depth) - (axis + half_rule)
	//   gap_below_rule = (axis - half_rule) - (-shift_down + den.height)
	gapAbove := (shiftUp - num.Depth) - (axisHeight + halfRule)
	gapBelow := (axisHeight - halfRule) - (-shiftDown + den.Height)
	if binomial {
		// One single gap, split evenly above & below the (absent) rule's
		// implicit center on axisHeight. The "above" and "below" gaps still
		// have to sum to the total gap between num.bottom and den.top.
		totalGap := (shiftUp - num.Depth) - (-shiftDown + den.Height)
		gapAbove = totalGap / 2
		gapBelow = totalGap - gapAbove
	}
	if gapAbove < 0 {
		gapAbove = 0
	}
	if gapBelow < 0 {
		gapBelow = 0
	}

	var head, tail node.Node
	head, tail = appendNode(head, tail, numCentered)
	if gapAbove > 0 {
		k := node.NewKern()
		k.Kern = gapAbove
		head, tail = appendNode(head, tail, k)
	}
	if !binomial && thickness > 0 {
		r := node.NewRule()
		r.Width = W
		r.Height = thickness
		r.Depth = 0
		head, tail = appendNode(head, tail, r)
	}
	if gapBelow > 0 {
		k := node.NewKern()
		k.Kern = gapBelow
		head, tail = appendNode(head, tail, k)
	}
	head, tail = appendNode(head, tail, denCentered)
	_ = tail

	vl := node.NewVList()
	vl.List = head
	vl.Width = W
	vl.Height = shiftUp + num.Height
	vl.Depth = shiftDown + den.Depth

	// Wrap the VList in a one-child HList so the outer engine can splice
	// it into the inter-atom-spacing pass like any other item.
	hl := node.NewHList()
	hl.List = vl
	hl.Width = vl.Width
	hl.Height = vl.Height
	hl.Depth = vl.Depth

	// Delimiters (phase 2): stretch via MathVariants/GlyphAssembly to
	// the fraction's total height, then center each delimiter on the
	// math axis so they line up with the rule.
	if f.LeftDelim == 0 && f.RightDelim == 0 {
		return hl
	}
	return wrapFractionDelimiters(f, hl, style, ctx, size, upem)
}

// wrapFractionDelimiters builds an outer HList of the form
// [leftDelim] [hl] [rightDelim].
//
// Style-aware sizing:
//   - InlineMath (TextStyle): use the BASE glyph at natural size, no
//     stretching. TeX/LuaTeX's `\left(\right)` inside `\(...\)` does the
//     same — the smallest variant in the font is already deliberately
//     bigger than the base, so picking it gives oversized parens for
//     an inline fraction.
//   - DisplayMath: stretch via MathVariants/GlyphAssembly to match the
//     fraction's total span, with DelimitedSubFormulaMinHeight as a
//     lower bound. This ensures parens never look "too small" for a
//     display fraction — even one with a single 'a' and 'b' inside.
//
// Both branches center each delim on the math axis so the rule sits at
// the bracket's mid-line.
func wrapFractionDelimiters(f *Fraction, hl *node.HList, style MathStyle, ctx *engineCtx, size bag.ScaledPoint, upem int) *node.HList {
	c := ctx.cons
	axis := fuToSP(c.AxisHeight, size, upem)

	makeDelim := func(gid ot.GlyphID) *node.HList {
		var delim *node.HList
		if style.IsDisplay() {
			// Display: stretch with DelimitedSubFormulaMinHeight floor.
			requiredSP := hl.Height + hl.Depth
			minSP := bag.ScaledPoint(int64(c.DelimitedSubFormulaMinHeight) * int64(size) / int64(upem))
			if minSP > requiredSP {
				requiredSP = minSP
			}
			requiredFU := uint16(int64(requiredSP) * int64(upem) / int64(size))
			if requiredFU == 0 {
				requiredFU = 1
			}
			delim = stretchedVertical(ctx, gid, requiredFU, style)
		} else {
			// Inline: just emit the base glyph, no stretching.
			delim = wrapGlyphInHList(buildGlyph(ctx, gid, style))
		}
		if delim.Height == 0 && delim.Depth == 0 {
			return delim
		}
		// Center the delimiter on the math axis. With real glyph
		// extents in place, the glyph's vertical mid-point is at
		// (Height − Depth)/2 above the baseline — NOT at Height/2,
		// which only held when we treated all glyphs as
		// baseline-anchored (the phase-2 convention before df7c4b75
		// brought in real CFF extents). Using Height/2 made the
		// centered delimiter sit roughly half its depth too low —
		// visible as parens hanging below 'a'/'b' in the demo's
		// (a/b) render.
		currentCenter := (delim.Height - delim.Depth) / 2
		shift := axis - currentCenter
		delim.Shift = shift
		delim.Height += shift
		delim.Depth -= shift
		if delim.Depth < 0 {
			delim.Depth = 0
		}
		if delim.Height < 0 {
			delim.Height = 0
		}
		return delim
	}

	out := node.NewHList()
	var oh, ot node.Node
	width := bag.ScaledPoint(0)
	height := hl.Height
	depth := hl.Depth
	if f.LeftDelim != 0 {
		d := makeDelim(f.LeftDelim)
		oh, ot = appendNode(oh, ot, d)
		width += d.Width
		if d.Height > height {
			height = d.Height
		}
		if d.Depth > depth {
			depth = d.Depth
		}
	}
	oh, ot = appendNode(oh, ot, hl)
	width += hl.Width
	if f.RightDelim != 0 {
		d := makeDelim(f.RightDelim)
		oh, ot = appendNode(oh, ot, d)
		width += d.Width
		if d.Height > height {
			height = d.Height
		}
		if d.Depth > depth {
			depth = d.Depth
		}
	}
	_ = ot
	out.List = oh
	out.Width = width
	out.Height = height
	out.Depth = depth
	return out
}

// centerInHList wraps an HList between two kerns so it is exactly W wide
// with its content visually centered. If the content is already at least W,
// it is returned unchanged.
//
// Returns a fresh HList — the caller's content node is reused but its
// `next`/`prev` pointers are mutated, so the caller must not hold on to the
// content as a standalone node afterwards.
func centerInHList(content *node.HList, W bag.ScaledPoint) *node.HList {
	if content.Width >= W {
		return content
	}
	pad := W - content.Width
	leftPad := pad / 2
	rightPad := pad - leftPad

	out := node.NewHList()
	var head, tail node.Node
	if leftPad > 0 {
		k := node.NewKern()
		k.Kern = leftPad
		head, tail = appendNode(head, tail, k)
	}
	head, tail = appendNode(head, tail, content)
	if rightPad > 0 {
		k := node.NewKern()
		k.Kern = rightPad
		head, tail = appendNode(head, tail, k)
	}
	_ = tail
	out.List = head
	out.Width = W
	out.Height = content.Height
	out.Depth = content.Depth
	return out
}
