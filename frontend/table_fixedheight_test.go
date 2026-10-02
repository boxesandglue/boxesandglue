package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

func boxCell(ht string) *TableCell {
	return &TableCell{Contents: []any{fixedBox(bag.MustSP("30pt"), bag.MustSP(ht))}}
}

func buildRows(t *testing.T, rows ...*TableRow) (*Table, []*node.VList) {
	t.Helper()
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	tbl := &Table{MaxWidth: bag.MustSP("200pt"), Rows: rows}
	vls, err := fe.BuildTable(tbl)
	if err != nil {
		t.Fatal(err)
	}
	return tbl, vls
}

func wantHeights(t *testing.T, tbl *Table, want ...string) {
	t.Helper()
	for i, w := range want {
		if got := tbl.rowHeights[i]; got != bag.MustSP(w) {
			t.Errorf("row %d: height %s, want %s", i, got, w)
		}
	}
}

// firstCell is the outer vlist of the first cell in the first row.
func firstCell(t *testing.T, vls []*node.VList) *node.VList {
	t.Helper()
	for n := vls[0].List.(*node.HList).List; n != nil; n = n.Next() {
		if vl, ok := n.(*node.VList); ok && vl.Attributes["origin"] == "td" {
			return vl
		}
	}
	t.Fatal("no cell in the first row")
	return nil
}

// alignGlue returns the widths of the glue above and below a cell's contents.
func alignGlue(vl *node.VList) (top, bottom bag.ScaledPoint) {
	var walk func(n node.Node)
	walk = func(n node.Node) {
		for ; n != nil; n = n.Next() {
			switch v := n.(type) {
			case *node.Glue:
				switch v.Attributes["origin"] {
				case "padding top / vertical alignment":
					top = v.Width
				case "padding bottom / vertical alignment":
					bottom = v.Width
				}
			case *node.HList:
				walk(v.List)
			case *node.VList:
				walk(v.List)
			}
		}
	}
	walk(vl.List)
	return top, bottom
}

// A fixed height is the row height whether the content is shorter or taller.
func TestFixedRowHeight(t *testing.T) {
	tbl, vls := buildRows(t,
		&TableRow{Cells: []*TableCell{boxCell("10pt")}, FixedHeight: bag.MustSP("30pt")},
		&TableRow{Cells: []*TableCell{boxCell("50pt")}, FixedHeight: bag.MustSP("20pt")},
		&TableRow{Cells: []*TableCell{boxCell("10pt")}},
	)
	wantHeights(t, tbl, "30pt", "20pt", "10pt")
	if got := vls[0].List.(*node.HList).Height; got != bag.MustSP("30pt") {
		t.Errorf("the first row's hlist is %s high, want 30pt", got)
	}
}

// MinHeight on a fixed row and on its cells is ignored.
func TestFixedRowHeightIgnoresMinHeight(t *testing.T) {
	cell := boxCell("10pt")
	cell.MinHeight = bag.MustSP("40pt")
	tbl, _ := buildRows(t,
		&TableRow{Cells: []*TableCell{cell}, MinHeight: bag.MustSP("50pt"), FixedHeight: bag.MustSP("20pt")},
	)
	wantHeights(t, tbl, "20pt")
}

// Content taller than a fixed row overflows; the cell box with its
// borders (and the background drawn at its size) keeps the row height.
func TestFixedRowHeightOverflowKeepsTheCellBox(t *testing.T) {
	rule := bag.MustSP("1pt")
	cell := boxCell("50pt")
	cell.BorderTopWidth, cell.BorderBottomWidth, cell.BorderLeftWidth, cell.BorderRightWidth = rule, rule, rule, rule
	tbl, vls := buildRows(t, &TableRow{Cells: []*TableCell{cell}, FixedHeight: bag.MustSP("20pt")})
	wantHeights(t, tbl, "20pt")
	td := firstCell(t, vls)
	if got := td.Height + td.Depth; got != bag.MustSP("20pt") {
		t.Errorf("the cell is %s high, want the row's 20pt", got)
	}
	var side *node.Rule
	for n := td.List; n != nil; n = n.Next() {
		if hl, ok := n.(*node.HList); ok {
			side, _ = hl.List.(*node.Rule)
		}
	}
	if side == nil || side.Attributes["origin"] != "left rule" {
		t.Fatal("no left rule in the cell")
	}
	if side.Height != bag.MustSP("18pt") {
		t.Errorf("the left rule is %s high, want 18pt between the top and bottom rules", side.Height)
	}
}

// A fixed row stays whole even when it may break inside.
func TestFixedRowHeightDoesNotSplit(t *testing.T) {
	_, vls := buildRows(t, &TableRow{Cells: []*TableCell{boxCell("100pt")}, BreakInside: true, FixedHeight: bag.MustSP("40pt")})
	if _, ok := vls[0].List.(*node.HList).Attributes["_split"]; ok {
		t.Error("a row with a fixed height has a splitter")
	}
}

// A rowspan's missing height goes to the rows of the span that are not fixed.
func TestFixedRowHeightRowspan(t *testing.T) {
	tall := boxCell("100pt")
	tall.ExtraRowspan = 2
	tbl, _ := buildRows(t,
		&TableRow{Cells: []*TableCell{tall, boxCell("10pt")}, FixedHeight: bag.MustSP("10pt")},
		&TableRow{Cells: []*TableCell{boxCell("10pt")}},
		&TableRow{Cells: []*TableCell{boxCell("10pt")}, FixedHeight: bag.MustSP("10pt")},
	)
	wantHeights(t, tbl, "10pt", "80pt", "10pt")
}

// With every row of the span fixed, the spanning cell overflows.
func TestFixedRowHeightRowspanAllFixed(t *testing.T) {
	tall := boxCell("100pt")
	tall.ExtraRowspan = 1
	tbl, _ := buildRows(t,
		&TableRow{Cells: []*TableCell{tall, boxCell("10pt")}, FixedHeight: bag.MustSP("10pt")},
		&TableRow{Cells: []*TableCell{boxCell("10pt")}, FixedHeight: bag.MustSP("15pt")},
	)
	wantHeights(t, tbl, "10pt", "15pt")
	if got := tall.CalculatedHeight; got != bag.MustSP("25pt") {
		t.Errorf("the spanning cell is %s high, want the span's 25pt", got)
	}
}

// Overflowing content leaves the cell where its vertical alignment points:
// top-aligned at the bottom, middle at both edges, bottom-aligned at the top.
func TestFixedRowHeightOverflowFollowsVAlign(t *testing.T) {
	for _, tc := range []struct {
		valign      VerticalAlignment
		top, bottom string
	}{
		{VAlignTop, "0pt", "-20pt"},
		{VAlignMiddle, "-10pt", "-10pt"},
		{VAlignBottom, "-20pt", "0pt"},
	} {
		cell := boxCell("40pt")
		cell.VAlign = tc.valign
		_, vls := buildRows(t, &TableRow{Cells: []*TableCell{cell}, FixedHeight: bag.MustSP("20pt")})
		top, bottom := alignGlue(firstCell(t, vls))
		if top != bag.MustSP(tc.top) || bottom != bag.MustSP(tc.bottom) {
			t.Errorf("valign %d: glue above %s and below %s, want %s and %s", tc.valign, top, bottom, tc.top, tc.bottom)
		}
	}
}
