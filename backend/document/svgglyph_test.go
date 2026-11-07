package document

import (
	"bytes"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// An SVG glyph is in font units. With a user unit of 0.75pt in svgreader the
// outer scale takes the factor back out, so the glyph keeps its size: one em
// of the glyph document is the font size.
func TestSVGGlyphKeepsItsSize(t *testing.T) {
	oc, out, g := glyphContext(t)
	upem := float64(g.Font.Face.UnitsPerEM)
	oc.emitColorSVGGlyph(g, 0, 0, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>`))
	got := out.String()
	// q s 0 0 s x y cm, then the renderer's 0.75 0 0 -0.75 0 0 cm.
	var outer, inner float64
	for _, line := range strings.Split(got, "\n") {
		f := strings.Fields(line)
		if len(f) == 8 && f[0] == "q" && f[7] == "cm" {
			outer = mustFloat(t, f[1])
		}
		if len(f) == 7 && f[6] == "cm" && f[2] == "0" && outer != 0 && inner == 0 {
			inner = mustFloat(t, f[0])
		}
	}
	if want := 12 / upem; outer == 0 || inner == 0 || math.Abs(outer*inner-want) > 1e-9 {
		t.Errorf("font units scale by %g × %g = %g, want %g:\n%s", outer, inner, outer*inner, want, got)
	}
}

func mustFloat(t *testing.T, s string) float64 {
	t.Helper()
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// glyphContext returns an objectContext in a text object with a glyph of
// Crimson Pro at 12pt, and the buffer it writes to.
func glyphContext(t *testing.T) (*objectContext, *bytes.Buffer, *node.Glyph) {
	t.Helper()
	var doc, out bytes.Buffer
	d := NewDocument(&doc)
	face, err := d.LoadFace("../../qa/fonts/upem/fonts/CrimsonPro-Regular.ttf", 0)
	if err != nil {
		t.Fatal(err)
	}
	g := node.NewGlyph()
	g.Font = font.NewFont(face, bag.MustSP("12pt"))
	g.Codepoint = 5
	oc := &objectContext{s: &out, textmode: ScopeText, p: &Page{document: d}}
	oc.text.font = g.Font
	return oc, &out, g
}
