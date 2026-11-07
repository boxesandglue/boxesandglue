package document

import (
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// A color glyph extracts as its source characters (issue #104): it is drawn
// in a marked-content sequence with ActualText, which opens outside the text
// object, as the glyph itself is drawn there.

func TestActualTextSpan(t *testing.T) {
	oc, out, g := glyphContext(t)
	// U+1F469 U+200D U+1F4BB, a ZWJ sequence: one glyph, three characters.
	g.Components = "\U0001F469‍\U0001F4BB"
	if !oc.beginActualText(g) {
		t.Fatal("no sequence opened for a glyph with components")
	}
	oc.endActualText(true)
	got := out.String()
	want := "/Span <</ActualText <feffd83ddc69200dd83ddcbb>>> BDC"
	if !strings.Contains(got, want) {
		t.Errorf("%q is missing in\n%s", want, got)
	}
	// The text object ends before the sequence opens.
	if et, bdc := strings.Index(got, "ET"), strings.Index(got, "BDC"); et < 0 || et > bdc {
		t.Errorf("the sequence opens inside the text object:\n%s", got)
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "EMC") {
		t.Errorf("the sequence is not closed:\n%s", got)
	}
	if oc.textmode != ScopePage {
		t.Errorf("text mode %d after the sequence, want ScopePage", oc.textmode)
	}
}

func TestActualTextWithoutComponents(t *testing.T) {
	oc, out, g := glyphContext(t)
	g.Components = ""
	if oc.beginActualText(g) {
		t.Error("a sequence opened for a glyph without components")
	}
	oc.endActualText(false)
	if strings.Contains(out.String(), "BDC") || strings.Contains(out.String(), "EMC") {
		t.Errorf("marked content written:\n%s", out.String())
	}
}

// A bitmap or SVG glyph shows its base glyph as invisible text, so that an
// extractor finds text in the sequence, and the rendering mode is reset.
func TestInvisibleBaseGlyph(t *testing.T) {
	oc, out, g := glyphContext(t)
	oc.gotoTextMode(ScopePage)
	out.Reset()
	oc.showInvisibleGlyph(g, bag.MustSP("10pt"), bag.MustSP("20pt"))
	got := out.String()
	tr3, glyph, tr0 := strings.Index(got, "3 Tr"), strings.Index(got, "0005"), strings.Index(got, "0 Tr")
	if tr3 < 0 || glyph < tr3 || tr0 < glyph {
		t.Errorf("want 3 Tr, the glyph, 0 Tr in that order:\n%s", got)
	}
	if !strings.Contains(got, "BT") || !strings.Contains(got, "ET") {
		t.Errorf("the glyph is not in a text object of its own:\n%s", got)
	}
	if oc.textmode != ScopePage {
		t.Errorf("text mode %d after the glyph, want ScopePage", oc.textmode)
	}
}
