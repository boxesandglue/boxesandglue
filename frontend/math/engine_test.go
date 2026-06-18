package math

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

// loadMathFont loads testdata/latinmodern-math.otf for the integration
// tests. If the font is not present (the typical CI / fresh-checkout
// case), the test calling this helper is skipped instead of failing —
// the spec-shape tests don't depend on the font and stay green either way.
func loadMathFont(t *testing.T) *font.Font {
	t.Helper()
	path := filepath.Join("testdata", "latinmodern-math.otf")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("math font not available at %s — see testdata/README.md", path)
	}
	pw := pdf.NewPDFWriter(io.Discard)
	face, err := pw.LoadFace(path, 0)
	if err != nil {
		t.Fatalf("LoadFace(%s): %v", path, err)
	}
	return font.NewFont(face, bag.MustSP("10pt"))
}

// glyphFor resolves a single rune to its glyph id via the text shaper.
// The math tests use this to bootstrap atom trees from human-readable
// characters — `glyphFor(t, fnt, 'x')` is much nicer to read than a
// hardcoded glyph index.
func glyphFor(t *testing.T, fnt *font.Font, r rune) ot.GlyphID {
	t.Helper()
	atoms := fnt.Shape(string(r), nil, nil)
	if len(atoms) == 0 {
		t.Fatalf("Shape(%q) returned no atoms", string(r))
	}
	return ot.GlyphID(atoms[0].Codepoint)
}

// glyphWidth returns the advance (in scaled points) the math engine
// assigns to a glyph at the given style — matches buildGlyph()'s
// calculation so the assertions are easy to spell.
func glyphWidth(fnt *font.Font, gid ot.GlyphID) bag.ScaledPoint {
	advFU := int64(fnt.Face.Shaper.GetGlyphHAdvanceVar(gid))
	upem := int64(fnt.Face.UnitsPerEM)
	return bag.ScaledPoint(advFU * int64(fnt.Size) / upem)
}

// TestSimpleOrdOrd — `x y`: width must be exactly the sum of the two
// glyph advances; no kern is inserted (Ord/Ord = 0 in the spacing table).
func TestSimpleOrdOrd(t *testing.T) {
	fnt := loadMathFont(t)
	xGid := glyphFor(t, fnt, 'x')
	yGid := glyphFor(t, fnt, 'y')
	hl, err := InlineMath(fnt, Ord(xGid), Ord(yGid))
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}
	want := glyphWidth(fnt, xGid) + glyphWidth(fnt, yGid)
	if hl.Width != want {
		t.Errorf("HList.Width = %d sp, want %d sp (x+y, no kern)", hl.Width, want)
	}
	// Walk the list to confirm no Kern between the two glyphs.
	for n := hl.List; n != nil; n = n.Next() {
		if _, isKern := n.(*node.Kern); isKern {
			t.Errorf("unexpected Kern in Ord/Ord list — table says 0 between Ord and Ord")
		}
	}
}

// TestBinRewriteFirstPos — `+ x`: a leading Bin must be reclassified to
// Ord. Result: no Med-Space appears before the x.
func TestBinRewriteFirstPos(t *testing.T) {
	fnt := loadMathFont(t)
	plusGid := glyphFor(t, fnt, '+')
	xGid := glyphFor(t, fnt, 'x')
	hl, err := InlineMath(fnt, Bin(plusGid), Ord(xGid))
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}
	// Total width should be plus + x advance with no Med-Space between.
	want := glyphWidth(fnt, plusGid) + glyphWidth(fnt, xGid)
	if hl.Width != want {
		t.Errorf("Width = %d, want %d (no med-space because leading Bin → Ord)", hl.Width, want)
	}
}

// TestOrdBinOrd — `x + y`: Bin stays Bin, the spacing table emits
// med-space on each side of it. Total width = x + 4mu + plus + 4mu + y.
func TestOrdBinOrd(t *testing.T) {
	fnt := loadMathFont(t)
	xGid := glyphFor(t, fnt, 'x')
	yGid := glyphFor(t, fnt, 'y')
	plusGid := glyphFor(t, fnt, '+')
	hl, err := InlineMath(fnt, Ord(xGid), Bin(plusGid), Ord(yGid))
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}
	// Engine computes 4mu as int64(size) * 4 / 18 (single division). Do the
	// same in the test — `mu(size) * 4` would round twice and miss by 1-3 sp.
	med := bag.ScaledPoint(int64(fnt.Size) * 4 / 18)
	want := glyphWidth(fnt, xGid) + med + glyphWidth(fnt, plusGid) + med + glyphWidth(fnt, yGid)
	if hl.Width != want {
		t.Errorf("Width = %d, want %d (x + medkern + plus + medkern + y)", hl.Width, want)
	}
}

// TestSubscriptShiftDown — `x_i`: the subscript glyph must end up shifted
// down by approximately SubscriptShiftDown FUnits (modulo height/depth
// tweaks). We assert the YOffset on the subscript glyph is negative and
// at least the SubscriptShiftDown amount.
func TestSubscriptShiftDown(t *testing.T) {
	fnt := loadMathFont(t)
	c := fnt.MathConstantsFU()
	if c == nil {
		t.Fatal("font reports no MATH table")
	}
	xGid := glyphFor(t, fnt, 'x')
	iGid := glyphFor(t, fnt, 'i')
	atom := Ord(xGid).WithSubGlyph(iGid)
	hl, err := InlineMath(fnt, atom)
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}
	// The engine nests HLists: outer (from spliceWithSpacing) → atom HList
	// (from placeSubSup) → nucleus HList and sub HList (from wrapGlyphInHList).
	// Recursive walk to collect all leaf glyphs.
	var glyphs []*node.Glyph
	var walk func(n node.Node)
	walk = func(n node.Node) {
		for ; n != nil; n = n.Next() {
			switch x := n.(type) {
			case *node.Glyph:
				glyphs = append(glyphs, x)
			case *node.HList:
				walk(x.List)
			case *node.VList:
				walk(x.List)
			}
		}
	}
	walk(hl.List)
	if len(glyphs) < 2 {
		t.Fatalf("expected at least 2 glyphs, got %d", len(glyphs))
	}
	subYOffset := glyphs[1].YOffset
	if subYOffset >= 0 {
		t.Errorf("subscript YOffset = %d sp, want negative (= shifted down)", subYOffset)
	}
	minDrop := fuToSP(c.SubscriptShiftDown, fnt.Size, int(fnt.Face.UnitsPerEM))
	if -subYOffset < minDrop {
		t.Errorf("subscript drop = %d sp, want at least SubscriptShiftDown = %d sp", -subYOffset, minDrop)
	}
}

// TestSubSuperGap — `x_i^j`: when both sub and sup are present, the gap
// between sup.bottom and sub.top must be at least SubSuperscriptGapMin.
// The test reads the YOffsets of the two scripts and asserts the gap
// constraint holds at or above the spec minimum.
func TestSubSuperGap(t *testing.T) {
	fnt := loadMathFont(t)
	c := fnt.MathConstantsFU()
	xGid := glyphFor(t, fnt, 'x')
	iGid := glyphFor(t, fnt, 'i')
	jGid := glyphFor(t, fnt, 'j')

	atom := Ord(xGid).WithSubGlyph(iGid).WithSupGlyph(jGid)
	hl, err := InlineMath(fnt, atom)
	if err != nil {
		t.Fatalf("InlineMath: %v", err)
	}

	var glyphs []*node.Glyph
	var walk func(n node.Node)
	walk = func(n node.Node) {
		for ; n != nil; n = n.Next() {
			switch x := n.(type) {
			case *node.Glyph:
				glyphs = append(glyphs, x)
			case *node.HList:
				walk(x.List)
			case *node.VList:
				walk(x.List)
			}
		}
	}
	walk(hl.List)
	if len(glyphs) < 3 {
		t.Fatalf("expected at least 3 glyphs, got %d", len(glyphs))
	}

	// Identify which glyph is which by codepoint.
	var supG, subG *node.Glyph
	for _, g := range glyphs {
		switch g.Codepoint {
		case int(iGid):
			subG = g
		case int(jGid):
			supG = g
		}
	}
	if supG == nil || subG == nil {
		t.Fatalf("could not find sub (i) and sup (j) glyphs in the output")
	}

	// Read the REAL glyph heights/depths from the node — the engine now
	// uses per-glyph extents (CFF/glyf) instead of the pauschal model,
	// so a hand-computed `Size − Depth` would mis-measure the gap for
	// letters whose actual yMax is below ascender height.
	supBottom := supG.YOffset - supG.Depth
	subTop := subG.YOffset + subG.Height
	gap := supBottom - subTop
	gapMin := fuToSP(c.SubSuperscriptGapMin, fnt.Size, int(fnt.Face.UnitsPerEM))
	if gap < gapMin {
		t.Errorf("sub/sup gap = %d sp, want >= SubSuperscriptGapMin (%d sp)", gap, gapMin)
	}
}
