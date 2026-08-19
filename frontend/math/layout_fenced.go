package math

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

// layoutFenced lowers a \left…\right group to an HList: the body at the
// caller's style (TeX does not change style inside \left…\right), flanked
// by delimiters stretched to cover the body's span.
//
// Sizing follows TeX's rule for delimited subformulas: the delimiter must
// reach at least 2·max(bodyHeight − axis, bodyDepth + axis), i.e. it is
// sized symmetrically around the math axis. In display style
// DelimitedSubFormulaMinHeight acts as a floor. A base glyph that already
// covers the target is used unstretched — this keeps \left( x \right)
// around inline-height content a visual no-op instead of jumping to the
// first (deliberately bigger) variant.
func layoutFenced(f *Fenced, style MathStyle, ctx *engineCtx) *node.HList {
	body := mlistToHlist(f.Body, style, ctx)
	fnt := ctx.at(style)
	c := ctx.cons
	upem := upemOf(fnt)
	if upem == 0 {
		return body
	}
	axis := fuToSP(c.AxisHeight, fnt.Size, upem)

	delta := body.Height - axis
	if d := body.Depth + axis; d > delta {
		delta = d
	}
	targetSP := 2 * delta
	if style.IsDisplay() {
		// DelimitedSubFormulaMinHeight is uint16 — convert manually, the
		// int16-typed fuToSP would overflow for large values.
		if minSP := bag.ScaledPoint(int64(c.DelimitedSubFormulaMinHeight) * int64(fnt.Size) / int64(upem)); minSP > targetSP {
			targetSP = minSP
		}
	}

	makeFence := func(gid ot.GlyphID) *node.HList {
		fence := wrapGlyphInHList(buildGlyph(ctx, gid, style))
		if fence.Height+fence.Depth < targetSP {
			fence = stretchedVertical(ctx, gid, spToFU(targetSP, fnt.Size, upem), style)
		}
		centerOnAxis(fence, axis)
		return fence
	}

	out := node.NewHList()
	var head, tail node.Node
	width := bag.ScaledPoint(0)
	height, depth := body.Height, body.Depth
	add := func(hl *node.HList) {
		head, tail = appendNode(head, tail, hl)
		width += hl.Width
		if hl.Height > height {
			height = hl.Height
		}
		if hl.Depth > depth {
			depth = hl.Depth
		}
	}
	if f.Left != 0 {
		add(makeFence(f.Left))
	}
	add(body)
	if f.Right != 0 {
		add(makeFence(f.Right))
	}
	out.List = head
	out.Width = width
	out.Height = height
	out.Depth = depth
	return out
}
