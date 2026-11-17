package frontend

import (
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// hyphenKernLines sets text in TeX Gyre Heros at 60pt on 150pt in English
// and returns the lines.
func hyphenKernLines(t *testing.T, text string, settings TypesettingSettings) []*node.HList {
	t.Helper()
	fe, ff := lineModelDocument(t)
	l, err := GetLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	fe.Doc.DefaultLanguage = l
	te := NewText()
	te.Settings[SettingHAlign] = HAlignLeft
	for k, v := range settings {
		te.Settings[k] = v
	}
	te.Items = []any{text}
	vl, _, err := fe.FormatParagraph(te, bag.MustSP("150pt"), FontSize(bag.MustSP("60pt")), Family(ff))
	if err != nil {
		t.Fatal(err)
	}
	var lines []*node.HList
	for n := vl.List; n != nil; n = n.Next() {
		if hl, ok := n.(*node.HList); ok {
			lines = append(lines, hl)
		}
	}
	return lines
}

// lineEnd writes the glyphs and kerns at the end of a line, from the last
// glyph before the hyphen on.
func lineEnd(hl *node.HList) (string, *node.Glyph) {
	var parts []string
	var before *node.Glyph
	for n := hl.List; n != nil; n = n.Next() {
		switch v := n.(type) {
		case *node.Glyph:
			if v.Components == "-" {
				parts = append(parts, "-")
			} else {
				parts = []string{v.Components}
				before = v
			}
		case *node.Kern:
			if before != nil {
				parts = append(parts, "kern "+v.Kern.String())
			}
		}
	}
	return strings.Join(parts, " "), before
}

// A line that ends with a hyphen has the kern of the font between the glyph
// before the break and the hyphen, and not the kern between that glyph and
// the next one on the next line. In Heros r and hyphen kern, r and s too
// (bag#109).
func TestHyphenKernsWithGlyphBefore(t *testing.T) {
	for _, word := range []string{"international", "person", "inter­national"} {
		lines := hyphenKernLines(t, word, nil)
		if len(lines) < 2 {
			t.Fatalf("%s: %d lines, want a break", word, len(lines))
		}
		got, r := lineEnd(lines[0])
		if r == nil || r.Components != "r" {
			t.Fatalf("%s: first line ends %q, want r and a hyphen", word, got)
		}
		atoms := r.Font.Shape("r-", nil, nil)
		if len(atoms) != 2 || atoms[0].Kernafter == 0 {
			t.Fatalf("Heros has no kern between r and hyphen: %+v", atoms)
		}
		if want := "r kern " + atoms[0].Kernafter.String() + " -"; got != want {
			t.Errorf("%s: first line ends %q, want %q", word, got, want)
		}
	}
}

// With hanging punctuation the hyphen hangs into the margin, the glyph
// before it does not, and the kern between them goes with the hyphen.
func TestHangingHyphenAfterKern(t *testing.T) {
	lines := hyphenKernLines(t, "person", TypesettingSettings{SettingHangingPunctuation: HangingPunctuation(HangingPunctuationAllowEnd)})
	if len(lines) < 2 {
		t.Fatalf("%d lines, want a break", len(lines))
	}
	var wd bag.ScaledPoint
	var text strings.Builder
	var fnt *font.Font
	for n := lines[0].List; n != nil; n = n.Next() {
		switch v := n.(type) {
		case *node.Glyph:
			wd += v.Width
			text.WriteString(v.Components)
			fnt = v.Font
		case *node.Kern:
			wd += v.Kern
		}
	}
	if text.String() != "per-" {
		t.Fatalf("first line %q, want per-", text.String())
	}
	var want bag.ScaledPoint
	for _, a := range fnt.Shape("per", nil, nil) {
		want += a.Advance + a.Kernafter
	}
	if wd != want {
		t.Errorf("first line is %s wide, want %s, the width of per", wd, want)
	}
}

// The min-content width of a word is its widest part between two
// hyphenation points with the hyphen shaped after it. A Disc that replaces
// the glyph before a point does not count that glyph for the next part.
func TestMinContentWithReplacingDisc(t *testing.T) {
	fe, ff := lineModelDocument(t)
	l, err := GetLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	fe.Doc.DefaultLanguage = l
	word := "international"
	te := NewText()
	te.Items = []any{word}
	prep, err := fe.prepareParagraph(te, bag.MustSP("150pt"), FontSize(bag.MustSP("60pt")), Family(ff))
	if err != nil {
		t.Fatal(err)
	}
	var fnt *font.Font
	for n := prep.hlist; n != nil && fnt == nil; n = n.Next() {
		if g, ok := n.(*node.Glyph); ok {
			fnt = g.Font
		}
	}
	width := func(s string) (w bag.ScaledPoint) {
		for _, a := range fnt.Shape(s, nil, nil) {
			w += a.Advance + a.Kernafter
		}
		return w
	}
	var want bag.ScaledPoint
	var parts []string
	r := []rune(word)
	at := 0
	for _, step := range l.Hyphenate(word) {
		parts = append(parts, string(r[at:at+step])+"-")
		at += step
	}
	parts = append(parts, string(r[at:]))
	if len(parts) < 3 {
		t.Fatalf("parts %q, want at least three", parts)
	}
	for _, p := range parts {
		want = max(want, width(p))
	}
	if got, _ := measureParagraph(prep); got != want {
		t.Errorf("min-content %s, want %s, the widest of %q", got, want, parts)
	}
}
