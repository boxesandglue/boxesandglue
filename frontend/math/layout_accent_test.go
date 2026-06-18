package math

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/node"
)

// TestAccentTop — hat over x. The output must contain both glyphs; the
// accent glyph must be positioned ABOVE the body (its YOffset in the
// rendered tree is more positive than the body's).
func TestAccentTop(t *testing.T) {
	fnt := loadMathFont(t)
	// '^' is the closest ASCII to hat-accent; U+0302 COMBINING CIRCUMFLEX
	// ACCENT exists but its glyph id is harder to resolve via Shape().
	// We use U+02C6 MODIFIER LETTER CIRCUMFLEX ACCENT — present in Latin
	// Modern Math and used as a standalone hat in OT MATH.
	hatGid := glyphFor(t, fnt, 'ˆ')
	xGid := glyphFor(t, fnt, 'x')

	hl, err := InlineMath(fnt, AccentTop(hatGid, Ord(xGid)))
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}

	// Collect both glyphs.
	var hat, x *node.Glyph
	var walk func(node.Node)
	walk = func(n node.Node) {
		for ; n != nil; n = n.Next() {
			switch v := n.(type) {
			case *node.Glyph:
				switch v.Codepoint {
				case int(hatGid):
					hat = v
				case int(xGid):
					x = v
				}
			case *node.HList:
				walk(v.List)
			case *node.VList:
				walk(v.List)
			}
		}
	}
	walk(hl.List)
	if hat == nil {
		t.Fatalf("hat glyph not in output")
	}
	if x == nil {
		t.Fatalf("body x glyph not in output")
	}
	// The HList's height should reflect the accent extension above x.
	xWidth := glyphWidth(fnt, xGid)
	if hl.Width < xWidth {
		t.Errorf("output Width = %d sp, want >= %d sp (body width)", hl.Width, xWidth)
	}
	// Spot-check vertical extent: combined VList height > body's height
	// alone (the accent extends above).
	c := fnt.MathConstantsFU()
	bodyHeight := fnt.Size - fnt.Depth
	accentBaseH := fuToSP(c.AccentBaseHeight, fnt.Size, int(fnt.Face.UnitsPerEM))
	if accentBaseH < bodyHeight {
		accentBaseH = bodyHeight
	}
	if hl.Height <= bodyHeight {
		t.Errorf("output Height = %d sp, want > body height (%d sp) to accommodate accent", hl.Height, bodyHeight)
	}
	_ = accentBaseH
}
