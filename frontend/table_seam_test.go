package frontend

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// gridTable builds an n×n table in the collapsing model, every cell ruled
// all round with the same width, after mod has changed its cells.
func gridTable(t *testing.T, n int, rule bag.ScaledPoint, mod func([][]*TableCell)) [][]*TableCell {
	t.Helper()
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	tbl := &Table{BorderModel: BorderModelCollapse}
	cells := make([][]*TableCell, n)
	for r := range n {
		row := &TableRow{}
		for range n {
			c := &TableCell{
				BorderLeftWidth: rule, BorderRightWidth: rule,
				BorderTopWidth: rule, BorderBottomWidth: rule,
				Contents: []any{fixedBox(bag.MustSP("20pt"), bag.MustSP("10pt"))},
			}
			row.Cells = append(row.Cells, c)
			cells[r] = append(cells[r], c)
		}
		tbl.Rows = append(tbl.Rows, row)
	}
	for range n {
		tbl.ColSpec = append(tbl.ColSpec, ColSpec{ColumnWidth: &node.Glue{Width: bag.MustSP("60pt")}})
	}
	if mod != nil {
		mod(cells)
	}
	if _, err := fe.BuildTable(tbl); err != nil {
		t.Fatal(err)
	}
	return cells
}

// leftRule is the rule a built cell draws on its left.
func leftRule(t *testing.T, c *TableCell) (*node.Rule, bag.ScaledPoint) {
	t.Helper()
	td, err := c.build()
	if err != nil {
		t.Fatal(err)
	}
	for n := td.List; n != nil; n = n.Next() {
		hl, ok := n.(*node.HList)
		if !ok {
			continue
		}
		for m := hl.List; m != nil; m = m.Next() {
			if r, ok := m.(*node.Rule); ok && r.Attributes["origin"] == "left rule" {
				return r, td.Width
			}
		}
	}
	t.Fatal("no left rule")
	return nil, 0
}

// rectOf is the x and width of the rectangle a rule's Pre fills, if any.
func rectOf(pre string) (x, wd string, ok bool) {
	f := strings.Fields(pre)
	for i, s := range f {
		if s == "re" && i >= 4 {
			return f[i-4], f[i-2], true
		}
	}
	return "", "", false
}

// In the collapsing model the border between two columns is one line
// centred on the grid line (CSS 2.1 §17.6.2). Each cell keeps its half of
// the width, but the cell after the boundary paints the whole line in one
// piece: two abutting halves show a light seam where they meet once a
// renderer antialiases their edges.
func TestCollapsedColumnBorderIsPaintedInOnePiece(t *testing.T) {
	rule := bag.MustSP("1.5pt")
	half := rule / 2
	for _, n := range []int{2, 3} {
		t.Run(fmt.Sprintf("%dx%d", n, n), func(t *testing.T) {
			cells := gridTable(t, n, rule, nil)
			for r := range n {
				for c := range n {
					cell := cells[r][c]
					lr, wd := leftRule(t, cell)
					if wd != bag.MustSP("60pt") {
						t.Errorf("cell %d,%d is %s wide, want 60pt", r, c, wd)
					}
					x, w, ok := rectOf(lr.Pre)
					if c == 0 {
						if lr.Width != rule || ok {
							t.Errorf("cell %d,%d at the table's edge: left rule %s wide, painted %q; want its own %s", r, c, lr.Width, lr.Pre, rule)
						}
						continue
					}
					if lr.Width != half {
						t.Errorf("cell %d,%d takes %s of the seam, want half, %s", r, c, lr.Width, half)
					}
					if !ok || !lr.Hide || x != (-(rule - half)).String() || w != rule.String() {
						t.Errorf("cell %d,%d paints its seam as %q, want one %s rectangle from %s", r, c, lr.Pre, rule, -(rule - half))
					}
				}
			}
		})
	}
}

// A seam between borders of different widths takes the wider, painted
// whole from the cell after it.
func TestCollapsedColumnBorderTakesTheWiderInOnePiece(t *testing.T) {
	wide := bag.MustSP("2pt")
	cells := gridTable(t, 2, bag.MustSP("0.5pt"), func(c [][]*TableCell) { c[0][1].BorderLeftWidth = wide })
	lr, _ := leftRule(t, cells[0][1])
	if x, w, ok := rectOf(lr.Pre); !ok || x != (-wide/2).String() || w != wide.String() || lr.Width != wide/2 {
		t.Errorf("a 2pt seam is %s of the cell, painted as %q", lr.Width, lr.Pre)
	}
}

// spanTable builds A | R over B |, R spanning both rows, every border 1pt,
// with A's right border in colour a and B's in colour b.
func spanTable(t *testing.T, a, b string) *TableCell {
	t.Helper()
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	rule := bag.MustSP("1pt")
	cell := func() *TableCell {
		return &TableCell{
			BorderLeftWidth: rule, BorderRightWidth: rule,
			BorderTopWidth: rule, BorderBottomWidth: rule,
			Contents: []any{fixedBox(bag.MustSP("20pt"), bag.MustSP("10pt"))},
		}
	}
	ca, cb, r := cell(), cell(), cell()
	ca.BorderRightColor = fe.GetColor(a)
	cb.BorderRightColor = fe.GetColor(b)
	r.ExtraRowspan = 1
	tbl := &Table{BorderModel: BorderModelCollapse}
	tbl.Rows = []*TableRow{{Cells: []*TableCell{ca, r}}, {Cells: []*TableCell{cb}}}
	for range 2 {
		tbl.ColSpec = append(tbl.ColSpec, ColSpec{ColumnWidth: &node.Glue{Width: bag.MustSP("60pt")}})
	}
	if _, err := fe.BuildTable(tbl); err != nil {
		t.Fatal(err)
	}
	return r
}

// A cell spanning rows paints its left line in one piece only when every
// seam on its left agrees; one colour painted over all of them would be
// wrong next to the others.
func TestCollapsedColumnBorderDisagreeingSeamsKeepTheirHalves(t *testing.T) {
	lr, _ := leftRule(t, spanTable(t, "red", "blue"))
	if _, _, ok := rectOf(lr.Pre); ok || lr.Hide {
		t.Errorf("seams in red and blue are painted in one piece as %q", lr.Pre)
	}
	if lr.Width != bag.MustSP("0.5pt") {
		t.Errorf("the spanning cell takes %s of the seam, want half, 0.5pt", lr.Width)
	}
}

func TestCollapsedColumnBorderAgreeingSeamsArePaintedInOnePiece(t *testing.T) {
	lr, _ := leftRule(t, spanTable(t, "red", "red"))
	if x, w, ok := rectOf(lr.Pre); !ok || !lr.Hide || x != bag.MustSP("-0.5pt").String() || w != bag.MustSP("1pt").String() {
		t.Errorf("two red seams are painted as %q, want one 1pt rectangle from -0.5pt", lr.Pre)
	}
}
