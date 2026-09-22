package document

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// renderGlyphs writes a single line of glyphs to an uncompressed PDF and
// returns the content stream text. adjust lets the caller change the
// glyphs after shaping, the way the line breaker does for hanging
// punctuation.
func renderGlyphs(t *testing.T, text string, adjust func([]*node.Glyph)) (string, *font.Font, []font.Atom) {
	t.Helper()
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.CompressLevel = 0
	face, err := d.LoadFace("../../qa/fonts/upem/fonts/CrimsonPro-Regular.ttf", 0)
	if err != nil {
		t.Fatal(err)
	}
	fnt := font.NewFont(face, bag.MustSP("12pt"))
	atoms := fnt.Shape(text, nil, nil)
	var head, cur node.Node
	var glyphs []*node.Glyph
	for _, a := range atoms {
		g := node.NewGlyph()
		g.Font = fnt
		g.Codepoint = a.Codepoint
		g.Components = a.Components
		g.Width = a.Advance
		g.XOffset = a.XOffset
		glyphs = append(glyphs, g)
		head = node.InsertAfter(head, cur, g)
		cur = g
	}
	if adjust != nil {
		adjust(glyphs)
	}
	hl := node.Hpack(head)
	vl := node.Vpack(hl)
	p := d.NewPage()
	p.OutputAt(bag.MustSP("1cm"), bag.MustSP("10cm"), vl)
	p.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	return buf.String(), fnt, atoms
}

// tjUnits converts a scaled point distance into TJ array units (1/1000 of
// the text space) for the given font size.
func tjUnits(wd, size bag.ScaledPoint) int {
	return int(math.Round(1000 * wd.ToPT() / size.ToPT()))
}

// advanceTJ is the exact advance of the glyph in TJ array units, the
// distance the PDF moves after showing it.
func advanceTJ(fnt *font.Font, gid int) float64 {
	return fnt.Face.AdvanceWidth(gid) * 1000 / fnt.Face.Scale
}

// A glyph that keeps its shaped advance must not produce a TJ adjustment,
// otherwise every ordinary text run would fill up with corrections.
func TestGlyphWithShapedWidthEmitsNoAdjustment(t *testing.T) {
	out, _, atoms := renderGlyphs(t, "AV", nil)
	want := fmt.Sprintf("[<%04x%04x>]TJ", atoms[0].Codepoint, atoms[1].Codepoint)
	if !strings.Contains(out, want) {
		t.Errorf("want %q in the content stream, got:\n%s", want, tjLines(out))
	}
}

// A zero width glyph (hanging punctuation, left to right) leaves the PDF's
// text position where the glyph started: the TJ array moves back by the
// font advance right after the glyph.
func TestZeroWidthGlyphTakesTheAdvanceBack(t *testing.T) {
	out, fnt, atoms := renderGlyphs(t, "A.", func(g []*node.Glyph) {
		g[1].Width = 0
	})
	back := int(math.Round(advanceTJ(fnt, atoms[1].Codepoint)))
	want := fmt.Sprintf("[<%04x%04x> %d ]TJ", atoms[0].Codepoint, atoms[1].Codepoint, back)
	if !strings.Contains(out, want) {
		t.Errorf("want %q in the content stream, got:\n%s", want, tjLines(out))
	}
}

// Hanging punctuation in a right to left line: zero width and an XOffset of
// minus the advance. The shift before the glyph and the width correction
// after it cancel out, so only the leading shift remains. Shift and
// correction both round to whole TJ units, so a leftover unit can only
// appear when the shaped width and the exact advance straddle a rounding
// boundary; the expectation accounts for that.
func TestZeroWidthGlyphWithXOffsetShiftsOnce(t *testing.T) {
	out, fnt, atoms := renderGlyphs(t, ".A", func(g []*node.Glyph) {
		g[0].XOffset = -g[0].Width
		g[0].Width = 0
	})
	shift := tjUnits(fnt.GlyphAdvance(atoms[0].Codepoint), fnt.Size)
	rest := ""
	if left := int(math.Round(advanceTJ(fnt, atoms[0].Codepoint) - float64(shift))); left != 0 {
		rest = fmt.Sprintf(" %d ", left)
	}
	want := fmt.Sprintf("[ %d <%04x>%s<%04x>]TJ", shift, atoms[0].Codepoint, rest, atoms[1].Codepoint)
	if rest == "" {
		want = fmt.Sprintf("[ %d <%04x%04x>]TJ", shift, atoms[0].Codepoint, atoms[1].Codepoint)
	}
	if !strings.Contains(out, want) {
		t.Errorf("want %q in the content stream, got:\n%s", want, tjLines(out))
	}
}

func tjLines(out string) string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "TJ") {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}
