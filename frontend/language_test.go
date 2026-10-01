package frontend

import (
	"bytes"
	"path/filepath"
	"testing"
	"unicode"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/document"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

func TestLoadLang(t *testing.T) {
	testdata := []struct {
		req  string
		want string
	}{
		{"en", "en"},
		{"en_US", "en_US"},
		{"de", "de"},
		{"de_DE", "de_DE"},
	}

	for _, tc := range testdata {
		l, err := GetLanguage(tc.req)
		if err != nil {
			t.Error(err)
		}
		if l == nil {
			t.Errorf("d.GetLanguage(%q) = nil, want lang", tc.req)
		}
		if l.Name != tc.want {
			t.Errorf("d.GetLanguage(%q) Name = %q, want %q", tc.req, l.Name, tc.want)
		}
	}
}

// TestNoOpLanguage exercises the CSS Text 3 §6 contract: tags without TeX
// hyphenation patterns must resolve without error and never produce break
// points. Arabic, Hebrew, CJK, and any custom non-listed tag fall in here.
func TestNoOpLanguage(t *testing.T) {
	testdata := []string{"ar", "he", "ja", "zh", "made-up-tag", "x-test"}
	for _, tag := range testdata {
		l, err := GetLanguage(tag)
		if err != nil {
			t.Fatalf("GetLanguage(%q) returned error: %v", tag, err)
		}
		if l == nil {
			t.Fatalf("GetLanguage(%q) returned nil", tag)
		}
		if l.Name != tag {
			t.Errorf("GetLanguage(%q).Name = %q, want %q", tag, l.Name, tag)
		}
		// A typical word in a Latin-pattern language would have several
		// breakpoints; with a no-op hyphenator we expect the empty slice.
		if pos := l.Hyphenate("supercalifragilistic"); len(pos) != 0 {
			t.Errorf("GetLanguage(%q).Hyphenate produced %d breakpoints, want 0",
				tag, len(pos))
		}
	}
}

// TestIsHyphenationSupported documents which tags qualify as "auto-hyphenable"
// without falling through to the no-op sentinel.
func TestIsHyphenationSupported(t *testing.T) {
	for _, tag := range []string{"en", "EN", "en_US", "de", "de_DE"} {
		if !IsHyphenationSupported(tag) {
			t.Errorf("IsHyphenationSupported(%q) = false, want true", tag)
		}
	}
	for _, tag := range []string{"ar", "he", "ja", "zh", "klingon", ""} {
		if IsHyphenationSupported(tag) {
			t.Errorf("IsHyphenationSupported(%q) = true, want false", tag)
		}
	}
}

func TestHyphenate(t *testing.T) {
	str := ` computer.`
	var head, cur node.Node
	for _, r := range str {
		if r == ' ' {
			n := node.NewGlue()
			n.Width = 5 * bag.Factor
			head = node.InsertAfter(head, cur, n)
			cur = n
		} else {
			n := node.NewGlyph()
			n.Hyphenate = unicode.IsLetter(r)
			n.Codepoint = 123
			n.Components = string(r)
			n.Width = 5 * bag.Factor
			head = node.InsertAfter(head, cur, n)
			cur = n
		}
	}

	var dummy bytes.Buffer
	doc := document.NewDocument(&dummy)
	l, err := doc.LoadPatternFile(filepath.Join("testdata", "hyph-en-us.pat.txt"), "dummylang")
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	Hyphenate(head, l)
	data := []node.Type{
		node.TypeGlue, node.TypeGlyph, node.TypeGlyph, node.TypeGlyph, node.TypeDisc, node.TypeGlyph, node.TypeGlyph, node.TypeGlyph, node.TypeGlyph, node.TypeGlyph,
	}

	for i, nt := range data {
		if want, got := nt, head.Type(); want != got {
			t.Errorf("head.Type() = %d, want %d (pos %d)", got, want, i)
		}
		head = head.Next()
	}
}

// A word that spans two runs with different scales gets a hyphen in the
// scale of the glyph before the break, the one the hyphen follows on the
// line. A kern between the two glyphs does not count as the glyph.
func TestHyphenTakesScaleOfGlyphBeforeBreak(t *testing.T) {
	for _, tc := range []struct{ first, second, want float64 }{
		{0.8, 0, 0.8},
		{0, 1.2, 0},
	} {
		var head, cur node.Node
		for i, r := range "computer" {
			n := node.NewGlyph()
			n.Hyphenate = true
			n.Codepoint = 123
			n.Components = string(r)
			n.Width = 5 * bag.Factor
			n.HorizontalScale = tc.first
			if i >= 3 {
				n.HorizontalScale = tc.second
			}
			head = node.InsertAfter(head, cur, n)
			cur = n
			if i == 2 {
				k := node.NewKern()
				k.Kern = bag.Factor
				head = node.InsertAfter(head, cur, k)
				cur = k
			}
		}
		var dummy bytes.Buffer
		doc := document.NewDocument(&dummy)
		l, err := doc.LoadPatternFile(filepath.Join("testdata", "hyph-en-us.pat.txt"), "dummylang")
		if err != nil {
			t.Fatal(err)
		}
		Hyphenate(head, l)
		var disc *node.Disc
		for n := head; n != nil; n = n.Next() {
			if d, ok := n.(*node.Disc); ok {
				disc = d
				break
			}
		}
		if disc == nil {
			t.Fatal("no hyphenation point in computer")
		}
		if g, ok := disc.Pre.(*node.Glyph); !ok || g.HorizontalScale != tc.want {
			t.Errorf("com|puter at %g|%g: hyphen %v, want scale %g", tc.first, tc.second, disc.Pre, tc.want)
		}
	}
}
