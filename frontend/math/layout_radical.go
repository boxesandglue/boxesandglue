package math

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

// layoutRadical lowers a Radical (√body or ⁿ√body) to an HList. Horizontal
// layout, left to right:
//
//	[degree?]  [kern br]  [radical glyph]  [VList: rule above, body below]
//
// Degree is in script-script style and is raised vertically by
// RadicalDegreeBottomRaisePercent of the radical glyph's height. Kerns
// before / after the degree come from RadicalKernBeforeDegree /
// RadicalKernAfterDegree — the "after" kern is typically negative and pulls
// the degree close to the radical, so the structure reads as ⁿ√ rather
// than ⁿ √.
//
// Phase 1 limitation: the radical glyph is used at its natural size. For
// tall bodies that visually need a bigger glyph (MathVariants), a warning
// is logged once per layout.
func layoutRadical(r *Radical, style MathStyle, ctx *engineCtx) *node.HList {
	fnt := ctx.at(style)
	c := ctx.cons
	size := fnt.Size
	upem := upemOf(fnt)

	bodyStyle := crampify(style)
	body := mlistToHlist(r.Body, bodyStyle, ctx)

	// Vertical gap above the body: display style picks the larger gap.
	var vgapConst int16
	if style.IsDisplay() {
		vgapConst = c.RadicalDisplayStyleVerticalGap
	} else {
		vgapConst = c.RadicalVerticalGap
	}
	vgap := fuToSP(vgapConst, size, upem)
	rule := fuToSP(c.RadicalRuleThickness, size, upem)
	extraAsc := fuToSP(c.RadicalExtraAscender, size, upem)

	// Build the right-side VList: rule above the body, separated by vgap,
	// with ExtraAscender on top.
	var head, tail node.Node
	if extraAsc > 0 {
		k := node.NewKern()
		k.Kern = extraAsc
		head, tail = appendNode(head, tail, k)
	}
	ruleNode := node.NewRule()
	ruleNode.Width = body.Width
	ruleNode.Height = rule
	ruleNode.Depth = 0
	head, tail = appendNode(head, tail, ruleNode)
	if vgap > 0 {
		k := node.NewKern()
		k.Kern = vgap
		head, tail = appendNode(head, tail, k)
	}
	head, tail = appendNode(head, tail, body)
	_ = tail

	rightVL := node.NewVList()
	rightVL.List = head
	rightVL.Width = body.Width
	rightVL.Height = body.Height + vgap + rule + extraAsc
	rightVL.Depth = body.Depth

	// Stretch the radical glyph to the required size. requiredFU targets
	// the variant selector's advanceMeasurement, which is the font
	// designer's nominal span for that variant; the variant's REAL yMax
	// is read by buildGlyph from the outline and may be slightly smaller
	// than advanceMeasurement (the bottom hook eats a bit of the span).
	// We compensate after the fact by shifting the radical so its real
	// yMax lines up with the overbar's top.
	requiredSP := rightVL.Height + rightVL.Depth
	requiredFU := uint16(int64(requiredSP) * int64(upem) / int64(size))
	if requiredFU == 0 {
		requiredFU = 1
	}
	radHL := stretchedVertical(ctx, r.Glyph, requiredFU, style)

	// Trim the radical's right-side bearing. The glyph's HAdvance
	// includes a small gap past the outline's xMax — fine for
	// neighboring atoms, but here it would leave a visible gap
	// between the radical's top stub and the start of the overbar.
	// Read the real xMax from the outline and clamp Width to it.
	if otFace := fnt.Face.OTFace(); otFace != nil {
		if g, ok := radHL.List.(*node.Glyph); ok {
			if bbox, bboxOk := otFace.GlyphExtents(ot.GlyphID(g.Codepoint)); bboxOk {
				xMaxSP := bag.ScaledPoint(int64(bbox.XMax) * int64(fnt.Size) / int64(upem))
				if xMaxSP > 0 && xMaxSP < radHL.Width {
					radHL.Width = xMaxSP
				}
			}
		}
	}

	// Position the radical's visible top (= its baseline + its real yMax)
	// exactly at the overbar's top. The overbar TOP is at body.Height +
	// vgap + rule above the outer baseline — RadicalExtraAscender is
	// extra WHITE SPACE above the rule, not part of the rule itself
	// (OT-MATH spec). Solve for Shift:  Shift + radHL.Height = overbarTop.
	//
	// The outer container needs to see the FULL bounding box of the
	// radical INCLUDING extraAsc above the rule, so the declared
	// Height becomes overbarTop + extraAsc.
	overbarTop := body.Height + vgap + rule
	realYMax := radHL.Height
	realDepth := radHL.Depth
	radHL.Shift = overbarTop - realYMax
	radHL.Height = overbarTop + extraAsc
	visibleBottom := radHL.Shift - realDepth
	if visibleBottom >= 0 {
		radHL.Depth = 0
	} else {
		radHL.Depth = -visibleBottom
	}

	// Assemble the outer HList.
	out := node.NewHList()
	var oh, ot node.Node
	width := bag.ScaledPoint(0)

	if r.Degree != nil {
		degree := mlistToHlist(r.Degree, ScriptScriptStyleCramped, ctx)
		kernBefore := fuToSP(c.RadicalKernBeforeDegree, size, upem)
		kernAfter := fuToSP(c.RadicalKernAfterDegree, size, upem)
		// Clamp the after-kern: the degree must not be pulled left of the
		// before-kern (LuaTeX mlist.c:2388).
		if -kernAfter > degree.Width+kernBefore {
			kernAfter = -(degree.Width + kernBefore)
		}
		// Raise the degree by raisePercent% of the radical's box height.
		raisePercent := int32(c.RadicalDegreeBottomRaisePercent)
		if raisePercent == 0 {
			raisePercent = 60 // OT-MATH guidance default when unset
		}
		raise := bag.ScaledPoint(int64(radHL.Height+radHL.Depth) * int64(raisePercent) / 100)
		degree.Shift = raise

		if kernBefore > 0 {
			k := node.NewKern()
			k.Kern = kernBefore
			oh, ot = appendNode(oh, ot, k)
			width += kernBefore
		}
		oh, ot = appendNode(oh, ot, degree)
		width += degree.Width
		if kernAfter != 0 {
			k := node.NewKern()
			k.Kern = kernAfter
			oh, ot = appendNode(oh, ot, k)
			width += kernAfter
		}
	}

	oh, ot = appendNode(oh, ot, radHL)
	width += radHL.Width
	oh, ot = appendNode(oh, ot, rightVL)
	width += rightVL.Width
	_ = ot

	out.List = oh
	out.Width = width

	// Outer bounding box: union of (stretched) radical and right VList.
	height := radHL.Height
	if rightVL.Height > height {
		height = rightVL.Height
	}
	depth := radHL.Depth
	if rightVL.Depth > depth {
		depth = rightVL.Depth
	}
	out.Height = height
	out.Depth = depth
	return out
}
