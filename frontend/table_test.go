package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// fixedBox returns cell contents with a fixed size, independent of the width
// the table offers. This keeps the test free of any font setup.
func fixedBox(wd, ht bag.ScaledPoint) FormatToVList {
	return func(bag.ScaledPoint) (*node.VList, error) {
		r := node.NewRule()
		r.Width = wd
		r.Height = ht
		return node.Vpack(r), nil
	}
}

// TestRowspanHeightDistribution guards against a copy and paste slip in
// TableRow.setHeight which used ExtraColspan when building the row span
// ranges (upstream PR #20). For a cell with colspan 1 and rowspan > 1 the
// range collapsed to a single row, so the spanned rows were never stretched
// and the cell's content overflowed its cell.
func TestRowspanHeightDistribution(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	tall := &TableCell{
		ExtraRowspan: 1,
		Contents:     []any{fixedBox(bag.MustSP("20pt"), bag.MustSP("100pt"))},
	}
	short1 := &TableCell{Contents: []any{fixedBox(bag.MustSP("30pt"), bag.MustSP("10pt"))}}
	short2 := &TableCell{Contents: []any{fixedBox(bag.MustSP("30pt"), bag.MustSP("10pt"))}}

	tbl := &Table{
		MaxWidth: bag.MustSP("200pt"),
		Rows: TableRows{
			&TableRow{Cells: []*TableCell{tall, short1}},
			&TableRow{Cells: []*TableCell{short2}},
		},
	}
	if _, err = fe.BuildTable(tbl); err != nil {
		t.Fatal(err)
	}

	// The 100pt cell must be distributed across BOTH spanned rows. The buggy
	// version collapsed the span to row 0, dumping the full height there and
	// leaving row 1 at its natural 10pt.
	if got := tbl.rowHeights[0] + tbl.rowHeights[1]; got < bag.MustSP("100pt") {
		t.Errorf("rows sum to %s, want at least 100pt", got)
	}
	if tbl.rowHeights[1] <= bag.MustSP("10pt") {
		t.Errorf("second spanned row got no share of the rowspan cell height: row heights %s and %s",
			tbl.rowHeights[0], tbl.rowHeights[1])
	}
}
