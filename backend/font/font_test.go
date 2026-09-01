package font_test

import (
	"io"
	"os"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/document"
	"github.com/boxesandglue/boxesandglue/backend/font"
)

// TestInterwordSpaceFromFont checks that a font's interword glue comes from its
// own space glyph rather than from cmr10's fontdimen2. TeX Gyre Heros designs
// its space at 278/1000 em; the hardcoded default was 333/1000, so a 10pt font
// set every space 20% too wide.
func TestInterwordSpaceFromFont(t *testing.T) {
	const fontFile = "../../qa/fonts/upem/fonts/texgyreheros-regular.otf"
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
