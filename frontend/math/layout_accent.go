package math

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

// layoutAccent lowers an Accent (single-glyph mark above or below a body) to
// an HList. The accent glyph is shifted horizontally by `skew`, which is
// the difference of the body's and the accent's TopAccentAttachment points.
// When either attachment is missing (the font has no entry for that glyph),
// the convention is "half the glyph's advance" — matches LuaTeX's
// compute_accent_skew() in mlist.c:2630.
//
// Phase 1 limitation: the body must reduce to a single glyph. Multi-glyph
// bodies trigger a warning and the layout proceeds with the first glyph
// only — this is correct for the typical "hat over single letter" use case.
//
// Vertical placement: the accent's baseline sits AccentBaseHeight above
// the outer baseline, raised further if the body is taller than that — the
// classic "flat accent on tall body" trick. FlattenedAccentBaseHeight is
// not consumed in phase 1; that variant lookup is a phase-2 task.
func layoutAccent(a *Accent, style MathStyle, ctx *engineCtx) *node.HList {
	fnt := ctx.at(style)
	c := ctx.cons
	size := fnt.Size
	upem := upemOf(fnt)

	// Body in cramped style — its sub/sup (if any) get cramping too.
	bodyHL := mlistToHlist(a.Body, crampify(style), ctx)
	bodyG := firstGlyph(bodyHL)
	if bodyG == nil {
		// Empty body — nothing to accent, emit just the accent at home.
		return wrapGlyphInHList(buildGlyph(ctx, a.Glyph, style))
	}

	// Walk the body for a second glyph — multi-glyph body is a phase-1
	// limit (we use only the first).
	multiglyph := false
	count := 0
	walkAllInternal(bodyHL.List, func(n node.Node) {
		if _, ok := n.(*node.Glyph); ok {
			count++
		}
	})
	if count > 1 {
		multiglyph = true
		bag.Logger.Warn("math: accent body has multiple glyphs — phase 1 attaches accent to the first only",
			"glyphCount", count)
	}
	_ = multiglyph

	accentG := buildGlyph(ctx, a.Glyph, style)
	accentHL := wrapGlyphInHList(accentG)

	// Attachment points. -1 sentinel means "absent" → half-width fallback.
	bodyAttach := fnt.Face.Shaper.Math().TopAccentAttachment(ot.GlyphID(bodyG.Codepoint))
	accentAttach := fnt.Face.Shaper.Math().TopAccentAttachment(a.Glyph)

	bodyAttachSP := bodyG.Width / 2
	if bodyAttach >= 0 {
		bodyAttachSP = fuToSP(int16(bodyAttach), size, upem)
	}
	accentAttachSP := accentG.Width / 2
	if accentAttach >= 0 {
		accentAttachSP = fuToSP(int16(accentAttach), size, upem)
	}
	skew := bodyAttachSP - accentAttachSP

	// Apply skew by wrapping the accent in an HList with a leading kern.
	accentWrapped := node.NewHList()
	{
		var head, tail node.Node
		if skew != 0 {
			k := node.NewKern()
			k.Kern = skew
			head, tail = appendNode(head, tail, k)
		}
		head, tail = appendNode(head, tail, accentHL)
		_ = tail
		accentWrapped.List = head
		accentWrapped.Width = skew + accentHL.Width
		accentWrapped.Height = accentHL.Height
		accentWrapped.Depth = accentHL.Depth
	}

	// Vertical placement. For a top accent, the accent's baseline sits at
	// max(AccentBaseHeight, body.Height) above the outer baseline. The
	// body itself lives at its natural baseline.
	accentBaseH := fuToSP(c.AccentBaseHeight, size, upem)
	if bodyHL.Height > accentBaseH {
		accentBaseH = bodyHL.Height
	}

	// Build the outer VList: accent on top (shifted up so its baseline is
	// at accentBaseH), then body below. For a bottom accent, swap.
	var head, tail node.Node
	if a.Bottom {
		// Bottom accent: body on top, then accent below the body.
		// Vertical positioning: accent sits at body.depth + accent.height
		// below the outer baseline.
		head, tail = appendNode(head, tail, bodyHL)
		// Gap kern: small fixed visual gap. UnderbarVerticalGap is the
		// closest constant; phase 1 uses zero so the accent sits flush.
		head, tail = appendNode(head, tail, accentWrapped)
		_ = tail
	} else {
		// Top accent: VList top is the accent, then a kern down to the
		// body's top, then body. The gap is computed against the
		// accent's BOTTOM (= baseline − Depth), not its baseline —
		// otherwise accents with real depth would push the body too
		// far down by accent.Depth.
		gap := accentBaseH - accentWrapped.Depth - bodyHL.Height
		if gap < 0 {
			gap = 0
		}
		head, tail = appendNode(head, tail, accentWrapped)
		if gap > 0 {
			k := node.NewKern()
			k.Kern = gap
			head, tail = appendNode(head, tail, k)
		}
		head, tail = appendNode(head, tail, bodyHL)
		_ = tail
	}

	vl := node.NewVList()
	vl.List = head
	w := bodyHL.Width
	if accentWrapped.Width > w {
		w = accentWrapped.Width
	}
	vl.Width = w
	if a.Bottom {
		vl.Height = bodyHL.Height
		vl.Depth = bodyHL.Depth + accentWrapped.Height + accentWrapped.Depth
	} else {
		// vl.Height must include the accent's own Depth so the body
		// baseline lands on the outer baseline. Walking children top-
		// to-bottom: accent (h+d) + gap + body (h+d). For body.baseline
		// to coincide with VList baseline (= top − vl.Height), we need
		// vl.Height = accent.h + accent.d + gap + body.h. With
		// gap = accentBaseH − accent.d − body.h, this simplifies to
		// vl.Height = accentBaseH + accent.h. The accent's depth is
		// absorbed by the gap reduction above.
		vl.Height = accentBaseH + accentWrapped.Height
		vl.Depth = bodyHL.Depth
	}

	out := node.NewHList()
	out.List = vl
	out.Width = vl.Width
	out.Height = vl.Height
	out.Depth = vl.Depth
	return out
}

// walkAllInternal is the production-side cousin of walkAll in walk_test.go —
// recursive traversal across HList/VList children, used by layout helpers
// that need to count or inspect glyphs in a sub-tree.
func walkAllInternal(root node.Node, fn func(node.Node)) {
	for n := root; n != nil; n = n.Next() {
		fn(n)
		switch x := n.(type) {
		case *node.HList:
			walkAllInternal(x.List, fn)
		case *node.VList:
			walkAllInternal(x.List, fn)
		}
	}
}
