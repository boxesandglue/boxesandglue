package frontend

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// spaceNode is a glue or rule of a node list, the way a space is set.
type spaceNode struct {
	width, stretch, shrink bag.ScaledPoint
	rule                   bool
}

// near reports whether a and b differ by the rounding of a font unit at
// most.
func near(a, b bag.ScaledPoint) bool {
	return a-b <= 2 && b-a <= 2
}

// spaceNodes returns the glues and rules between the glyphs of text set in
// TeX Gyre Heros at 10pt, and how many infinite penalties there are.
func spaceNodes(t *testing.T, ws WhiteSpace, text string) ([]spaceNode, int) {
	t.Helper()
	fe, ff := lineModelDocument(t)
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	if ws != WhiteSpaceNormal {
		te.Settings[SettingWhiteSpace] = ws
	}
	te.Items = append(te.Items, text)
	head, _, err := fe.Mknodes(te)
	if err != nil {
		t.Fatal(err)
	}
	var out []spaceNode
	var penalties int
	for n := head; n != nil; n = n.Next() {
		switch v := n.(type) {
		case *node.Glue:
			out = append(out, spaceNode{width: v.Width, stretch: v.Stretch, shrink: v.Shrink})
		case *node.Rule:
			out = append(out, spaceNode{width: v.Width, rule: true})
		case *node.Penalty:
			if v.Penalty >= 10000 {
				penalties++
			}
		}
	}
	return out, penalties
}

// Unicode spaces other than U+0020 and U+00A0 are not word separators (CSS
// Text 3 §4.1.1): they keep their own width, which for a font without the
// glyph is the shaper's fallback width, and do not stretch, shrink or
// collapse with a space next to them.
func TestFixedWidthSpace(t *testing.T) {
	// TeX Gyre Heros has no U+2009 or U+2003. The shaper falls back to one
	// fifth of an em and to an em, at 10pt 2pt and 10pt.
	for _, tc := range []struct {
		name  string
		ws    WhiteSpace
		text  string
		width bag.ScaledPoint
		rule  bool
	}{
		{"thin space", WhiteSpaceNormal, "a b", bag.MustSP("2pt"), false},
		{"em space", WhiteSpaceNormal, "a b", bag.MustSP("10pt"), false},
		{"thin space, pre", WhiteSpacePre, "a b", bag.MustSP("2pt"), true},
	} {
		got, _ := spaceNodes(t, tc.ws, tc.text)
		if len(got) != 1 || !near(got[0].width, tc.width) || got[0].stretch != 0 || got[0].shrink != 0 || got[0].rule != tc.rule {
			t.Errorf("%s: spaces %+v, want one of width %s, rule %t", tc.name, got, tc.width, tc.rule)
		}
	}

	t.Run("does not collapse", func(t *testing.T) {
		got, _ := spaceNodes(t, WhiteSpaceNormal, "a   b")
		if len(got) != 3 || !near(got[1].width, bag.MustSP("2pt")) || got[1].stretch != 0 || got[0].stretch == 0 || got[2].stretch == 0 {
			t.Errorf("spaces %+v, want word space, thin space, word space", got)
		}
	})

	t.Run("no-break", func(t *testing.T) {
		for _, tc := range []struct {
			ws        WhiteSpace
			text      string
			penalties int
		}{
			{WhiteSpaceNormal, "a b", 0},
			{WhiteSpaceNormal, "a b", 1},
			{WhiteSpaceNormal, "a b", 1},
			{WhiteSpaceNowrap, "a b", 1},
		} {
			if _, p := spaceNodes(t, tc.ws, tc.text); p != tc.penalties {
				t.Errorf("%q, %v: %d infinite penalties, want %d", tc.text, tc.ws, p, tc.penalties)
			}
		}
	})
}

// A line breaks after a thin space, not after a narrow no-break space, and
// the thin space is dropped at the end of the line.
func TestFixedWidthSpaceBreaks(t *testing.T) {
	fe, ff := lineModelDocument(t)
	format := func(measure bag.ScaledPoint, text string) *node.VList {
		t.Helper()
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Settings[SettingHAlign] = HAlignLeft
		te.Items = append(te.Items, text)
		vl, _, err := fe.FormatParagraph(te, measure)
		if err != nil {
			t.Fatal(err)
		}
		return vl
	}
	first, second := "Abcdefghijklm", "nopqrstuvwxyz"
	wide := bag.MustSP("120mm")
	measure := max(runEnds(format(wide, first))[0], runEnds(format(wide, second))[0]) + bag.MustSP("1pt")

	vl := format(measure, first+" "+second)
	if got := lineTexts(vl); len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("thin space: lines %q, want [%q %q]", got, first, second)
	}
	for n := vl.List.(*node.HList).List; n != nil; n = n.Next() {
		if g, ok := n.(*node.Glue); ok && near(g.Width, bag.MustSP("2pt")) {
			t.Errorf("thin space kept at the end of the line")
		}
	}

	if got := lineTexts(format(measure, first+" "+second)); len(got) > 0 && got[0] == first {
		t.Errorf("narrow no-break space: lines %q, want no break after %q", got, first)
	}
}
