package font_test

import (
	"io"
	"os"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/document"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/textshape/ot"
)

// TestInterwordSpaceFromFont checks that a font's interword glue comes from its
// own space glyph rather than from cmr10's fontdimen2. TeX Gyre Heros designs
// its space at 278/1000 em; the hardcoded default was 333/1000, so a 10pt font
// set every space 20% too wide.
func TestInterwordSpaceFromFont(t *testing.T) {
	const fontFile = "../../qa/fonts/upem/fonts/texgyreheros-regular.otf"
	doc := document.NewDocument(io.Discard)
	face, err := doc.LoadFace(fontFile, 0)
	if err != nil {
		t.Fatal(err)
	}

	size := bag.MustSP("10pt")
	fnt := font.NewFont(face, size)

	want := size * 278 / 1000
	if diff := fnt.Space - want; diff > bag.MustSP("0.01pt") || diff < -bag.MustSP("0.01pt") {
		t.Errorf("Space = %s, want %s (the font's own space advance)", fnt.Space, want)
	}
	if fnt.Space != fnt.SpaceChar.Advance {
		t.Errorf("Space = %s, SpaceChar.Advance = %s: they should agree", fnt.Space, fnt.SpaceChar.Advance)
	}
	if fnt.SpaceStretch != fnt.Space/2 {
		t.Errorf("SpaceStretch = %s, want %s", fnt.SpaceStretch, fnt.Space/2)
	}
	if fnt.SpaceShrink != fnt.Space/3 {
		t.Errorf("SpaceShrink = %s, want %s", fnt.SpaceShrink, fnt.Space/3)
	}
}

// TestAdvanceIsExactForAnyUpem checks that a shaped advance is the design
// advance scaled by size/upem without truncation. With an integer scale,
// 10pt (655350sp) over 2048 upem is 319 instead of 319.995, so every glyph
// came out 1/320 narrower than the PDF draws it.
func TestAdvanceIsExactForAnyUpem(t *testing.T) {
	const fontFile = "../../qa/fonts/upem/fonts/CrimsonPro-Regular2048.ttf"
	if _, err := os.Stat(fontFile); err != nil {
		t.Skipf("font fixture missing: %v", err)
	}

	doc := document.NewDocument(io.Discard)
	face, err := doc.LoadFace(fontFile, 0)
	if err != nil {
		t.Fatal(err)
	}
	size := bag.MustSP("10pt")
	fnt := font.NewFont(face, size)

	atoms := fnt.Shape("Hamburgefonstiv quick brown fox", nil, nil)
	var got, want float64
	for _, a := range atoms {
		got += float64(a.Advance)
		want += float64(face.OTFace().HorizontalAdvance(ot.GlyphID(a.Codepoint))) * float64(size) / float64(face.UnitsPerEM)
	}
	// Each advance truncates to a whole scaled point, so allow one per glyph.
	if d := want - got; d < -1 || d > float64(len(atoms)) {
		t.Errorf("shaped width = %.0fsp, want %.0fsp (off by %.0fsp)", got, want, d)
	}
}

// TestKernedWidthIsTheSameAtAnyUpem shapes kerned text in one design stored at
// 1024 and at 2048 upem. The kerned widths should agree; with an integer scale
// each upem lost a different fraction.
func TestKernedWidthIsTheSameAtAnyUpem(t *testing.T) {
	const text = "AVAWAYToWaVe LTAVAT"
	size := bag.MustSP("10pt")
	width := func(file string) (sum, kerns bag.ScaledPoint, n int) {
		doc := document.NewDocument(io.Discard)
		face, err := doc.LoadFace("../../qa/fonts/upem/fonts/"+file, 0)
		if err != nil {
			t.Fatal(err)
		}
		atoms := font.NewFont(face, size).Shape(text, nil, nil)
		for _, a := range atoms {
			sum += a.Advance + a.Kernafter
			kerns += a.Kernafter
		}
		return sum, kerns, len(atoms)
	}
	w1, k1, n := width("CrimsonPro-Regular.ttf")
	w2, k2, _ := width("CrimsonPro-Regular2048.ttf")
	if k1 == 0 || k2 == 0 {
		t.Fatalf("no kerning in %q (kerns %d, %d): the test needs kerned pairs", text, k1, k2)
	}
	// Each glyph's advance and kern truncate to a whole scaled point.
	if d := w1 - w2; d < -bag.ScaledPoint(2*n) || d > bag.ScaledPoint(2*n) {
		t.Errorf("kerned width at 1024 upem = %s, at 2048 upem = %s (off by %dsp)", w1, w2, d)
	}
}
