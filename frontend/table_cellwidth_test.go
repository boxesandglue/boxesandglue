package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// TestCellBorderSitsAtTheCellEdge: content narrower than the cell's content
// area, such as a fixed-width box or a paragraph htmlbag sets with a side
// margin, must not pull the right border in from the cell's edge.
func TestCellBorderSitsAtTheCellEdge(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	colWidth := bag.MustSP("100pt")
	rule := bag.MustSP("1pt")
	cell := &TableCell{
		BorderRightWidth: rule,
		BorderLeftWidth:  rule,
		Contents:         []any{fixedBox(bag.MustSP("20pt"), bag.MustSP("10pt"))},
	}
	tbl := &Table{
		BorderModel: BorderModelSeparate,
		ColSpec:     []ColSpec{{ColumnWidth: &node.Glue{Width: colWidth}}},
		Rows:        TableRows{&TableRow{Cells: []*TableCell{cell}}},
	}
	if _, err = fe.BuildTable(tbl); err != nil {
		t.Fatal(err)
	}
	td, err := cell.build()
	if err != nil {
		t.Fatal(err)
	}
	var row *node.HList
	for n := td.List; n != nil; n = n.Next() {
		if hl, ok := n.(*node.HList); ok {
			row = hl
		}
	}
	if row == nil {
		t.Fatal("no horizontal cell part")
	}
	var x, ruleAt bag.ScaledPoint = 0, -1
	for n := row.List; n != nil; n = n.Next() {
		switch v := n.(type) {
		case *node.Rule:
			if v.Attributes["origin"] == "right rule" {
				ruleAt = x
			}
			x += v.Width
		case *node.Glue:
			x += v.Width
		case *node.VList:
			x += v.Width
		}
	}
	if want := colWidth - rule; ruleAt != want {
		t.Errorf("right rule at %s, want %s: the cell's right edge less the rule", ruleAt, want)
	}
	if td.Width != colWidth {
		t.Errorf("cell width %s, want %s", td.Width, colWidth)
	}
}
