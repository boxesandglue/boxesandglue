package font_test

import (
	"io"
	"runtime"
	"testing"
	"weak"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/document"
	"github.com/boxesandglue/boxesandglue/backend/font"
)

// TestGlyphHasInkKeepsNoFace checks that asking a font for the ink of its
// space does not keep the face alive. A global cache keyed by the face held
// every document a process ever made, with its fonts and compressors.
func TestGlyphHasInkKeepsNoFace(t *testing.T) {
	face := func() weak.Pointer[pdf.Face] {
		doc := document.NewDocument(io.Discard)
		face, err := doc.LoadFace("../../qa/fonts/upem/fonts/texgyreheros-regular.otf", 0)
		if err != nil {
			t.Fatal(err)
		}
		fnt := font.NewFont(face, bag.MustSP("10pt"))
		if fnt.GlyphHasInk(fnt.SpaceChar.Codepoint) {
			t.Error("the space of TeX Gyre Heros has ink")
		}
		return weak.Make(face)
	}()
	runtime.GC()
	runtime.GC()
	if face.Value() != nil {
		t.Error("the face is still reachable after the document is gone")
	}
}
