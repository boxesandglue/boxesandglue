package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

func splitRow(t *testing.T, breakInside bool) *node.HList {
	t.Helper()
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var boxes []any
	for range 10 {
		boxes = append(boxes, fixedBox(bag.MustSP("20pt"), bag.MustSP("10pt")))
	}
	rule := bag.MustSP("1pt")
	long := &TableCell{BorderTopWidth: rule, BorderBottomWidth: rule, BorderLeftWidth: rule, BorderRightWidth: rule, Contents: boxes}
	short := &TableCell{BorderTopWidth: rule, BorderBottomWidth: rule, BorderLeftWidth: rule, BorderRightWidth: rule,
		Contents: []any{fixedBox(bag.MustSP("20pt"), bag.MustSP("10pt"))}}
	tbl := &Table{
		ColSpec: []ColSpec{{ColumnWidth: &node.Glue{Width: bag.MustSP("50pt")}}, {ColumnWidth: &node.Glue{Width: bag.MustSP("50pt")}}},
		Rows:    TableRows{&TableRow{BreakInside: breakInside, Cells: []*TableCell{short, long}}},
	}
	vls, err := fe.BuildTable(tbl)
	if err != nil {
		t.Fatal(err)
	}
	return vls[0].List.(*node.HList)
}

func rules(n node.Node) int {
	count := 0
	for ; n != nil; n = n.Next() {
		switch v := n.(type) {
		case *node.Rule:
			if v.Width == bag.MustSP("20pt") {
				count++
			}
		case *node.HList:
			count += rules(v.List)
		case *node.VList:
			count += rules(v.List)
		}
	}
	return count
}

// A row that may break inside splits where it is asked to: the first part no
// taller than the room, every box of the cells in one part or the other, both
// cells as tall as each other in each part, and the rest splittable again.
func TestARowBreaksInside(t *testing.T) {
	row := splitRow(t, true)
	split, _ := row.Attributes["_split"].(RowSplitter)
	if split == nil {
		t.Fatal("a row that may break inside has no splitter")
	}
	avail := bag.MustSP("45pt")
	first, rest, ok := split(avail)
	if !ok {
		t.Fatal("45pt holds some of the row, but it did not split")
	}
	if first.Height+first.Depth > avail {
		t.Errorf("the first part is %s, taller than the %s it had", first.Height+first.Depth, avail)
	}
	if got := rules(first.List) + rules(rest.List); got != 11 {
		t.Errorf("the parts hold %d boxes, want the row's 11", got)
	}
	for _, part := range []*node.HList{first, rest} {
		var hts []bag.ScaledPoint
		for n := part.List; n != nil; n = n.Next() {
			if vl, ok := n.(*node.VList); ok {
				hts = append(hts, vl.Height+vl.Depth)
			}
		}
		if len(hts) != 2 || hts[0] != hts[1] {
			t.Errorf("the cells of a part are %v high, want two of the same height", hts)
		}
	}
	if _, ok := rest.Attributes["_split"].(RowSplitter); !ok {
		t.Error("the rest of the row cannot split again")
	}
	if _, _, ok := split(bag.MustSP("3pt")); ok {
		t.Error("3pt holds no box of the row, but it split")
	}
}

// A row that does not ask to break inside stays whole.
func TestARowStaysWhole(t *testing.T) {
	if _, ok := splitRow(t, false).Attributes["_split"]; ok {
		t.Error("a row that may not break inside has a splitter")
	}
}

// ruleHeights counts the horizontal rules 1pt high: the top and bottom lines
// of the cells.
func ruleHeights(n node.Node) int {
	count := 0
	for ; n != nil; n = n.Next() {
		switch v := n.(type) {
		case *node.Rule:
			if v.Height == bag.MustSP("1pt") && v.Width > bag.MustSP("1pt") {
				count++
			}
		case *node.HList:
			count += ruleHeights(v.List)
		case *node.VList:
			count += ruleHeights(v.List)
		}
	}
	return count
}

// Both parts of a split row are closed. Below a row whose bottom line is the
// line between them, the first part draws its line below and the rest draws
// the line above again, where the break left it open.
func TestASplitRowKeepsItsBorders(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	rule := bag.MustSP("1pt")
	cell := func(n int) *TableCell {
		var boxes []any
		for range n {
			boxes = append(boxes, fixedBox(bag.MustSP("20pt"), bag.MustSP("10pt")))
		}
		return &TableCell{BorderTopWidth: rule, BorderBottomWidth: rule, BorderLeftWidth: rule, BorderRightWidth: rule, Contents: boxes}
	}
	tbl := &Table{
		ColSpec: []ColSpec{{ColumnWidth: &node.Glue{Width: bag.MustSP("50pt")}}, {ColumnWidth: &node.Glue{Width: bag.MustSP("50pt")}}},
		Rows: TableRows{
			&TableRow{Cells: []*TableCell{cell(1), cell(1)}},
			&TableRow{BreakInside: true, Cells: []*TableCell{cell(1), cell(10)}},
		},
	}
	vls, err := fe.BuildTable(tbl)
	if err != nil {
		t.Fatal(err)
	}
	split, _ := vls[0].List.Next().(*node.HList).Attributes["_split"].(RowSplitter)
	first, rest, ok := split(bag.MustSP("45pt"))
	if !ok {
		t.Fatal("the row did not split")
	}
	if got := ruleHeights(first.List); got != 2 {
		t.Errorf("the first part draws %d lines, want the 2 below its cells", got)
	}
	if got := ruleHeights(rest.List); got != 4 {
		t.Errorf("the rest draws %d lines, want 4, above and below its two cells", got)
	}
}

// A row that holds a nested table breaks inside it: above the nested table,
// and inside a nested row that may break inside itself.
func TestANestedRowBreaksInside(t *testing.T) {
	for _, inner := range []bool{true, false} {
		fe, err := NewForWriter(io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		var boxes []any
		for range 10 {
			boxes = append(boxes, fixedBox(bag.MustSP("20pt"), bag.MustSP("10pt")))
		}
		nested := &Table{
			ColSpec: []ColSpec{{ColumnWidth: &node.Glue{Width: bag.MustSP("40pt")}}},
			Rows:    TableRows{&TableRow{BreakInside: inner, Cells: []*TableCell{{Contents: boxes}}}},
		}
		cell := &TableCell{Contents: []any{fixedBox(bag.MustSP("20pt"), bag.MustSP("10pt")), FormatToVList(func(bag.ScaledPoint) (*node.VList, error) {
			vls, err := fe.BuildTable(nested)
			if err != nil {
				return nil, err
			}
			return vls[0], nil
		})}}
		tbl := &Table{
			ColSpec: []ColSpec{{ColumnWidth: &node.Glue{Width: bag.MustSP("50pt")}}},
			Rows:    TableRows{&TableRow{BreakInside: true, Cells: []*TableCell{cell}}},
		}
		vls, err := fe.BuildTable(tbl)
		if err != nil {
			t.Fatal(err)
		}
		split, _ := vls[0].List.(*node.HList).Attributes["_split"].(RowSplitter)
		first, rest, ok := split(bag.MustSP("45pt"))
		if !ok {
			t.Fatal("the row did not split")
		}
		// Whole, the nested row goes to the rest; else some of it stays.
		want := 1
		if inner {
			want = 4
		}
		if a, b := rules(first.List), rules(rest.List); a != want || a+b != 11 {
			t.Errorf("nested row breaks inside %v: the parts hold %d and %d boxes, want %d and %d", inner, a, b, want, 11-want)
		}
	}
}

// A row a rowspan reaches into or out of stays whole, even if it asks to
// break inside: the spanning cell is drawn whole with its first row.
func TestARowspanRowStaysWhole(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	box := func() []any { return []any{fixedBox(bag.MustSP("20pt"), bag.MustSP("10pt"))} }
	tbl := &Table{
		ColSpec: []ColSpec{{ColumnWidth: &node.Glue{Width: bag.MustSP("50pt")}}, {ColumnWidth: &node.Glue{Width: bag.MustSP("50pt")}}},
		Rows: TableRows{
			&TableRow{BreakInside: true, Cells: []*TableCell{{ExtraRowspan: 1, Contents: box()}, {Contents: box()}}},
			&TableRow{BreakInside: true, Cells: []*TableCell{{Contents: box()}}},
			&TableRow{BreakInside: true, Cells: []*TableCell{{Contents: box()}, {Contents: box()}}},
		},
	}
	vls, err := fe.BuildTable(tbl)
	if err != nil {
		t.Fatal(err)
	}
	var i int
	for n := vls[0].List; n != nil; n = n.Next() {
		hl, ok := n.(*node.HList)
		if !ok {
			continue
		}
		_, got := hl.Attributes["_split"].(RowSplitter)
		if want := i == 2; got != want {
			t.Errorf("row %d splits: %v, want %v", i+1, got, want)
		}
		i++
	}
	if i != 3 {
		t.Fatalf("%d rows, want 3", i)
	}
}

// nestedTable is a table in a cell: its rows are 20pt, 20pt and 10pt with a
// 40pt cell spanning the first two, after a 10pt row if lead is set.
func nestedTable(lead bool) *Table {
	box := func(h string) any { return fixedBox(bag.MustSP("20pt"), bag.MustSP(h)) }
	var rows TableRows
	if lead {
		rows = append(rows, &TableRow{Cells: []*TableCell{{Contents: []any{box("10pt")}}, {Contents: []any{box("10pt")}}}})
	}
	rows = append(rows,
		&TableRow{Cells: []*TableCell{{ExtraRowspan: 1, Contents: []any{box("40pt")}}, {Contents: []any{box("20pt")}}}},
		&TableRow{Cells: []*TableCell{{Contents: []any{box("20pt")}}}},
		&TableRow{Cells: []*TableCell{{Contents: []any{box("10pt")}}, {Contents: []any{box("10pt")}}}},
	)
	return &Table{
		ColSpec: []ColSpec{{ColumnWidth: &node.Glue{Width: bag.MustSP("30pt")}}, {ColumnWidth: &node.Glue{Width: bag.MustSP("30pt")}}},
		Rows:    rows,
	}
}

// inRow is a one-row table whose row may break inside and whose only cell
// holds inner.
func inRow(fe *Document, width string, inner *Table) *Table {
	cell := &TableCell{Contents: []any{FormatToVList(func(bag.ScaledPoint) (*node.VList, error) {
		vls, err := fe.BuildTable(inner)
		if err != nil {
			return nil, err
		}
		return vls[0], nil
	})}}
	return &Table{
		ColSpec: []ColSpec{{ColumnWidth: &node.Glue{Width: bag.MustSP(width)}}},
		Rows:    TableRows{&TableRow{BreakInside: true, Cells: []*TableCell{cell}}},
	}
}

// nestedRows is the rows of the table depth levels of nesting below n,
// following the last row at each level.
func nestedRows(n node.Node, depth int) []*node.HList {
	for ; n != nil; n = n.Next() {
		var rows []*node.HList
		var list node.Node
		switch v := n.(type) {
		case *node.VList:
			for c := v.List; c != nil; c = c.Next() {
				if hl, ok := c.(*node.HList); ok && hl.Attributes["origin"] == "table row" {
					rows = append(rows, hl)
				}
			}
			list = v.List
		case *node.HList:
			list = v.List
		}
		if len(rows) > 0 {
			if depth == 0 {
				return rows
			}
			return nestedRows(rows[len(rows)-1].List, depth-1)
		}
		if r := nestedRows(list, depth); r != nil {
			return r
		}
	}
	return nil
}

func splitAt(t *testing.T, fe *Document, tbl *Table, avail string) (*node.HList, bool) {
	t.Helper()
	vls, err := fe.BuildTable(tbl)
	if err != nil {
		t.Fatal(err)
	}
	split, _ := vls[0].List.(*node.HList).Attributes["_split"].(RowSplitter)
	first, _, ok := split(bag.MustSP(avail))
	return first, ok
}

// A nested table does not break between the rows a rowspan joins: the break
// moves back before the group, and when nothing is left above it, nothing of
// the row fits.
func TestANestedRowspanGroupStaysTogether(t *testing.T) {
	for _, avail := range []string{"25pt", "30pt", "35pt"} {
		fe, err := NewForWriter(io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if first, ok := splitAt(t, fe, inRow(fe, "80pt", nestedTable(false)), avail); ok {
			rows := nestedRows(first.List, 0)
			t.Errorf("%s: the row split with %d nested rows in the first part, want no split", avail, len(rows))
		}
	}
	for _, c := range []struct {
		avail string
		want  int
	}{{"35pt", 1}, {"45pt", 1}, {"55pt", 3}} {
		avail, want := c.avail, c.want
		for depth := range 3 {
			fe, err := NewForWriter(io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			tbl := nestedTable(true)
			for range depth + 1 {
				tbl = inRow(fe, "80pt", tbl)
			}
			first, ok := splitAt(t, fe, tbl, avail)
			if !ok {
				t.Errorf("%s, depth %d: the row did not split", avail, depth)
				continue
			}
			rows := nestedRows(first.List, depth)
			if len(rows) != want {
				t.Errorf("%s, depth %d: %d nested rows in the first part, want %d", avail, depth, len(rows), want)
				continue
			}
			if keep, _ := rows[len(rows)-1].Attributes["_keepWithNext"].(bool); keep {
				t.Errorf("%s, depth %d: the first part ends inside a rowspan group", avail, depth)
			}
		}
	}
}
