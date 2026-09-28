package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// A left aligned paragraph holding only a coloured space is one line in a
// measure narrower than the space, as in a narrow spacer column of a table.
// The colour's markers made the space breakable, and the paragraph broke
// into two empty lines.
func TestSpaceOnlyParagraphIsOneLine(t *testing.T) {
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
	span := NewText()
	span.Settings[SettingColor] = fe.GetColor("black")
	span.Items = append(span.Items, " ")
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	te.Settings[SettingHAlign] = HAlignLeft
	te.Items = append(te.Items, span)
	vl, _, err := fe.FormatParagraph(te, bag.MustSP("0.75pt"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for e := vl.List; e != nil; e = e.Next() {
		if _, ok := e.(*node.HList); ok {
			n++
		}
	}
	if n != 1 {
		t.Errorf("got %d lines, want 1", n)
	}
}
