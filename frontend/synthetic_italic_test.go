package frontend

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
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
	ff.SetSynthesizeItalic(true)
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
	if n := strings.Count(logged.String(), "synthesised by slanting the upright"); n != 1 {
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
	ff.SetSynthesizeItalic(true)
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
