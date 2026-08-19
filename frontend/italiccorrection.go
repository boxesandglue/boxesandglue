package frontend

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

// applyItalicCorrection inserts a small kern wherever a glyph of a slanted
// font directly abuts a glyph of an upright font (and vice versa) — the
// classic italic-correction heuristic. OpenType fonts carry no text
// italic-correction metric, so the kern is derived from the boundary
// glyph's outline: leaving italics, the amount the last slanted glyph's
// ink overhangs its advance width; entering italics, the amount the first
// slanted glyph's ink underhangs its origin. Pairs separated by glue
// (i.e. a word space) need no correction and are left alone.
func applyItalicCorrection(head node.Node) {
	for n := head; n != nil; n = n.Next() {
		g, ok := n.(*node.Glyph)
		if !ok {
			continue
		}
		next, ok := n.Next().(*node.Glyph)
		if !ok || g.Font == nil || next.Font == nil || g.Font.Face == next.Font.Face {
			continue
		}
		prevSlanted := isSlantedFont(g.Font)
		nextSlanted := isSlantedFont(next.Font)
		var corr bag.ScaledPoint
		switch {
		case prevSlanted && !nextSlanted:
			corr = inkRightOverhang(g)
		case !prevSlanted && nextSlanted:
			corr = inkLeftOverhang(next)
		}
		if corr > 0 {
			k := node.NewKern()
			k.Kern = corr
			node.InsertAfter(head, n, k)
			n = k
		}
	}
}

func isSlantedFont(fnt *font.Font) bool {
	if fnt == nil || fnt.Face == nil {
		return false
	}
	if otf := fnt.Face.OTFace(); otf != nil {
		return otf.ItalicAngle() != 0
	}
	return false
}

// inkRightOverhang returns how far the glyph's ink reaches beyond its
// advance width, scaled to the glyph's font size. Zero when the ink stays
// inside the advance.
func inkRightOverhang(g *node.Glyph) bag.ScaledPoint {
	fnt := g.Font
	otf := fnt.Face.OTFace()
	if otf == nil || fnt.Face.Shaper == nil {
		return 0
	}
	upem := int64(fnt.Face.UnitsPerEM)
	if upem == 0 {
		return 0
	}
	gid := ot.GlyphID(g.Codepoint)
	bbox, ok := otf.GlyphExtents(gid)
	if !ok {
		return 0
	}
	over := int64(bbox.XMax) - int64(fnt.Face.Shaper.GetGlyphHAdvanceVar(gid))
	if over <= 0 {
		return 0
	}
	return bag.ScaledPoint(over * int64(fnt.Size) / upem)
}

// inkLeftOverhang returns how far the glyph's ink reaches left of its
// origin, scaled to the glyph's font size.
func inkLeftOverhang(g *node.Glyph) bag.ScaledPoint {
	fnt := g.Font
	otf := fnt.Face.OTFace()
	if otf == nil {
		return 0
	}
	upem := int64(fnt.Face.UnitsPerEM)
	if upem == 0 {
		return 0
	}
	bbox, ok := otf.GlyphExtents(ot.GlyphID(g.Codepoint))
	if !ok {
		return 0
	}
	over := -int64(bbox.XMin)
	if over <= 0 {
		return 0
	}
	return bag.ScaledPoint(over * int64(fnt.Size) / upem)
}
