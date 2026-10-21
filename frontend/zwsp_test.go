package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// lineTexts returns the glyph text of each line of vl.
func lineTexts(vl *node.VList) []string {
	var out []string
	for n := vl.List; n != nil; n = n.Next() {
		hl, ok := n.(*node.HList)
		if !ok {
			continue
		}
		var s string
		for m := hl.List; m != nil; m = m.Next() {
			if g, ok := m.(*node.Glyph); ok {
				s += g.Components
			}
		}
		out = append(out, s)
	}
	return out
}

// TestZeroWidthSpace checks U+200B against CSS Text 3 §5.1: it adds no
// width and is a soft wrap opportunity, unless the text does not wrap.
func TestZeroWidthSpace(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("test")
	if err := ff.AddMember(
		&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"},
		FontWeight400, FontStyleNormal,
	); err != nil {
		t.Fatal(err)
	}
	format := func(t *testing.T, measure bag.ScaledPoint, ws WhiteSpace, items ...any) *node.VList {
		t.Helper()
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Settings[SettingWhiteSpace] = ws
		// Justified, a line without stretchable glue is infeasible and
		// the line breaker would rather hyphenate.
		te.Settings[SettingHAlign] = HAlignLeft
		te.Items = append(te.Items, items...)
		vl, _, err := fe.FormatParagraph(te, measure)
		if err != nil {
			t.Fatal(err)
		}
		return vl
	}
	wide := bag.MustSP("120mm")

	t.Run("no width on its own", func(t *testing.T) {
		want := runEnds(format(t, wide, WhiteSpaceNormal, "[left]", "[right]"))
		got := runEnds(format(t, wide, WhiteSpaceNormal, "[left]", "\u200B", "[right]"))
		if len(got) != 1 || len(want) != 1 || got[0] != want[0] {
			t.Errorf("line ends at %v, want %v", got, want)
		}
	})

	t.Run("a space after it stays", func(t *testing.T) {
		want := runEnds(format(t, wide, WhiteSpaceNormal, "[left] [right]"))
		got := runEnds(format(t, wide, WhiteSpaceNormal, "[left]\u200B [right]"))
		if len(got) != 1 || len(want) != 1 || got[0] != want[0] {
			t.Errorf("line ends at %v, want %v", got, want)
		}
	})

	first, second := "Abcdefghijklm", "nopqrstuvwxyz"
	measure := max(runEnds(format(t, wide, WhiteSpaceNormal, first))[0],
		runEnds(format(t, wide, WhiteSpaceNormal, second))[0]) + bag.MustSP("1pt")

	t.Run("breaks between letters", func(t *testing.T) {
		got := lineTexts(format(t, measure, WhiteSpaceNormal, first+"\u200B"+second))
		if len(got) != 2 || got[0] != first || got[1] != second {
			t.Errorf("lines %q, want [%q %q]", got, first, second)
		}
	})

	t.Run("nowrap does not break", func(t *testing.T) {
		// Pattern hyphenation is switched off for nowrap a layer up, in
		// htmlbag, so only the break at the zero width space is checked.
		got := lineTexts(format(t, measure, WhiteSpaceNowrap, first+"\u200B"+second))
		if len(got) > 0 && got[0] == first {
			t.Errorf("lines %q, want no break after %q", got, first)
		}
	})
}
