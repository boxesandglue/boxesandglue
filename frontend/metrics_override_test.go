package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// A source's MetricsOverride replaces its face's ascent, descent and line gap
// on the fonts made from it, a negative value keeping the face's own, and the
// same face in a family without the override keeps its metrics.
func TestMetricsOverride(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	const file = "../qa/fonts/upem/fonts/texgyreheros-regular.otf"
	plain := fe.NewFontFamily("plain")
	if err := plain.AddMember(&FontSource{Location: file}, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	over := fe.NewFontFamily("overridden")
	src := &FontSource{Location: file, Metrics: &MetricsOverride{Ascent: 0.9, Descent: 0.25, LineGap: -1}}
	if err := over.AddMember(src, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	fontOf := func(ff *FontFamily) *font.Font {
		t.Helper()
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Items = append(te.Items, "x")
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("100pt"))
		if err != nil {
			t.Fatal(err)
		}
		for n := vl.List; n != nil; n = n.Next() {
			if hl, ok := n.(*node.HList); ok {
				for m := hl.List; m != nil; m = m.Next() {
					if g, ok := m.(*node.Glyph); ok {
						return g.Font
					}
				}
			}
		}
		t.Fatal("no glyph")
		return nil
	}
	p, o := fontOf(plain), fontOf(over)
	if p == o {
		t.Fatal("the overridden source shares the plain font")
	}
	if want := bag.MustSP("9pt"); o.Ascent != want {
		t.Errorf("Ascent = %s, want %s", o.Ascent, want)
	}
	if want := bag.MustSP("2.5pt"); o.Descent != want {
		t.Errorf("Descent = %s, want %s", o.Descent, want)
	}
	if o.LineGap != p.LineGap {
		t.Errorf("LineGap = %s, want the face's %s", o.LineGap, p.LineGap)
	}
	if p.Ascent == o.Ascent || p.Descent == o.Descent {
		t.Errorf("the plain font took the override: ascent %s, descent %s", p.Ascent, p.Descent)
	}
}
