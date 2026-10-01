package frontend

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// SettingHorizontalScale scales the advances, kerns and spaces of its run,
// so the line breaker sees the width the run takes on the page, and marks
// the glyphs so the writer draws them to match. The rest of the paragraph
// keeps its widths.
func TestSettingHorizontalScale(t *testing.T) {
	const scale = 0.9
	build := func(s float64) (glyphs []*node.Glyph, glues []*node.Glue, width bag.ScaledPoint) {
		fe, ff := lineModelDocument(t)
		run := NewText()
		if s != 1 {
			run.Settings[SettingHorizontalScale] = s
		}
		run.Items = append(run.Items, "AV To")
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Items = append(te.Items, run, "x")
		head, _, err := fe.Mknodes(te)
		if err != nil {
			t.Fatal(err)
		}
		node.Walk(head, func(n node.Node) bool {
			switch v := n.(type) {
			case *node.Glyph:
				glyphs = append(glyphs, v)
			case *node.Glue:
				glues = append(glues, v)
			}
			return true
		})
		return glyphs, glues, node.Hpack(head).Width
	}
	plain, plainGlue, plainWidth := build(1)
	scaled, scaledGlue, scaledWidth := build(scale)
	if len(plain) != len(scaled) || len(plainGlue) == 0 || len(plainGlue) != len(scaledGlue) {
		t.Fatalf("got %d and %d glyphs, %d and %d glues", len(plain), len(scaled), len(plainGlue), len(scaledGlue))
	}
	last := len(scaled) - 1
	for i := range last {
		if want := bag.MultiplyFloat(plain[i].Width, scale); scaled[i].Width != want || scaled[i].HorizontalScale != scale {
			t.Errorf("glyph %q: width %s, scale %g; want %s, %g", scaled[i].Components, scaled[i].Width, scaled[i].HorizontalScale, want, scale)
		}
	}
	if scaled[last].Width != plain[last].Width || scaled[last].HorizontalScale != 0 {
		t.Errorf("the glyph after the run is scaled: width %s, scale %g", scaled[last].Width, scaled[last].HorizontalScale)
	}
	if want := bag.MultiplyFloat(plainGlue[0].Width, scale); scaledGlue[0].Width != want {
		t.Errorf("space %s, want %s", scaledGlue[0].Width, want)
	}
	tail := plain[last].Width
	if got, want := (scaledWidth - tail).ToPT(), (plainWidth-tail).ToPT()*scale; got < want-0.01 || got > want+0.01 {
		t.Errorf("the run is %.3fpt wide, want %.3fpt", got, want)
	}
}
