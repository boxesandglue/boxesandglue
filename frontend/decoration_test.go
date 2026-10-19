package frontend

import (
	"io"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// TestSeveralDecorationsOnOneRun covers boxesandglue#96: a run with an
// underline and a line-through drew only one of them. Both a set of lines in
// SettingTextDecorationLine and a list in SettingTextDecorations are drawn
// line by line, on every line the run is broken onto.
func TestSeveralDecorationsOnOneRun(t *testing.T) {
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
	red, blue := fe.GetColor("red"), fe.GetColor("blue")

	// rules formats a paragraph and returns the decoration rules of each line.
	rules := func(t *testing.T, te *Text, width bag.ScaledPoint) [][]string {
		t.Helper()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		vl, _, err := fe.FormatParagraph(te, width)
		if err != nil {
			t.Fatal(err)
		}
		var lines [][]string
		for e := vl.List; e != nil; e = e.Next() {
			hl, ok := e.(*node.HList)
			if !ok {
				continue
			}
			var line []string
			for n := hl.List; n != nil; n = n.Next() {
				if r, ok := n.(*node.Rule); ok && r.Hide && strings.Contains(r.Pre, " S") {
					line = append(line, r.Pre)
				}
			}
			lines = append(lines, line)
		}
		return lines
	}

	t.Run("set of lines", func(t *testing.T) {
		te := NewText()
		te.Settings[SettingTextDecorationLine] = TextDecorationUnderline | TextDecorationLineThrough
		te.Items = append(te.Items, "text")
		lines := rules(t, te, bag.MustSP("100pt"))
		if len(lines) != 1 || len(lines[0]) != 2 {
			t.Fatalf("got rules %q, want two on one line", lines)
		}
		if lines[0][0] == lines[0][1] {
			t.Errorf("both rules are drawn at the same place: %q", lines[0][0])
		}
	})

	t.Run("propagated and own decoration", func(t *testing.T) {
		// A struck span (red) inside an underlined one (blue): the span's own
		// line comes from SettingTextDecorationLine, the underline from the
		// list.
		inner := NewText()
		inner.Settings[SettingTextDecorations] = []TextDecoration{{Line: TextDecorationUnderline, Color: blue}}
		inner.Settings[SettingTextDecorationLine] = TextDecorationLineThrough
		inner.Settings[SettingTextDecorationColor] = red
		inner.Items = append(inner.Items, "struck")
		outer := NewText()
		outer.Settings[SettingTextDecorations] = []TextDecoration{{Line: TextDecorationUnderline, Color: blue}}
		outer.Items = append(outer.Items, "under ", inner, " under")
		te := NewText()
		te.Items = append(te.Items, outer)
		lines := rules(t, te, bag.MustSP("200pt"))
		if len(lines) != 1 {
			t.Fatalf("got %d lines, want one", len(lines))
		}
		var nRed, nBlue int
		for _, r := range lines[0] {
			switch {
			case strings.Contains(r, red.PDFStringStroking()):
				nRed++
			case strings.Contains(r, blue.PDFStringStroking()):
				nBlue++
			}
		}
		// One blue underline for each of the three runs, one red strike.
		if nBlue != 3 || nRed != 1 {
			t.Errorf("got %d blue and %d red rules, want 3 and 1: %q", nBlue, nRed, lines[0])
		}
	})

	t.Run("across line breaks", func(t *testing.T) {
		te := NewText()
		te.Settings[SettingTextDecorationLine] = TextDecorationUnderline | TextDecorationOverline
		te.Items = append(te.Items, "one two three four five six seven eight nine ten")
		lines := rules(t, te, bag.MustSP("60pt"))
		if len(lines) < 3 {
			t.Fatalf("expected at least three lines, got %d", len(lines))
		}
		for i, l := range lines {
			if len(l) != 2 {
				t.Errorf("line %d has %d decoration rules, want 2: %q", i+1, len(l), l)
			}
		}
	})
}
