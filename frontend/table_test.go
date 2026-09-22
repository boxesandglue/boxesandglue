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

// TestStretchGivesSlackToAutoColumns checks that a cell's specified width
// survives the stretch pass. The slack between the content width and the table
// width used to be shared by ratio across every column, which widened declared
// widths along with the auto ones: a 50pt cell in a 200pt table came out at
// 100pt.
func TestStretchGivesSlackToAutoColumns(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	fixed := &TableCell{
		SpecifiedWidth: bag.MustSP("50pt"),
		Contents:       []any{fixedBox(bag.MustSP("10pt"), bag.MustSP("10pt"))},
	}
	auto := &TableCell{Contents: []any{fixedBox(bag.MustSP("50pt"), bag.MustSP("10pt"))}}

	tbl := &Table{
		MaxWidth: bag.MustSP("200pt"),
		Stretch:  true,
		Rows:     TableRows{&TableRow{Cells: []*TableCell{fixed, auto}}},
	}
	if _, err = fe.BuildTable(tbl); err != nil {
		t.Fatal(err)
	}

	if got, want := tbl.columnWidths[0], bag.MustSP("50pt"); got != want {
		t.Errorf("declared column = %s, want %s", got, want)
	}
	if got, want := tbl.columnWidths[1], bag.MustSP("150pt"); got != want {
		t.Errorf("auto column = %s, want %s: the slack should all land here", got, want)
	}
}

// TestCellStacksBlockContents checks that a cell holding more than one
// block-level item stacks them. They used to be HpackTo'd, which laid them out
// side by side: the cell stayed one item tall and everything after the first
// was pushed past the cell's width, where the next row drew over it. The
// content vanished with no warning and no overflow diagnostic.
func TestCellStacksBlockContents(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	const blockHeight = "30pt"
	cell := &TableCell{Contents: []any{
		fixedBox(bag.MustSP("20pt"), bag.MustSP(blockHeight)),
		fixedBox(bag.MustSP("20pt"), bag.MustSP(blockHeight)),
	}}

	tbl := &Table{
		MaxWidth: bag.MustSP("200pt"),
		Rows:     TableRows{&TableRow{Cells: []*TableCell{cell}}},
	}
	if _, err = fe.BuildTable(tbl); err != nil {
		t.Fatal(err)
	}

	want := 2 * bag.MustSP(blockHeight)
	if tbl.rowHeights[0] < want {
		t.Errorf("row height = %s, want at least %s: the second block was not stacked",
			tbl.rowHeights[0], want)
	}
}

// TestTableColspanRest checks that a cell with a huge colspan, the "rule
// across all columns" idiom colspan="9999", does not widen the table beyond
// the columns its other rows have.
func TestTableColspanRest(t *testing.T) {
	cell := func(colspan int) *TableCell {
		return &TableCell{ExtraColspan: colspan - 1}
	}
	tbl := &Table{Rows: TableRows{
		{Cells: []*TableCell{cell(1), cell(1), cell(1)}},
		{Cells: []*TableCell{cell(9999)}},
		{Cells: []*TableCell{cell(2), cell(1)}},
	}}
	tbl.analyzeTable()
	if tbl.nCol != 3 {
		t.Errorf("got %d columns, want 3", tbl.nCol)
	}
	if got := tbl.Rows[1].Cells[0].ExtraColspan; got != 2 {
		t.Errorf("rule cell spans %d extra columns, want 2", got)
	}
}

// TestRowMinHeight checks that MinHeight on a row and on a cell is a lower
// bound for the row height (CSS 2.1 §17.5.3): a short row grows to it, a
// row whose content is taller keeps its natural height.
func TestRowMinHeight(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	content := func() *TableCell {
		return &TableCell{Contents: []any{fixedBox(bag.MustSP("30pt"), bag.MustSP("10pt"))}}
	}
	tallCell := content()
	tallCell.MinHeight = bag.MustSP("40pt")
	tbl := &Table{
		MaxWidth: bag.MustSP("200pt"),
		Rows: TableRows{
			&TableRow{Cells: []*TableCell{content()}, MinHeight: bag.MustSP("50pt")},
			&TableRow{Cells: []*TableCell{tallCell}},
			&TableRow{Cells: []*TableCell{content()}, MinHeight: bag.MustSP("5pt")},
			&TableRow{Cells: []*TableCell{content()}},
		},
	}
	if _, err = fe.BuildTable(tbl); err != nil {
		t.Fatal(err)
	}
	want := []bag.ScaledPoint{bag.MustSP("50pt"), bag.MustSP("40pt"), bag.MustSP("10pt"), bag.MustSP("10pt")}
	for i, w := range want {
		if got := tbl.rowHeights[i]; got != w {
			t.Errorf("row %d: height %s, want %s", i, got, w)
		}
	}
}

// TestCellKeepsSoleBlockAttributes: a cell whose contents are a single VList
// reuses that VList, so labelling it must not discard what it already carries,
// such as the alt text htmlbag puts on an image that is alone in its cell.
func TestCellKeepsSoleBlockAttributes(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	var image *node.VList
	format := func(bag.ScaledPoint) (*node.VList, error) {
		r := node.NewRule()
		r.Width = bag.MustSP("20pt")
		r.Height = bag.MustSP("10pt")
		image = node.Vpack(r)
		image.Attributes = node.H{"alt": "company logo"}
		return image, nil
	}
	cell := &TableCell{Contents: []any{FormatToVList(format)}}
	tbl := &Table{
		MaxWidth: bag.MustSP("200pt"),
		Rows:     TableRows{&TableRow{Cells: []*TableCell{cell}}},
	}
	if _, err = fe.BuildTable(tbl); err != nil {
		t.Fatal(err)
	}

	if got := image.Attributes["alt"]; got != "company logo" {
		t.Errorf("alt = %v, want it kept (attributes %v)", got, image.Attributes)
	}
	if got := image.Attributes["origin"]; got != "cell contents" {
		t.Errorf(`origin = %v, want "cell contents"`, got)
	}
}
