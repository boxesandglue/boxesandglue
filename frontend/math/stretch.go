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

	// Find a multiplier N ≥ 1 such that the maximum achievable advance
	// (minimal overlaps) covers minSizeFU. Some assemblies have no
	// extenders — then N = 1 is the only option.
	extenderCount := 0
	for _, p := range assembly.Parts {
		if p.IsExtender {
			extenderCount++
		}
	}
	repeats := 1
	if extenderCount > 0 {
		const maxRepeats = 64
		for repeats <= maxRepeats {
			if assemblyMaxAdvanceFU(assembly.Parts, repeats, minOverlapFU) >= int(minSizeFU) {
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

	// Plan overlaps on the emitted sequence. OT-MATH lists vertical
	// parts bottom-to-top (spec §6.5) while the VList stacks top-down,
	// so the sequence is reversed; swapping each part's connector roles
	// lets assemblyPlan keep its uniform End/Start pairing. Each part
	// glyph is wrapped in its own HList: the page output routine
	// positions boxes inside a VList — a bare Glyph child would be
	// skipped silently.
	seq := expandParts(assembly.Parts, repeats)
	for i, j := 0, len(seq)-1; i < j; i, j = i+1, j-1 {
		seq[i], seq[j] = seq[j], seq[i]
	}
	for i := range seq {
		seq[i].StartConnectorLengthFU, seq[i].EndConnectorLengthFU = seq[i].EndConnectorLengthFU, seq[i].StartConnectorLengthFU
	}
	overlaps, totalFU := assemblyPlan(seq, minOverlapFU, int(minSizeFU))

	var head, tail node.Node
	for i, p := range seq {
		if i > 0 && overlaps[i-1] != 0 {
			k := node.NewKern()
			k.Kern = -bag.ScaledPoint(int64(overlaps[i-1]) * int64(fnt.Size) / int64(upem))
			head, tail = appendNode(head, tail, k)
		}
		g := node.NewGlyph()
		g.Font = fnt
		g.Codepoint = int(p.GlyphID)
		advSP := bag.ScaledPoint(int64(p.FullAdvanceFU) * int64(fnt.Size) / int64(upem))
		// For the parts of a vertical assembly we use FullAdvance as
		// the vertical extent and the horizontal advance of the
		// glyph itself for width. Look the latter up via the shaper.
		hAdv := int64(fnt.Face.Shaper.GetGlyphHAdvanceVar(p.GlyphID))
		g.Width = bag.ScaledPoint(hAdv * int64(fnt.Size) / int64(upem))
		g.Height = advSP
		g.Depth = 0
		box := node.NewHList()
		box.List = g
		box.Width = g.Width
		box.Height = g.Height
		box.Depth = 0
		head, tail = appendNode(head, tail, box)
	}
	_ = tail

	vl := node.NewVList()
	vl.List = head
	vl.Height = bag.ScaledPoint(int64(totalFU) * int64(fnt.Size) / int64(upem))
	vl.Depth = 0
	// Width: max child width.
	for n := head; n != nil; n = n.Next() {
		if hb, ok := n.(*node.HList); ok {
			if hb.Width > vl.Width {
				vl.Width = hb.Width
			}
		}
	}
	return vl, true
}

// expandParts flattens an assembly part list into the emitted sequence:
// each extender repeated `repeats` times, order preserved.
func expandParts(parts []ot.MathGlyphPart, repeats int) []ot.MathGlyphPart {
	seq := make([]ot.MathGlyphPart, 0, len(parts)+repeats)
	for _, p := range parts {
		n := 1
		if p.IsExtender {
			n = repeats
		}
		for i := 0; i < n; i++ {
			seq = append(seq, p)
		}
	}
	return seq
}

// assemblyPlan computes per-joint overlaps for an emitted part sequence.
// OT-MATH connector lengths give the MAXIMUM overlappable region of a
// joint; MinConnectorOverlap the minimum a seamless joint needs. The
// assembly grows with minimal overlaps and, when that overshoots the
// target, the excess is distributed back onto the joints up to each
// joint's connector capacity — the LuaTeX/HarfBuzz scheme. Returns the
// per-joint overlaps and the resulting total advance, all in font units.
func assemblyPlan(seq []ot.MathGlyphPart, minOverlap uint16, targetFU int) (overlaps []int, totalFU int) {
	if len(seq) == 0 {
		return nil, 0
	}
	joints := len(seq) - 1
	overlaps = make([]int, joints)
	total := 0
	for _, p := range seq {
		total += int(p.FullAdvanceFU)
	}
	caps := make([]int, joints)
	for i := 0; i < joints; i++ {
		maxOv := int(seq[i].EndConnectorLengthFU)
		if st := int(seq[i+1].StartConnectorLengthFU); st < maxOv {
			maxOv = st
		}
		oMin := int(minOverlap)
		if oMin > maxOv {
			oMin = maxOv
		}
		overlaps[i] = oMin
		caps[i] = maxOv - oMin
		total -= oMin
	}
	// Shrink toward the target by deepening overlaps, front to back.
	excess := total - targetFU
	for i := 0; i < joints && excess > 0; i++ {
		add := excess / (joints - i)
		if rem := excess % (joints - i); rem > 0 {
			add++
		}
		if add > caps[i] {
			add = caps[i]
		}
		overlaps[i] += add
		total -= add
		excess -= add
	}
	return overlaps, total
}

// assemblyMaxAdvanceFU is the largest advance an assembly reaches with
// `repeats` extender repetitions (minimal overlaps everywhere).
func assemblyMaxAdvanceFU(parts []ot.MathGlyphPart, repeats int, minOverlap uint16) int {
	_, total := assemblyPlan(expandParts(parts, repeats), minOverlap, 1<<30)
	return total
}

// centerOnAxis shifts hl vertically so its visual midpoint — at
// (Height − Depth)/2 above the baseline, using real per-glyph extents —
// lands on the math axis. Height/Depth are adjusted to the shifted
// geometry and clamped at zero so the surrounding box union stays sane.
// Shared by display big-op nuclei, fraction delimiters and fences.
func centerOnAxis(hl *node.HList, axis bag.ScaledPoint) {
	if hl.Height == 0 && hl.Depth == 0 {
		return
	}
	currentCenter := (hl.Height - hl.Depth) / 2
	shift := axis - currentCenter
	if shift == 0 {
		return
	}
	hl.Shift = shift
	hl.Height += shift
	hl.Depth -= shift
	if hl.Depth < 0 {
		hl.Depth = 0
	}
	if hl.Height < 0 {
		hl.Height = 0
	}
	// Anchoring asymmetry in the page output: a Glyph child of an HList
	// renders at the (Shift-adjusted) baseline, but a VList child hangs
	// its top edge from the DECLARED hlist.Height — which now includes
	// the shift, so a glyph-assembly VList would move twice. Compensate
	// on the child's own Shift.
	if vl, ok := hl.List.(*node.VList); ok && vl.Next() == nil {
		vl.Shift -= shift
	}
}

// stretchedHorizontal builds a horizontally-stretched glyph for baseGid
// reaching at least minWidthFU font units of advance — the horizontal
// mirror of stretchedVertical, used for stretchy under/over operators
// (arrows, braces) below or above a wide base. Pipeline: pre-built
// HorizontalVariants, then HorizontalAssembly, then largest-variant/base
// fallback with a warning.
func stretchedHorizontal(ctx *engineCtx, baseGid ot.GlyphID, minWidthFU uint16, style MathStyle) *node.HList {
	fnt := ctx.at(style)
	base := wrapGlyphInHList(buildGlyph(ctx, baseGid, style))
	if fnt.Face == nil || fnt.Face.Shaper == nil {
		return base
	}
	upem := upemOf(fnt)
	if upem == 0 {
		return base
	}
	// Early out: the base glyph already covers the request.
	if spToFU(base.Width, fnt.Size, upem) >= minWidthFU {
		return base
	}
	m := fnt.Face.Shaper.Math()
	if m == nil || !m.HasMathVariants() {
		return base
	}

	variants := m.HorizontalVariants(baseGid)
	for _, v := range variants {
		if v.AdvanceFU >= minWidthFU {
			return wrapGlyphInHList(buildGlyph(ctx, v.GlyphID, style))
		}
	}
	var largestVariant ot.GlyphID
	if len(variants) > 0 {
		largestVariant = variants[len(variants)-1].GlyphID
	}

	if assembly := m.HorizontalAssembly(baseGid); assembly != nil {
		if hl, ok := buildHorizontalAssembly(ctx, assembly, minWidthFU, m.MinConnectorOverlap(), style); ok {
			return hl
		}
	}

	fallbackGid := baseGid
	if largestVariant != 0 {
		fallbackGid = largestVariant
	}
	bag.Logger.Warn("math: no horizontal variant or assembly reaches required width — using largest available",
		"glyph", baseGid, "needFU", minWidthFU)
	return wrapGlyphInHList(buildGlyph(ctx, fallbackGid, style))
}

// buildHorizontalAssembly lines the assembly's parts up left-to-right in
// an HList until the total advance meets minWidthFU. OT-MATH lists
// horizontal assembly parts in visual order left to right, which matches
// the HList direction, so no reversal is needed; joints become negative
// kerns per the assemblyPlan overlaps.
func buildHorizontalAssembly(ctx *engineCtx, assembly *ot.MathGlyphAssembly, minWidthFU, minOverlapFU uint16, style MathStyle) (*node.HList, bool) {
	if len(assembly.Parts) == 0 {
		return nil, false
	}
	extenderCount := 0
	for _, p := range assembly.Parts {
		if p.IsExtender {
			extenderCount++
		}
	}
	repeats := 1
	if extenderCount > 0 {
		const maxRepeats = 64
		for repeats <= maxRepeats {
			if assemblyMaxAdvanceFU(assembly.Parts, repeats, minOverlapFU) >= int(minWidthFU) {
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

	seq := expandParts(assembly.Parts, repeats)
	overlaps, totalFU := assemblyPlan(seq, minOverlapFU, int(minWidthFU))

	var head, tail node.Node
	var height, depth bag.ScaledPoint
	for i, p := range seq {
		if i > 0 && overlaps[i-1] != 0 {
			k := node.NewKern()
			k.Kern = -bag.ScaledPoint(int64(overlaps[i-1]) * int64(fnt.Size) / int64(upem))
			head, tail = appendNode(head, tail, k)
		}
		g := buildGlyph(ctx, p.GlyphID, style)
		// The advance along the assembly axis comes from the part
		// record, not the font's own advance.
		g.Width = bag.ScaledPoint(int64(p.FullAdvanceFU) * int64(fnt.Size) / int64(upem))
		head, tail = appendNode(head, tail, g)
		if g.Height > height {
			height = g.Height
		}
		if g.Depth > depth {
			depth = g.Depth
		}
	}
	_ = tail

	out := node.NewHList()
	out.List = head
	out.Width = bag.ScaledPoint(int64(totalFU) * int64(fnt.Size) / int64(upem))
	out.Height = height
	out.Depth = depth
	return out, true
}
