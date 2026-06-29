package math

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

// layoutAtom lowers a MathAtom (nucleus + optional sub/sup) to an HList.
// The nucleus is rendered at the caller's style; sub uses subStyle(),
// sup uses supStyle(). If neither sub nor sup is set, the nucleus is
// returned directly with no further packaging.
//
// Phase-2 big-op special case: when an Op atom is in display style and
// its nucleus is a single glyph, we ask the font for the larger
// "display variant" of that glyph via MathVariants — that's what makes
// ∑, ∫, ⋃ etc. visually grow in `\sum_{i=0}^{n}` versus a normal `\sum`
// inline. The variant is sized at DisplayOperatorMinHeight and shifted
// to sit centered on the math axis so sub/sup-script placement still
// works with the existing Atom-Geometry code.
func layoutAtom(a *MathAtom, style MathStyle, ctx *engineCtx) *node.HList {
	isDisplayBigOp := a.Class == ClassOp && style.IsDisplay() && a.Nucleus.Glyph != 0
	var nuc *node.HList
	if isDisplayBigOp {
		nuc = displayOpNucleus(a.Nucleus.Glyph, style, ctx)
	} else {
		nuc = layoutField(a.Nucleus, style, ctx)
	}
	if a.Sub.IsEmpty() && a.Sup.IsEmpty() {
		return nuc
	}
	// Big-op in display style: render sub/sup as LIMITS (centered above
	// and below the operator), not as scripts (to the right). This is
	// the visual convention `\sum_{i=0}^{n}` shows in LaTeX: i=0 below
	// the ∑ and n above, both centered on the ∑'s axis.
	if isDisplayBigOp {
		return placeLimits(a, nuc, style, ctx)
	}
	return placeSubSup(a, nuc, style, ctx)
}

// placeLimits assembles a big-op nucleus with sub/sup as upper and lower
// limits (centered above/below) instead of inline scripts. The vertical
// geometry uses OT-MATH's Upper/LowerLimit constants:
//
//	upper baseline rise = max(UpperLimitBaselineRiseMin, sub-from-bbox)
//	lower baseline drop = max(LowerLimitBaselineDropMin, sub-from-bbox)
//	+ gap-min constraints to ensure the limit doesn't touch the op
//
// All three boxes (sup limit, nucleus, sub limit) are centered horizontally
// on the widest of the three. Each non-empty limit goes through its own
// style cascade (subStyle / supStyle), so they shrink as expected.
func placeLimits(a *MathAtom, nuc *node.HList, style MathStyle, ctx *engineCtx) *node.HList {
	fnt := ctx.at(style)
	c := ctx.cons
	size := fnt.Size
	upem := upemOf(fnt)

	hasUpper := !a.Sup.IsEmpty()
	hasLower := !a.Sub.IsEmpty()
	var upper, lower *node.HList
	if hasUpper {
		upper = layoutField(a.Sup, supStyle(style), ctx)
	}
	if hasLower {
		lower = layoutField(a.Sub, subStyle(style), ctx)
	}

	// Compute the widest box; everything centers on it.
	W := nuc.Width
	if hasUpper && upper.Width > W {
		W = upper.Width
	}
	if hasLower && lower.Width > W {
		W = lower.Width
	}

	// Constants.
	upperBaselineRise := fuToSP(c.UpperLimitBaselineRiseMin, size, upem)
	upperGapMin := fuToSP(c.UpperLimitGapMin, size, upem)
	lowerBaselineDrop := fuToSP(c.LowerLimitBaselineDropMin, size, upem)
	lowerGapMin := fuToSP(c.LowerLimitGapMin, size, upem)

	// Upper: vertical distance from op's top to upper's baseline must be
	// at least UpperLimitBaselineRiseMin; the resulting gap (op top to
	// upper depth) must be at least UpperLimitGapMin.
	upperShift := bag.ScaledPoint(0)
	if hasUpper {
		upperShift = nuc.Height + upperBaselineRise
		gap := upperShift - upper.Depth - nuc.Height
		if gap < upperGapMin {
			upperShift += upperGapMin - gap
		}
	}
	// Lower: analogous with bottom.
	lowerShift := bag.ScaledPoint(0)
	if hasLower {
		lowerShift = nuc.Depth + lowerBaselineDrop
		gap := lowerShift - lower.Height - nuc.Depth
		if gap < lowerGapMin {
			lowerShift += lowerGapMin - gap
		}
	}

	// Build the VList top-to-bottom.
	var head, tail node.Node
	if hasUpper {
		// Center upper in width W.
		centered := centerInHList(upper, W)
		head, tail = appendNode(head, tail, centered)
		// Kern from upper's bottom to nucleus's top.
		gapTop := (upperShift - upper.Depth) - nuc.Height
		if gapTop > 0 {
			k := node.NewKern()
			k.Kern = gapTop
			head, tail = appendNode(head, tail, k)
		}
	}
	nucCentered := centerInHList(nuc, W)
	head, tail = appendNode(head, tail, nucCentered)
	if hasLower {
		// Kern from nucleus's bottom to lower's top.
		gapBot := (lowerShift - lower.Height) - nuc.Depth
		if gapBot > 0 {
			k := node.NewKern()
			k.Kern = gapBot
			head, tail = appendNode(head, tail, k)
		}
		centered := centerInHList(lower, W)
		head, tail = appendNode(head, tail, centered)
	}
	_ = tail

	vl := node.NewVList()
	vl.List = head
	vl.Width = W
	// Height: from VList top down to the outer baseline (which sits at
	// the nucleus's baseline). That equals upperShift + upper.Height
	// (if any) or nuc.Height alone.
	if hasUpper {
		vl.Height = upperShift + upper.Height
	} else {
		vl.Height = nuc.Height
	}
	if hasLower {
		vl.Depth = lowerShift + lower.Depth
	} else {
		vl.Depth = nuc.Depth
	}

	// Wrap the VList in an HList so the engine can splice it into the
	// inter-atom-spacing pass like any other atom result.
	out := node.NewHList()
	out.List = vl
	out.Width = vl.Width
	out.Height = vl.Height
	out.Depth = vl.Depth
	return out
}

// displayOpNucleus builds the display-size variant of an operator glyph
// (∑, ∫, …) using the OT-MATH MathVariants pipeline and centers the
// resulting box on the math axis. The vertical centering is what lets
// the operator's "middle" line up with the surrounding inline math —
// without it the giant ∑ would sit too low (baseline-aligned, with
// most of the glyph above the line).
func displayOpNucleus(gid ot.GlyphID, style MathStyle, ctx *engineCtx) *node.HList {
	fnt := ctx.at(style)
	c := ctx.cons
	upem := upemOf(fnt)
	if upem == 0 {
		return wrapGlyphInHList(buildGlyph(ctx, gid, style))
	}
	minSizeFU := c.DisplayOperatorMinHeight
	if minSizeFU == 0 {
		// Fallback when the font doesn't set the constant — use 1.5 × em.
		minSizeFU = uint16(upem * 3 / 2)
	}
	hl := stretchedVertical(ctx, gid, minSizeFU, style)

	// Center on math axis. The glyph's vertical midpoint sits at
	// (Height − Depth)/2 above the baseline (uses real per-glyph
	// extents from df7c4b75; the old "Depth=0 after stretch"
	// assumption no longer holds). Shift by (AxisHeight − midpoint)
	// so the visual center lands on +AxisHeight.
	axis := fuToSP(c.AxisHeight, fnt.Size, upem)
	half := (hl.Height + hl.Depth) / 2
	desiredCenter := axis
	currentCenter := hl.Height - half // = (Height − Depth)/2
	delta := desiredCenter - currentCenter
	if delta != 0 {
		hl.Shift = delta
		hl.Height += delta
		hl.Depth -= delta
		if hl.Depth < 0 {
			hl.Depth = 0
		}
		if hl.Height < 0 {
			hl.Height = 0
		}
	}
	return hl
}

// layoutField lowers a MathField to an HList. Three cases:
//
//   - Glyph != 0  → single-glyph HList at the caller's style.
//   - Sublist     → recurse into mlistToHlist.
//   - empty       → empty HList (width 0, height 0, depth 0).
func layoutField(f MathField, style MathStyle, ctx *engineCtx) *node.HList {
	if f.Glyph != 0 {
		g := buildGlyph(ctx, f.Glyph, style)
		return wrapGlyphInHList(g)
	}
	if f.Sublist != nil {
		return mlistToHlist(f.Sublist, style, ctx)
	}
	return emptyHList()
}

// buildGlyph creates a single Glyph node at the given style's scaled body
// size. The Glyph node carries the *style-specific* font (from ctx.at),
// not the base font — the PDF renderer emits the glyph's Tf size from
// `Glyph.Font.Size`, so a script glyph using the base font would render
// at full size, overlap its neighbors, and break the layout's geometry.
//
// Metrics use REAL per-glyph extents read from the outline (works for
// both TrueType and CFF/CFF2). This replaces the phase-1 pauschal
// approximation (Height = Size − Depth, applied uniformly) — important
// for sub/sup placement against letters with no descender (a, c, e)
// vs letters with full descender (g, p, q). Glyphs without an outline
// (space, control characters) fall back to the pauschal model.
func buildGlyph(ctx *engineCtx, gid ot.GlyphID, style MathStyle) *node.Glyph {
	fnt := ctx.at(style)
	upem := upemOf(fnt)

	g := node.NewGlyph()
	g.Font = fnt
	g.Codepoint = int(gid)
	if upem == 0 || fnt.Face == nil || fnt.Face.Shaper == nil {
		g.Height = fnt.Size - fnt.Depth
		g.Depth = fnt.Depth
		return g
	}
	advFU := int64(fnt.Face.Shaper.GetGlyphHAdvanceVar(gid))
	g.Width = bag.ScaledPoint(advFU * int64(fnt.Size) / int64(upem))

	if otFace := fnt.Face.OTFace(); otFace != nil {
		if bbox, ok := otFace.GlyphExtents(gid); ok {
			g.Height = bag.ScaledPoint(int64(bbox.YMax) * int64(fnt.Size) / int64(upem))
			depthFU := int64(-bbox.YMin)
			if depthFU < 0 {
				depthFU = 0
			}
			g.Depth = bag.ScaledPoint(depthFU * int64(fnt.Size) / int64(upem))
			if g.Height < 0 {
				g.Height = 0
			}
			return g
		}
	}
	g.Height = fnt.Size - fnt.Depth
	g.Depth = fnt.Depth
	return g
}

// wrapGlyphInHList builds a one-glyph HList whose declared width / height /
// depth match the glyph's. The HList is the lowest-common-denominator for
// the rest of the engine (every layout helper returns one).
func wrapGlyphInHList(g *node.Glyph) *node.HList {
	hl := node.NewHList()
	hl.List = g
	hl.Width = g.Width
	hl.Height = g.Height
	hl.Depth = g.Depth
	return hl
}

// placeSubSup attaches sub- and superscripts to a nucleus HList, returning
// a fresh HList whose width / height / depth account for the scripts. The
// rules follow TeXbook Appendix G and the OT-MATH constants:
//
//	supShift = max( SuperscriptShiftUp{Cramped}[FU→SP],
//	                nuc.Height − SuperscriptBaselineDropMax,
//	                SuperscriptBottomMin + sup.Depth )
//	subShift = max( SubscriptShiftDown,
//	                nuc.Depth + SubscriptBaselineDropMin,
//	                sub.Height − SubscriptTopMax )
//
// If both scripts are present and the resulting gap between sup-bottom and
// sub-top is below SubSuperscriptGapMin, sup is pushed up and sub is pushed
// down until the gap is satisfied, and the result is reconciled against
// SuperscriptBottomMaxWithSubscript.
//
// The horizontal layout uses a "stacked" trick: the sub box is appended
// with a negative kern that walks back to the script-start position, so
// sub and sup share the same X origin. Trailing space is SpaceAfterScript
// from the MATH table.
func placeSubSup(a *MathAtom, nuc *node.HList, style MathStyle, ctx *engineCtx) *node.HList {
	fnt := ctx.at(style)
	c := ctx.cons
	size := fnt.Size
	upem := upemOf(fnt)
	hasSub := !a.Sub.IsEmpty()
	hasSup := !a.Sup.IsEmpty()

	// Pre-compute laid-out boxes and italic correction.
	var subBox, supBox *node.HList
	if hasSub {
		subBox = layoutField(a.Sub, subStyle(style), ctx)
	}
	if hasSup {
		supBox = layoutField(a.Sup, supStyle(style), ctx)
	}

	// Italic correction is read off the last glyph of the nucleus, if it
	// is a single glyph. TeXbook Appendix G rule 18a.
	italicCorr := bag.ScaledPoint(0)
	if g, ok := nuc.List.(*node.Glyph); ok && g.Next() == nil {
		italicCorr = fnt.ItalicCorrection(g.Codepoint)
	}

	supShift, subShift := computeScriptShifts(nuc, subBox, supBox, style, c, size, upem)

	// Shift the inner boxes vertically and adjust their declared
	// height/depth so the outer HList "sees" the shifted geometry.
	if hasSup {
		shiftBoxVertically(supBox, +supShift)
	}
	if hasSub {
		shiftBoxVertically(subBox, -subShift)
	}

	// Apply per-corner math kerns. The nucleus's last glyph contributes
	// top-right (sup side) and bottom-right (sub side); the script's
	// first glyph contributes top-left / bottom-left. The two are added.
	var supKern, subKern bag.ScaledPoint
	if g, ok := nuc.List.(*node.Glyph); ok && g.Next() == nil {
		if hasSup {
			supKern = fnt.MathKernCorner(g.Codepoint, ot.MathKernTopRight, supShift)
			if sg := firstGlyph(supBox); sg != nil {
				supKern += fnt.MathKernCorner(sg.Codepoint, ot.MathKernBottomLeft, supShift)
			}
		}
		if hasSub {
			subKern = fnt.MathKernCorner(g.Codepoint, ot.MathKernBottomRight, subShift)
			if sg := firstGlyph(subBox); sg != nil {
				subKern += fnt.MathKernCorner(sg.Codepoint, ot.MathKernTopLeft, subShift)
			}
		}
	}

	// Assemble the outer HList. Layout:
	//   nucleus  italicCorr  [sup-box]  -W_sup  [sub-box]  finalKern  SpaceAfterScript
	scriptStartKern := italicCorr // applied before the first script
	out := node.NewHList()
	var head, tail node.Node
	width := bag.ScaledPoint(0)
	head, tail = appendNode(head, tail, nuc)
	width += nuc.Width

	var supW, subW bag.ScaledPoint
	if hasSup {
		supW = supBox.Width
	}
	if hasSub {
		subW = subBox.Width
	}
	scriptW := supW
	if subW > scriptW {
		scriptW = subW
	}

	if hasSub || hasSup {
		// Pre-kern (italic correction + per-corner kern of nucleus).
		preKern := scriptStartKern
		if hasSup && hasSub {
			// Use the smaller of the two corner kerns to stay conservative.
			if supKern < subKern {
				preKern += supKern
			} else {
				preKern += subKern
			}
		} else if hasSup {
			preKern += supKern
		} else {
			preKern += subKern
		}
		if preKern != 0 {
			k := node.NewKern()
			k.Kern = preKern
			head, tail = appendNode(head, tail, k)
			width += preKern
		}
	}

	if hasSup {
		head, tail = appendNode(head, tail, supBox)
		width += supW
	}
	if hasSub {
		// Walk back to script-start before placing the sub box, so sub
		// and sup share an x origin.
		if hasSup {
			back := node.NewKern()
			back.Kern = -supW
			head, tail = appendNode(head, tail, back)
			width -= supW
		}
		head, tail = appendNode(head, tail, subBox)
		width += subW
	}

	// Advance to the far edge of the script group.
	emittedScriptW := supW
	if !hasSup && hasSub {
		emittedScriptW = subW
	} else if hasSup && hasSub {
		emittedScriptW = subW // we ended at the sub box's right edge
	}
	if scriptW > emittedScriptW {
		k := node.NewKern()
		k.Kern = scriptW - emittedScriptW
		head, tail = appendNode(head, tail, k)
		width += scriptW - emittedScriptW
	}

	// Trailing space after the script group.
	spaceAfter := fuToSP(c.SpaceAfterScript, size, upem)
	if spaceAfter > 0 {
		k := node.NewKern()
		k.Kern = spaceAfter
		head, tail = appendNode(head, tail, k)
		width += spaceAfter
	}

	out.List = head
	out.Width = width

	// Compute bounding box: union of nucleus and (already-shifted) scripts.
	height, depth := nuc.Height, nuc.Depth
	if hasSup && supBox.Height > height {
		height = supBox.Height
	}
	if hasSub && subBox.Depth > depth {
		depth = subBox.Depth
	}
	out.Height = height
	out.Depth = depth
	return out
}

// computeScriptShifts implements the supShift / subShift formulas from
// TeXbook Appendix G via the OT-MATH constants. When both scripts are
// present, the SubSuperscriptGapMin constraint is enforced post-hoc by
// pushing sub down (cheaper than re-running sup) until the gap is right.
func computeScriptShifts(nuc, sub, sup *node.HList, style MathStyle, c *ot.MathConstants, size bag.ScaledPoint, upem int) (supShift, subShift bag.ScaledPoint) {
	// supShift baseline.
	if sup != nil {
		shiftConst := c.SuperscriptShiftUp
		if style.IsCramped() {
			shiftConst = c.SuperscriptShiftUpCramped
		}
		supShift = fuToSP(shiftConst, size, upem)
		// nuc.Height − SuperscriptBaselineDropMax (raise to clear nucleus top)
		raiseFromNuc := nuc.Height - fuToSP(c.SuperscriptBaselineDropMax, size, upem)
		if raiseFromNuc > supShift {
			supShift = raiseFromNuc
		}
		// SuperscriptBottomMin + sup.Depth (sup bottom can't dip below this)
		minBottom := fuToSP(c.SuperscriptBottomMin, size, upem) + sup.Depth
		if minBottom > supShift {
			supShift = minBottom
		}
	}

	if sub != nil {
		subShift = fuToSP(c.SubscriptShiftDown, size, upem)
		// nuc.Depth + SubscriptBaselineDropMin (drop to clear nucleus bottom)
		dropFromNuc := nuc.Depth + fuToSP(c.SubscriptBaselineDropMin, size, upem)
		if dropFromNuc > subShift {
			subShift = dropFromNuc
		}
		// sub.Height − SubscriptTopMax (sub top can't rise above this)
		topCap := sub.Height - fuToSP(c.SubscriptTopMax, size, upem)
		if topCap > subShift {
			subShift = topCap
		}
	}

	// Gap enforcement when both scripts present.
	if sub != nil && sup != nil {
		gapMin := fuToSP(c.SubSuperscriptGapMin, size, upem)
		gap := (supShift - sup.Depth) - (sub.Height - subShift)
		if gap < gapMin {
			// Push sub down by the missing amount.
			missing := gapMin - gap
			subShift += missing
			gap += missing
			// Then reconcile against SuperscriptBottomMaxWithSubscript:
			// sup bottom must be at most this far above the baseline.
			supBottomMax := fuToSP(c.SuperscriptBottomMaxWithSubscript, size, upem)
			supBottom := supShift - sup.Depth
			if supBottom > supBottomMax {
				// Lift the bottom by raising sup — actually we DROP sup so
				// its bottom matches supBottomMax (TeX rule), then re-check
				// the gap and let sub absorb the rest.
				excess := supBottom - supBottomMax
				supShift -= excess
				gap -= excess
				if gap < gapMin {
					subShift += gapMin - gap
				}
			}
		}
	}
	return supShift, subShift
}

// shiftBoxVertically applies a vertical offset to every Glyph and box-like
// child of an HList. Glyph.YOffset accumulates the offset; nested HList /
// VList children get their Shift adjusted. The HList's own declared
// Height / Depth are pre-adjusted so its parent sees the shifted box.
//
// dy > 0 shifts toward the top of the page (PDF +Y) — matches Glyph.YOffset
// and node.HList.Shift conventions.
func shiftBoxVertically(hl *node.HList, dy bag.ScaledPoint) {
	if hl == nil || dy == 0 {
		return
	}
	for n := hl.List; n != nil; n = n.Next() {
		switch x := n.(type) {
		case *node.Glyph:
			x.YOffset += dy
		case *node.HList:
			x.Shift += dy
		case *node.VList:
			x.Shift += dy
		}
	}
	hl.Height += dy
	hl.Depth -= dy
	if hl.Height < 0 {
		hl.Height = 0
	}
	if hl.Depth < 0 {
		hl.Depth = 0
	}
}

// firstGlyph recursively finds the first Glyph node in an HList's content,
// descending into nested HLists and VLists. mlistToHlist produces multi-
// level nesting (outer splice HList → per-atom HList → glyph wrapper HList →
// glyph), so a one-level walk would always return nil for any layout that
// went through the engine — a foot-gun caught when accent layout incorrectly
// hit its "empty body" fallback because firstGlyph couldn't find the body
// glyph through the wrapper.
func firstGlyph(hl *node.HList) *node.Glyph {
	if hl == nil {
		return nil
	}
	return firstGlyphIn(hl.List)
}

func firstGlyphIn(start node.Node) *node.Glyph {
	for n := start; n != nil; n = n.Next() {
		switch x := n.(type) {
		case *node.Glyph:
			return x
		case *node.HList:
			if g := firstGlyphIn(x.List); g != nil {
				return g
			}
		case *node.VList:
			if g := firstGlyphIn(x.List); g != nil {
				return g
			}
		}
	}
	return nil
}
