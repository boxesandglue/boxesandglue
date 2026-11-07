package frontend

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/svgreader"
)

// TestSVGTextLigatureHasSourceText: a ligature has no entry of its own in the
// cmap of the font, so ToUnicode needs the characters it was made from. The
// SVG is alone on the page, otherwise the same ligature in a paragraph hides
// the error.
func TestSVGTextLigatureHasSourceText(t *testing.T) {
	var buf bytes.Buffer
	fe, err := NewForWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	fe.Doc.CompressLevel = 0
	ff := fe.NewFontFamily("test")
	if err := ff.AddMember(
		&FontSource{Location: "../qa/fonts/upem/fonts/CrimsonPro-Regular.ttf"},
		FontWeight400, FontStyleNormal,
	); err != nil {
		t.Fatal(err)
	}
	svgDoc, err := svgreader.Parse(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="30" font-family="test"><text x="0" y="20" font-size="12">fi</text></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	rule := fe.Doc.CreateSVGNodeFromDocument(svgDoc, 0, 0, NewSVGTextRenderer(fe))

	page := fe.Doc.NewPage()
	page.OutputAt(bag.MustSP("10pt"), bag.MustSP("100pt"), node.Vpack(node.Hpack(rule)))
	page.Shipout()
	if err := fe.Finish(); err != nil {
		t.Fatal(err)
	}
	pdf := buf.String()
	if !strings.Contains(pdf, "<00660069>") {
		t.Errorf("ToUnicode has no entry for fi")
	}
	// The only glyph without text is .notdef, the first one of the subset.
	if got := strings.Count(strings.ToLower(pdf), "<fffd>"); got != 1 {
		t.Errorf("%d glyphs map to U+FFFD, want 1", got)
	}
}
