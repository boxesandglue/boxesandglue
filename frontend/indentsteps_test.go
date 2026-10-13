package frontend

import (
	"io"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// edgeGlues returns each line's leftskip and line-end glue widths.
func edgeGlues(vl *node.VList) (lefts, ends []bag.ScaledPoint) {
	for n := vl.List; n != nil; n = n.Next() {
		hl, ok := n.(*node.HList)
		if !ok {
			continue
		}
		for m := hl.List; m != nil; m = m.Next() {
			g, ok := m.(*node.Glue)
			if !ok {
				continue
			}
			switch o, _ := g.Attributes["origin"].(string); o {
			case "leftskip":
				lefts = append(lefts, g.Width)
			case "lineend":
				ends = append(ends, g.Width)
			}
		}
	}
	return lefts, ends
}

// The step settings reach the line breaker, and a text-indent gives way to
// steps on its own side as it gives way to a plain indent there, while
// steps on the other side leave it in place.
func TestIndentStepsSettings(t *testing.T) {
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
	steps := []node.IndentStep{{Inset: bag.MustSP("30pt"), Rows: 2}, {Inset: bag.MustSP("10pt"), Rows: 1}}
	indent := bag.MustSP("15pt")
	format := func(t *testing.T, key SettingType) (lefts, ends []bag.ScaledPoint) {
		t.Helper()
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Settings[SettingHAlign] = HAlignJustified
		te.Settings[SettingIndentStart] = indent
		te.Settings[key] = steps
		te.Items = append(te.Items, strings.Repeat("Lorem ipsum dolor sit amet. ", 12))
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("200pt"))
		if err != nil {
			t.Fatal(err)
		}
		lefts, ends = edgeGlues(vl)
		if len(lefts) < 5 {
			t.Fatalf("got %d lines, want at least 5", len(lefts))
		}
		return lefts, ends
	}
	ls := &node.LinebreakSettings{IndentSteps: steps}

	t.Run("left steps replace the text-indent", func(t *testing.T) {
		lefts, _ := format(t, SettingIndentLeftSteps)
		for row := range 4 {
			if got, want := lefts[row], ls.IndentForRow(row); got != want {
				t.Errorf("row %d: leftskip %s, want %s", row, got, want)
			}
		}
	})
	t.Run("right steps keep the text-indent", func(t *testing.T) {
		lefts, ends := format(t, SettingIndentRightSteps)
		for row := range 4 {
			wantLeft := bag.ScaledPoint(0)
			if row == 0 {
				wantLeft = indent
			}
			if lefts[row] != wantLeft {
				t.Errorf("row %d: leftskip %s, want %s", row, lefts[row], wantLeft)
			}
			if got, want := ends[row], ls.IndentForRow(row); got != want {
				t.Errorf("row %d: line-end glue %s, want %s", row, got, want)
			}
		}
	})
}
