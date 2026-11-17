package frontend

import (
	"io"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// visualLine formats te in one line and reads the line from left to right:
// the text of each glyph, a space for each interword glue, "|" for the
// padding of an inline box and "[box]" for a box.
func visualLine(t *testing.T, fe *Document, te *Text) string {
	t.Helper()
	vl, _, err := fe.FormatParagraph(te, bag.MustSP("400pt"))
	if err != nil {
		t.Fatal(err)
	}
	var lines []*node.HList
	for n := vl.List; n != nil; n = n.Next() {
		if hl, ok := n.(*node.HList); ok {
			lines = append(lines, hl)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	var b strings.Builder
	for n := lines[0].List; n != nil; n = n.Next() {
		switch v := n.(type) {
		case *node.Glyph:
			b.WriteString(v.Components)
		case *node.Glue:
			// The fill glue at the end of the line has a width as well.
			if v.Width > 0 && v.StretchOrder == 0 {
				b.WriteString(" ")
			}
		case *node.Kern:
			if o, _ := v.Attributes["origin"].(string); strings.HasPrefix(o, "padding") {
				b.WriteString("|")
			}
		case *node.HList:
			b.WriteString("[box]")
		}
	}
	return b.String()
}

// TestBidiParagraphLevels sets paragraphs whose levels depend on text
// across item boundaries (boxesandglue#108) and on items that are not text.
// The font has no Hebrew glyphs; the glyphs carry their characters anyway.
func TestBidiParagraphLevels(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("test")
	if err := ff.AddMember(&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"}, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	paragraph := func(dir Direction, items ...any) *Text {
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Settings[SettingDirection] = dir
		te.Items = items
		return te
	}
	span := func(items ...any) *Text {
		te := NewText()
		te.Items = items
		return te
	}
	padded := func(s string) *Text {
		te := span(s)
		te.Settings[SettingPaddingLeft] = bag.MustSP("10pt")
		te.Settings[SettingPaddingRight] = bag.MustSP("10pt")
		return te
	}
	box := func() node.Node {
		hl := node.NewHList()
		hl.Width = bag.MustSP("20pt")
		return hl
	}
	testcases := []struct {
		name string
		te   *Text
		want string
	}{
		// The number has level 2 inside the RTL run, so the RTL run
		// with the number is reversed as a whole.
		{"number in RTL run", paragraph(DirectionLTR, "abc אבג 123 דהו xyz"), "abc והד 123 גבא xyz"},
		{"number after a span", paragraph(DirectionLTR, "abc ", span("אבג"), " 123 דהו xyz"), "abc והד 123 גבא xyz"},
		{"soft hyphen in the number", paragraph(DirectionLTR, "abc אבג 1­23 דהו xyz"), "abc והד 123 גבא xyz"},
		{"LTR with number in RTL", paragraph(DirectionRTL, "אבג abc 123 דהו"), "והד abc 123 גבא"},
		// A box and the padding of an inline box take the level of
		// the text around them.
		{"box in RTL", paragraph(DirectionRTL, "אבג ", box(), " דהו"), "והד [box] גבא"},
		{"padding in RTL", paragraph(DirectionRTL, "אבג ", padded("זח"), " דהו"), "והד |חז| גבא"},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if got := visualLine(t, fe, tc.te); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDetectParagraphDirectionFirstStrong checks rules P2 and P3: a number
// is no strong character, and the text after a newline counts when the
// text before it has none.
func TestDetectParagraphDirectionFirstStrong(t *testing.T) {
	for _, tc := range []struct {
		text string
		want Direction
	}{
		{"2026 שנה טובה", DirectionRTL},
		{"123 abc", DirectionLTR},
		{"123\nאבג", DirectionRTL},
		{"123", DirectionLTR},
		{"⁧abc⁩ אבג", DirectionRTL},
	} {
		te := NewText()
		te.Items = append(te.Items, tc.text)
		if got := detectParagraphDirection(te); got != tc.want {
			t.Errorf("%q: %v, want %v", tc.text, got, tc.want)
		}
	}
}
