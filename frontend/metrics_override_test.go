package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
)

// A source's MetricsOverride replaces its face's ascent, descent and line gap
// on the fonts made from it, a negative value keeping the face's own, and the
// same face in a family without the override keeps its metrics.
func TestMetricsOverride(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	const file = metricsTestFont
	plain := fe.NewFontFamily("plain")
	if err := plain.AddMember(&FontSource{Location: file}, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	over := fe.NewFontFamily("overridden")
	src := &FontSource{Location: file, Metrics: &MetricsOverride{Ascent: 0.9, Descent: 0.25, LineGap: -1}}
	if err := over.AddMember(src, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	p, o := firstGlyphFont(t, fe, plain), firstGlyphFont(t, fe, over)
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

// Fonts are cached by face, size and effective metrics: an override that
// keeps every value is the face's own font, and two sources with the same
// override share one.
func TestFontKeyMetrics(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	family := func(name string, m *MetricsOverride) *FontFamily {
		ff := fe.NewFontFamily(name)
		if err := ff.AddMember(&FontSource{Location: metricsTestFont, Metrics: m}, FontWeight400, FontStyleNormal); err != nil {
			t.Fatal(err)
		}
		return ff
	}
	plain := firstGlyphFont(t, fe, family("plain", nil))
	keep := firstGlyphFont(t, fe, family("keep", &MetricsOverride{Ascent: -1, Descent: -2, LineGap: -1}))
	a := firstGlyphFont(t, fe, family("a", &MetricsOverride{Ascent: 0.9, Descent: -1, LineGap: -1}))
	b := firstGlyphFont(t, fe, family("b", &MetricsOverride{Ascent: 0.9, Descent: -5, LineGap: -1}))
	if keep != plain {
		t.Error("an override keeping every value got a font of its own")
	}
	if a != b {
		t.Error("two sources with the same override got separate fonts")
	}
	if a == plain {
		t.Error("an override shares the plain font")
	}
}

const metricsTestFont = "../qa/fonts/upem/fonts/texgyreheros-regular.otf"

func firstGlyphFont(t *testing.T, fe *Document, ff *FontFamily) *font.Font {
	t.Helper()
	return firstGlyphFontWith(t, fe, ff, NewText())
}
