package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// afterTabs returns, for each line of vl, where the text after each tab
// starts, measured from the left edge of the line.
func afterTabs(vl *node.VList) [][]bag.ScaledPoint {
	var out [][]bag.ScaledPoint
	for n := vl.List; n != nil; n = n.Next() {
		hl, ok := n.(*node.HList)
		if !ok {
			continue
		}
		var line []bag.ScaledPoint
		var x bag.ScaledPoint
		for m := hl.List; m != nil; m = m.Next() {
			w, _, _ := m.Sizes(node.Horizontal)
			x += w
			if g, ok := m.(*node.Glue); ok && g.Subtype == node.GlueTab {
				line = append(line, x)
			}
		}
		out = append(out, line)
	}
	return out
}

// TestTabStopsFormatParagraph sets label and value lines through
// FormatParagraph: the values line up at the stop in a paragraph with
// ordinary white-space handling, which without stops turns a tab into a
// space.
func TestTabStopsFormatParagraph(t *testing.T) {
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
	stop := bag.MustSP("40mm")
	format := func(t *testing.T, s string, stops ...TabStop) *node.VList {
		t.Helper()
		if stops == nil {
			stops = []TabStop{{Position: stop}}
		}
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Settings[SettingTabStops] = stops
		te.Items = append(te.Items, s)
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("120mm"))
		if err != nil {
			t.Fatal(err)
		}
		return vl
	}

	t.Run("values align", func(t *testing.T) {
		got := afterTabs(format(t, "Name\tAlice\nPostal address\tBob"))
		if len(got) != 2 {
			t.Fatalf("got %d lines, want 2", len(got))
		}
		for i, line := range got {
			if len(line) != 1 || line[0] != stop {
				t.Errorf("line %d: value starts at %v, want [%s]", i, line, stop)
			}
		}
	})

	t.Run("nowrap does not break at a tab", func(t *testing.T) {
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Settings[SettingTabStops] = []TabStop{{Position: bag.MustSP("5mm")}}
		te.Settings[SettingWhiteSpace] = WhiteSpaceNowrap
		te.Items = append(te.Items, "one\ttwo\tthree\tfour")
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("10mm"))
		if err != nil {
			t.Fatal(err)
		}
		if got := len(afterTabs(vl)); got != 1 {
			t.Errorf("got %d lines, want 1", got)
		}
	})

	t.Run("a leading tab is kept", func(t *testing.T) {
		got := afterTabs(format(t, "\tAlice"))
		if len(got) != 1 || len(got[0]) != 1 || got[0][0] != stop {
			t.Errorf("text after the tab starts at %v, want [[%s]]", got, stop)
		}
	})

	t.Run("leader", func(t *testing.T) {
		vl := format(t, "Name\tAlice", TabStop{Position: stop, Leader: "."})
		var tab *node.Glue
		node.Walk(vl, func(n node.Node) bool {
			if g, ok := n.(*node.Glue); ok && g.Subtype == node.GlueTab {
				tab = g
			}
			return true
		})
		if tab == nil || tab.Leader == nil {
			t.Fatal("no leader on the tab")
		}
		if g, ok := tab.Leader.List.(*node.Glyph); !ok || g.Components != "." {
			t.Errorf("leader pattern %v, want the glyph \".\"", tab.Leader.List)
		}
	})
}
