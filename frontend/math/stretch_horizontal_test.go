package math

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/node"
)

// TestStretchedHorizontalArrow — requesting an arrow at three times its
// natural width must produce a wider box: a pre-built variant (single
// glyph with larger advance) or a multi-glyph assembly.
func TestStretchedHorizontalArrow(t *testing.T) {
	fnt := loadMathFont(t)
	arrowGid := glyphFor(t, fnt, '←')

	c := fnt.MathConstantsFU()
	if c == nil {
		t.Fatal("no MATH constants")
	}
	ctx := newEngineCtx(fnt, c)
	natural := wrapGlyphInHList(buildGlyph(ctx, arrowGid, TextStyle))
	target := natural.Width * 3
	upem := upemOf(fnt)

	stretched := stretchedHorizontal(ctx, arrowGid, spToFU(target, fnt.Size, upem), TextStyle)
	if stretched.Width < target*9/10 {
		t.Errorf("stretched arrow width %v below 90%% of target %v", stretched.Width, target)
	}
}

// TestPlaceLimitsStretchesUnderArrow — <munder><mi>lim</mi><mo>←</mo></munder>
// semantics: the arrow beneath "lim" is stretched to at least the base
// width instead of staying at its natural (narrower or equal) size, and
// the stacked result centers everything in one VList.
func TestPlaceLimitsStretchesUnderArrow(t *testing.T) {
	fnt := loadMathFont(t)
	lGid := glyphFor(t, fnt, 'l')
	iGid := glyphFor(t, fnt, 'i')
	mGid := glyphFor(t, fnt, 'm')
	arrowGid := glyphFor(t, fnt, '←')

	arrow := &MathAtom{Class: ClassRel, Stretchy: true, Nucleus: MathField{Glyph: arrowGid}}
	base := &MathAtom{
		Class:   ClassOrd,
		Scripts: ScriptsLimits,
		Nucleus: MathField{Sublist: []MathItem{Ord(lGid), Ord(iGid), Ord(mGid)}},
		Sub:     MathField{Sublist: []MathItem{arrow}},
	}
	hl, err := DisplayMath(fnt, base)
	if err != nil {
		t.Fatalf("DisplayMath: %v", err)
	}

	plain, err := DisplayMath(fnt, Ord(lGid), Ord(iGid), Ord(mGid))
	if err != nil {
		t.Fatalf("DisplayMath (lim): %v", err)
	}
	// The overall width must be at least the base width (the arrow no
	// longer collapses to its narrow natural size next to the base).
	if hl.Width < plain.Width {
		t.Errorf("stacked box narrower than its base: %v < %v", hl.Width, plain.Width)
	}
	hasVList := false
	walkAll(hl.List, func(n node.Node) {
		if _, ok := n.(*node.VList); ok {
			hasVList = true
		}
	})
	if !hasVList {
		t.Errorf("expected stacked VList result")
	}

	// The non-stretchy control: same layout with a plain (non-stretchy)
	// arrow must not be wider than the stretched one.
	plainArrow := &MathAtom{Class: ClassRel, Nucleus: MathField{Glyph: arrowGid}}
	baseCtl := &MathAtom{
		Class:   ClassOrd,
		Scripts: ScriptsLimits,
		Nucleus: MathField{Sublist: []MathItem{Ord(lGid), Ord(iGid), Ord(mGid)}},
		Sub:     MathField{Sublist: []MathItem{plainArrow}},
	}
	ctl, err := DisplayMath(fnt, baseCtl)
	if err != nil {
		t.Fatalf("DisplayMath (control): %v", err)
	}
	if hl.Width < ctl.Width {
		t.Errorf("stretchy arrow result narrower than non-stretchy control: %v < %v", hl.Width, ctl.Width)
	}
}
