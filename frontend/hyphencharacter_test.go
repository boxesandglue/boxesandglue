package frontend

import (
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// A hyphen inside a ligature ends the characters before the point, and a
// hyphen of several characters stays in the list of the Disc.
func TestHyphenCharacterInsideLigature(t *testing.T) {
	l, err := GetLanguage("de")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		hyphen string
		want   string
	}{
		{"=", "Au{f=|l}[fl]age"},
		{"==", "Au{f==|l}[fl]age"},
		{"", "Au{f|l}[fl]age"},
	} {
		head, _ := shapedWord(t, "Auflage")
		for n := head; n != nil; n = n.Next() {
			if g, ok := n.(*node.Glyph); ok {
				g.Shaping = &font.Shaping{Hyphen: &tc.hyphen}
			}
		}
		Hyphenate(head, l)
		var b strings.Builder
		for n := head; n != nil; n = n.Next() {
			switch v := n.(type) {
			case *node.Glyph:
				b.WriteString(glyphString(v))
			case *node.Disc:
				b.WriteString("{" + glyphText(v.Pre) + "|" + glyphText(v.Post) + "}")
			}
		}
		if got := b.String(); got != tc.want {
			t.Errorf("hyphen %q: got %s, want %s", tc.hyphen, got, tc.want)
		}
	}
}

// The line breaker measures the whole text, so a long hyphen does not stick
// out of the line.
func TestHyphenCharacterIsMeasured(t *testing.T) {
	fe, err := NewForWriter(&strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("text")
	if err = ff.AddMember(&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"}, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	l, err := GetLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	hsize := bag.MustSP("80pt")
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	te.Settings[SettingLanguage] = l
	te.Settings[SettingHAlign] = HAlignLeft
	te.Settings[SettingHyphenCharacter] = "======"
	te.Items = append(te.Items, "supercalifragilisticexpialidocious supercalifragilisticexpialidocious")
	vl, _, err := fe.FormatParagraph(te, hsize)
	if err != nil {
		t.Fatal(err)
	}
	hyphens := 0
	for e := vl.List; e != nil; e = e.Next() {
		line, ok := e.(*node.HList)
		if !ok {
			continue
		}
		var wd bag.ScaledPoint
		text := ""
		for n := line.List; n != nil; n = n.Next() {
			if g, ok := n.(*node.Glyph); ok {
				wd += g.Width
				text += g.Components
			}
		}
		if strings.HasSuffix(text, "======") {
			hyphens++
		}
		if wd > hsize {
			t.Errorf("%q is %v wide, the line %v", text, wd, hsize)
		}
	}
	if hyphens == 0 {
		t.Error("no line ends in the hyphen")
	}
}
