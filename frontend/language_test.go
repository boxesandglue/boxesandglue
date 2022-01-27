package frontend

import (
	"bytes"
	"path/filepath"
	"testing"
	"unicode"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/document"
	"github.com/speedata/boxesandglue/backend/node"
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
	l, err := doc.LoadPatternFile(filepath.Join("testdata/hyph-en-us.pat.txt"), "dummylang")
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
