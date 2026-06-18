package math

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

// stretchedVertical builds a vertically-stretched glyph for `baseGid`
// reaching at least `minSizeFU` font units of total advance (= height +
// depth). It applies the OpenType MATH stretching pipeline:
//
//  1. Try the size-ordered pre-built variants — pick the smallest one
//     whose advance meets the request. This is what the font designer
//     intends for the typical sizes.
//  2. If no pre-built variant is large enough AND the font ships a
//     GlyphAssembly recipe, build a stack of top + N×extender + bottom
//     pieces (and any non-extender middle pieces) sized to fit.
//  3. If neither: log a phase-2 warning and fall back to the base glyph.
//
// The returned HList carries the stretched glyph (a single Glyph or a
// VList wrapping multiple parts), with width/height/depth pre-computed.
// `style` controls which scaled font is used for advance/depth math.
func stretchedVertical(ctx *engineCtx, baseGid ot.GlyphID, minSizeFU uint16, style MathStyle) *node.HList {
	fnt := ctx.at(style)
	if fnt.Face == nil || fnt.Face.Shaper == nil {
		return wrapGlyphInHList(buildGlyph(ctx, baseGid, style))
	}
	m := fnt.Face.Shaper.Math()
	if m == nil || !m.HasMathVariants() {
		return wrapGlyphInHList(buildGlyph(ctx, baseGid, style))
	}

	// Step 1: pre-built variants. We keep the wrapper's pauschal Height /
	// Depth as set by buildGlyph — overriding with the variant's
	// AdvanceFU shifted limits placement in a way that overlapped the
	// big-op's body with its lower limit (the variant glyph design
	// places its outline around the math axis, but Height=AdvanceFU /
	// Depth=0 would treat it as baseline-anchored, mis-locating the
	// "lower bound" of the box). The selected variant glyph still
	// renders at its actual design size in the PDF stream — only the
	// declared box matches the base pauschal metrics. This is OK for
	// the radical and fraction-delim callers, which size by requiredFU
	// up-front and don't read the result's Height back.
	variants := m.VerticalVariants(baseGid)
	for _, v := range variants {
		if v.AdvanceFU >= minSizeFU {
			return wrapGlyphInHList(buildGlyph(ctx, v.GlyphID, style))
		}
	}
	// The largest variant — if any — that is still smaller than required.
	// We may use it as a fallback below if no assembly is available.
	var largestVariant ot.GlyphID
	if len(variants) > 0 {
		largestVariant = variants[len(variants)-1].GlyphID
	}

	// Step 2: glyph assembly.
	if assembly := m.VerticalAssembly(baseGid); assembly != nil {
		if vl, ok := buildVerticalAssembly(ctx, assembly, minSizeFU, m.MinConnectorOverlap(), style); ok {
			out := node.NewHList()
			out.List = vl
			out.Width = vl.Width
			out.Height = vl.Height
			out.Depth = vl.Depth
			return out
		}
	}

	// Step 3: fall back to the largest pre-built variant (if any) or the
	// base glyph. A warning surfaces the under-sized result without
	// stopping rendering.
	fallbackGid := baseGid
	if largestVariant != 0 {
		fallbackGid = largestVariant
	}
	bag.Logger.Warn("math: no variant or assembly reaches required size — using largest available",
		"glyph", baseGid, "needFU", minSizeFU)
	return wrapGlyphInHList(buildGlyph(ctx, fallbackGid, style))
}

// buildVerticalAssembly stacks the assembly's parts top-to-bottom as a
// VList until the total advance meets minSizeFU. The algorithm follows
// OT-MATH spec §6.5 and LuaTeX's `var_glyph_construct` in mlist.c:
//
//   - Fixed parts (non-extenders) appear exactly once, in order.
//   - Each extender can be repeated N times. We pick the smallest N that
//     yields total advance ≥ minSizeFU.
//   - Adjacent parts overlap by max(minOverlap, min(prev.endConn, next.startConn))
//     so consecutive pieces share a seam region — necessary to avoid
//     transparent gaps along the spine of the assembled glyph.
//
// Returns (vlist, true) when an assembly was built; (nil, false) when
// the assembly contains no parts.
func buildVerticalAssembly(ctx *engineCtx, assembly *ot.MathGlyphAssembly, minSizeFU, minOverlapFU uint16, style MathStyle) (*node.VList, bool) {
	if len(assembly.Parts) == 0 {
		return nil, false
	}

	// Find a multiplier N ≥ 1 such that total advance covers minSizeFU.
	// Some assemblies have no extenders — in that case N = 1 is the only
	// option and we just use the fixed parts.
	extenderCount := 0
	for _, p := range assembly.Parts {
		if p.IsExtender {
			extenderCount++
		}
	}

	repeats := 1
	if extenderCount > 0 {
		// Upper bound on iterations — protects against degenerate fonts.
		const maxRepeats = 64
		for repeats <= maxRepeats {
			adv := assemblyAdvanceFU(assembly.Parts, repeats, minOverlapFU)
			if adv >= int(minSizeFU) {
				break
			}
			repeats++
		}
	}

	fnt := ctx.at(style)
	upem := upemOf(fnt)
	if upem == 0 {
		return nil, false
	}

	// Build the linked list of part glyphs top-to-bottom with negative
	// kerns realizing the overlaps. Each kern carries a negative height
	// equal to the overlap — by convention in our VList builder we add
	// vertical kerns as Kern nodes (which are horizontal advance only,
	// but inside a VList they advance vertically — see node.Vpack).
	var head, tail node.Node
	prevEndConn := uint16(0)
	hasPrevious := false
	totalAdvanceFU := 0
	for _, p := range assembly.Parts {
		n := 1
		if p.IsExtender {
			n = repeats
		}
		for i := 0; i < n; i++ {
			if hasPrevious {
				overlap := minOverlapFU
				thisStart := p.StartConnectorLengthFU
				smaller := prevEndConn
				if thisStart < smaller {
					smaller = thisStart
				}
				if smaller > overlap {
					overlap = smaller
				}
				if overlap > 0 {
					k := node.NewKern()
					k.Kern = -bag.ScaledPoint(int64(overlap) * int64(fnt.Size) / int64(upem))
					head, tail = appendNode(head, tail, k)
					totalAdvanceFU -= int(overlap)
				}
			}
			g := node.NewGlyph()
			g.Font = fnt
			g.Codepoint = int(p.GlyphID)
			advFU := int64(p.FullAdvanceFU)
			advSP := bag.ScaledPoint(advFU * int64(fnt.Size) / int64(upem))
			// For the parts of a vertical assembly we use FullAdvance as
			// the vertical extent and the horizontal advance of the
			// glyph itself for width. Look the latter up via the shaper.
			hAdv := int64(fnt.Face.Shaper.GetGlyphHAdvanceVar(p.GlyphID))
			g.Width = bag.ScaledPoint(hAdv * int64(fnt.Size) / int64(upem))
			// Each part takes its full advance vertically (height); depth
			// 0 by convention so the next piece starts immediately below.
			g.Height = advSP
			g.Depth = 0
			head, tail = appendNode(head, tail, g)
			totalAdvanceFU += int(p.FullAdvanceFU)
			prevEndConn = p.EndConnectorLengthFU
			hasPrevious = true
		}
	}
	_ = tail

	vl := node.NewVList()
	vl.List = head
	// Total advance in SP.
	totalAdvanceSP := bag.ScaledPoint(int64(totalAdvanceFU) * int64(fnt.Size) / int64(upem))
	vl.Height = totalAdvanceSP
	vl.Depth = 0
	// Width: max child width.
	for n := head; n != nil; n = n.Next() {
		if g, ok := n.(*node.Glyph); ok {
			if g.Width > vl.Width {
				vl.Width = g.Width
			}
		}
	}
	return vl, true
}

// assemblyAdvanceFU returns the total advance (in font units) that an
// assembly will achieve with `repeats` extender repetitions, given the
// minimum connector overlap. Pure arithmetic — no node allocations.
func assemblyAdvanceFU(parts []ot.MathGlyphPart, repeats int, minOverlap uint16) int {
	if len(parts) == 0 {
		return 0
	}
	total := 0
	prevEndConn := uint16(0)
	hasPrevious := false
	for _, p := range parts {
		n := 1
		if p.IsExtender {
			n = repeats
		}
		for i := 0; i < n; i++ {
			if hasPrevious {
				overlap := minOverlap
				smaller := prevEndConn
				if p.StartConnectorLengthFU < smaller {
					smaller = p.StartConnectorLengthFU
				}
				if smaller > overlap {
					overlap = smaller
				}
				total -= int(overlap)
			}
			total += int(p.FullAdvanceFU)
			prevEndConn = p.EndConnectorLengthFU
			hasPrevious = true
		}
	}
	return total
}
