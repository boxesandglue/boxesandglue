package frontend

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// A family cut without an italic, asked to synthesise one, answers an italic
// request with its upright slanted by SyntheticSlant: one source for every
// lookup, and glyphs whose font carries the slant.
func TestMissingItalicIsSynthesised(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("upright only")
	ff.SetSynthesizeStyle(true)
	upright := &FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"}
	if err := ff.AddMember(upright, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	var logged bytes.Buffer
	prev := bag.Logger
	bag.Logger = slog.New(slog.NewTextHandler(&logged, nil))
	defer func() { bag.Logger = prev }()
	a, err := ff.GetFontSource(FontWeight400, FontStyleItalic)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := ff.GetFontSource(FontWeight400, FontStyleItalic)
	if a == upright || a != b || a.Slant != SyntheticSlant || upright.Slant != 0 {
		t.Fatalf("synthetic source: %+v (upright %+v)", a, upright)
	}
	if n, _ := ff.GetFontSource(FontWeight400, FontStyleNormal); n != upright {
		t.Error("the upright is still itself")
	}
	if n := strings.Count(logged.String(), "synthesized by slanting the upright"); n != 1 {
		t.Errorf("said %d times that the italic is synthesised, want once:\n%s", n, logged.String())
	}

	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	te.Settings[SettingStyle] = FontStyleItalic
	te.Items = append(te.Items, "slanted")
	vl, _, err := fe.FormatParagraph(te, bag.MustSP("200pt"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	var walk func(n node.Node)
	walk = func(n node.Node) {
		for ; n != nil; n = n.Next() {
			switch v := n.(type) {
			case *node.Glyph:
				found = true
				if v.Font.Slant != SyntheticSlant {
					t.Fatalf("glyph font slant %v", v.Font.Slant)
				}
			case *node.HList:
				walk(v.List)
			case *node.VList:
				walk(v.List)
			}
		}
	}
	walk(vl)
	if !found {
		t.Fatal("no glyphs")
	}
}

// By default a missing italic is the upright itself, as it always was.
func TestMissingItalicIsTheUprightByDefault(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("upright only")
	upright := &FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"}
	if err := ff.AddMember(upright, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	if s, err := ff.GetFontSource(FontWeight400, FontStyleItalic); err != nil || s != upright {
		t.Fatalf("italic: %+v, %v; want the upright", s, err)
	}
}

// An oblique takes the family's italic before a synthetic one, and a
// synthetic oblique shares its upright's face, so the font is embedded once
// even when it was loaded from data.
func TestSyntheticObliqueSources(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../qa/fonts/upem/fonts/texgyreheros-regular.otf")
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("from data")
	ff.SetSynthesizeStyle(true)
	upright := &FontSource{Data: data}
	if err := ff.AddMember(upright, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	oblique, err := ff.GetFontSource(FontWeight400, FontStyleOblique)
	if err != nil {
		t.Fatal(err)
	}
	a, err := fe.LoadFace(oblique)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := fe.LoadFace(upright); a != b {
		t.Error("the synthetic oblique loaded a face of its own")
	}

	italic := &FontSource{Data: data}
	if err := ff.AddMember(italic, FontWeight400, FontStyleItalic); err != nil {
		t.Fatal(err)
	}
	if s, _ := ff.GetFontSource(FontWeight400, FontStyleOblique); s != italic {
		t.Errorf("oblique: got %+v, want the family's italic", s)
	}
}

// SettingSynthesizeStyle belongs to the text, as CSS's font-synthesis-style
// belongs to the element: it wins over the family's default either way, and
// one paragraph's choice does not carry over to the next.
func TestSynthesizeStyleSetting(t *testing.T) {
	for _, familyDefault := range []bool{false, true} {
		fe, err := NewForWriter(io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		ff := fe.NewFontFamily("upright only")
		ff.SetSynthesizeStyle(familyDefault)
		if err := ff.AddMember(&FontSource{Location: metricsTestFont}, FontWeight400, FontStyleNormal); err != nil {
			t.Fatal(err)
		}
		for _, setting := range []any{nil, !familyDefault, familyDefault, !familyDefault} {
			te := NewText()
			te.Settings[SettingStyle] = FontStyleItalic
			if setting != nil {
				te.Settings[SettingSynthesizeStyle] = setting
			}
			want := familyDefault
			if setting != nil {
				want = setting.(bool)
			}
			if got := firstGlyphFontWith(t, fe, ff, te).Slant != 0; got != want {
				t.Errorf("family default %v, setting %v: slanted = %v, want %v", familyDefault, setting, got, want)
			}
		}
	}
}

// The slant is part of the font's cache key: the synthetic oblique and its
// upright share a face but not a font, and two slanted runs share one.
func TestFontKeySlant(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("upright only")
	if err := ff.AddMember(&FontSource{Location: metricsTestFont}, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	styled := func(style FontStyle) *Text {
		te := NewText()
		te.Settings[SettingStyle] = style
		te.Settings[SettingSynthesizeStyle] = true
		return te
	}
	upright := firstGlyphFontWith(t, fe, ff, styled(FontStyleNormal))
	a := firstGlyphFontWith(t, fe, ff, styled(FontStyleItalic))
	b := firstGlyphFontWith(t, fe, ff, styled(FontStyleOblique))
	if a == upright || a.Slant == 0 || upright.Slant != 0 {
		t.Errorf("the oblique shares the upright's font: slants %v, %v", a.Slant, upright.Slant)
	}
	if a.Face != upright.Face {
		t.Error("the oblique has a face of its own")
	}
	if a != b {
		t.Error("two slanted runs got separate fonts")
	}
}

func firstGlyphFontWith(t *testing.T, fe *Document, ff *FontFamily, te *Text) *font.Font {
	t.Helper()
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
