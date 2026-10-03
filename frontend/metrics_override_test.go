package frontend

import (
	"encoding/binary"
	"io"
	"os"
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

// An override sets the content area with the ascent and descent, also when it
// pins a value to exactly the face's hhea one: with USE_TYPO_METRICS set, the
// typographic value must not take its place.
func TestMetricsOverrideContentArea(t *testing.T) {
	data, err := os.ReadFile(metricsTestFont)
	if err != nil {
		t.Fatal(err)
	}
	// Set USE_TYPO_METRICS, bit 7 of fsSelection at offset 62 of the OS/2
	// table, found in the table directory.
	for i := 0; i < int(binary.BigEndian.Uint16(data[4:])); i++ {
		rec := data[12+16*i:]
		if string(rec[:4]) == "OS/2" {
			data[int(binary.BigEndian.Uint32(rec[8:]))+63] |= 0x80
		}
	}
	file := t.TempDir() + "/typo.otf"
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	family := func(name string, m *MetricsOverride) *FontFamily {
		ff := fe.NewFontFamily(name)
		if err := ff.AddMember(&FontSource{Location: file, Metrics: m}, FontWeight400, FontStyleNormal); err != nil {
			t.Fatal(err)
		}
		return ff
	}
	pt := bag.ScaledPointFromFloat
	near := func(a, b bag.ScaledPoint) bool { return a-b < 2 && b-a < 2 }
	for _, c := range []struct {
		name         string
		m            *MetricsOverride
		ascent, desc float64
	}{
		{"no override", nil, 7.84, 2.16},
		// TeX Gyre Heros' hhea ascender is 1.148em.
		{"ascent pinned to the hhea value", &MetricsOverride{Ascent: 1.148, Descent: -1, LineGap: -1}, 11.48, 2.16},
		{"both overridden", &MetricsOverride{Ascent: 0.9, Descent: 0.25, LineGap: -1}, 9, 2.5},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := firstGlyphFont(t, fe, family(c.name, c.m))
			if !near(f.ContentAscent, pt(c.ascent)) || !near(f.ContentDescent, pt(c.desc)) {
				t.Errorf("content area %s + %s, want %vpt + %vpt", f.ContentAscent, f.ContentDescent, c.ascent, c.desc)
			}
		})
	}
}
