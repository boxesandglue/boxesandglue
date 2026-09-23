package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// TestElementIDReachesTheBox checks that SettingElementID ends up as
// Attributes["id"] on the box each element builds — the paragraph's VList, a
// cell's VList and a row's HList — so a consumer can map laid-out geometry
// back to the markup it came from. SettingDest, which does name the element,
// is a PDF destination and must not be the only way to find it.
func TestElementIDReachesTheBox(t *testing.T) {
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

	t.Run("paragraph", func(t *testing.T) {
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Settings[SettingElementID] = "p1"
		te.Items = append(te.Items, "a paragraph")
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("200pt"))
		if err != nil {
			t.Fatal(err)
		}
		if got := vl.Attributes["id"]; got != "p1" {
			t.Errorf(`paragraph id = %v, want "p1"`, got)
		}
	})

	t.Run("no id, no attribute", func(t *testing.T) {
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Items = append(te.Items, "a paragraph")
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("200pt"))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := vl.Attributes["id"]; ok {
			t.Errorf("paragraph without SettingElementID has id %v", vl.Attributes["id"])
		}
	})

	text := func(id string, items ...any) *Text {
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		if id != "" {
			te.Settings[SettingElementID] = id
		}
		te.Items = items
		return te
	}
	// tableIDs builds a one-cell table and counts the ids on its boxes.
	tableIDs := func(t *testing.T, cell *TableCell, rowID string) map[string]int {
		t.Helper()
		row := &TableRow{ID: rowID, Cells: []*TableCell{cell}}
		vls, err := fe.BuildTable(&Table{MaxWidth: bag.MustSP("200pt"), Rows: TableRows{row}})
		if err != nil {
			t.Fatal(err)
		}
		ids := map[string]int{}
		for _, vl := range vls {
			collectIDs(vl, ids)
		}
		return ids
	}
	expect := func(t *testing.T, got map[string]int, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Errorf("ids = %v, want each of %v once", got, want)
		}
		for _, w := range want {
			if got[w] != 1 {
				t.Errorf("id %q is on %d boxes, want 1 (ids %v)", w, got[w], got)
			}
		}
	}

	t.Run("empty paragraph", func(t *testing.T) {
		vl, _, err := fe.FormatParagraph(text("p0"), bag.MustSP("200pt"))
		if err != nil {
			t.Fatal(err)
		}
		if got := vl.Attributes["id"]; got != "p0" {
			t.Errorf(`empty paragraph id = %v, want "p0"`, got)
		}
	})

	t.Run("cell and row", func(t *testing.T) {
		cell := &TableCell{ID: "c1", Contents: []any{fixedBox(bag.MustSP("20pt"), bag.MustSP("10pt"))}}
		expect(t, tableIDs(t, cell, "r1"), "c1", "r1")
	})

	t.Run("paragraph alone in a cell", func(t *testing.T) {
		cell := &TableCell{ID: "c1", Contents: []any{text("p1", "alone")}}
		expect(t, tableIDs(t, cell, ""), "c1", "p1")
	})

	t.Run("box element", func(t *testing.T) {
		box := text("box", "loose text", text("", "a child paragraph"))
		box.Settings[SettingBox] = true
		expect(t, tableIDs(t, &TableCell{Contents: []any{box}}, ""), "box")
	})
}

// collectIDs walks a node tree and counts every Attributes["id"] it meets.
func collectIDs(n node.Node, into map[string]int) {
	for ; n != nil; n = n.Next() {
		switch t := n.(type) {
		case *node.VList:
			if id, ok := t.Attributes["id"].(string); ok {
				into[id]++
			}
			collectIDs(t.List, into)
		case *node.HList:
			if id, ok := t.Attributes["id"].(string); ok {
				into[id]++
			}
			collectIDs(t.List, into)
		}
	}
}
