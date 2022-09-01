package frontend

import (
	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/color"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/frontend/pdfdraw"
)

// TableCell represents a table cell
type TableCell struct {
	BorderTopWidth    bag.ScaledPoint
	BorderBottomWidth bag.ScaledPoint
	BorderLeftWidth   bag.ScaledPoint
	BorderRightWidth  bag.ScaledPoint
	BorderTopColor    *color.Color
	BorderBottomColor *color.Color
	BorderLeftColor   *color.Color
	BorderRightColor  *color.Color
	CalculatedWidth   bag.ScaledPoint
	CalculatedHeight  bag.ScaledPoint
	Contents          []*TypesettingElement
	row               *TableRow
}

func (cell *TableCell) minWidth() (bag.ScaledPoint, error) {
	te := cell.Contents[0]
	_, info, err := cell.row.table.doc.FormatParagraph(te, HSize(1*bag.Factor))
	if err != nil {
		return 0, err
	}
	minwd := bag.ScaledPoint(0)
	for _, inf := range info {
		if wd := inf.Width; wd > minwd {
			minwd = wd
		}
	}
	return minwd + cell.BorderLeftWidth + cell.BorderRightWidth, nil
}

func (cell *TableCell) maxWidth() (bag.ScaledPoint, error) {
	te := cell.Contents[0]
	_, info, err := cell.row.table.doc.FormatParagraph(te, HSize(bag.MaxSP))
	if err != nil {
		return 0, err
	}
	maxwd := bag.ScaledPoint(0)

	for _, inf := range info {
		if wd := inf.Width; wd > maxwd {
			maxwd = wd
		}
	}
	return maxwd + cell.BorderLeftWidth + cell.BorderRightWidth, nil
}

func (cell *TableCell) build() (*node.VList, error) {
	te := cell.Contents[0]
	paraWidth := cell.CalculatedWidth - cell.BorderLeftWidth - cell.BorderRightWidth
	para, _, err := cell.row.table.doc.FormatParagraph(te, HSize(paraWidth))
	if err != nil {
		return nil, err
	}
	cellHeight := cell.CalculatedHeight
	if cellHeight == 0 {
		cellHeight = para.Height + para.Depth + cell.BorderTopWidth + cell.BorderBottomWidth
	}

	para.Attributes = make(node.H)
	para.Attributes["origin"] = "cell mknodes"
	vl := para
	var head node.Node
	head = nil
	if cell.BorderTopWidth > 0 {
		if cell.BorderTopColor == nil {
			cell.BorderTopColor = cell.row.table.doc.GetColor("black")
		}
		toprule := node.NewRule()
		toprule.Width = cell.CalculatedWidth - cell.BorderLeftWidth - cell.BorderRightWidth
		toprule.Height = cell.BorderTopWidth
		toprule.Pre = pdfdraw.New().Save().ColorNonstroking(*cell.BorderTopColor).String()
		toprule.Post = pdfdraw.New().Restore().String()
		toprule.Attributes = node.H{"origin": "toprule"}
		head = toprule
	}
	glueHeight := (cellHeight - cell.BorderTopWidth - cell.BorderBottomWidth - vl.Height - vl.Depth) / 2
	topglue := node.NewGlue()
	topglue.Width = glueHeight

	head = node.InsertAfter(head, node.Tail(head), topglue)
	head = node.InsertAfter(head, topglue, vl)

	bottomglue := node.NewGlue()
	bottomglue.Width = glueHeight

	head = node.InsertAfter(head, vl, bottomglue)

	if cell.BorderBottomWidth > 0 {
		if cell.BorderBottomColor == nil {
			cell.BorderBottomColor = cell.row.table.doc.GetColor("black")
		}
		bottomrule := node.NewRule()
		bottomrule.Width = cell.CalculatedWidth - cell.BorderLeftWidth - cell.BorderRightWidth
		bottomrule.Height = cell.BorderBottomWidth
		bottomrule.Pre = pdfdraw.New().Save().ColorNonstroking(*cell.BorderBottomColor).String()
		bottomrule.Post = pdfdraw.New().Restore().String()
		head = node.InsertAfter(head, node.Tail(vl), bottomrule)
	}

	vl = node.Vpack(head)
	vl.Attributes = node.H{"origin": "vertical cell part"}
	head = nil
	if cell.BorderLeftWidth > 0 {
		if cell.BorderLeftColor == nil {
			cell.BorderLeftColor = cell.row.table.doc.GetColor("black")
		}
		leftrule := node.NewRule()
		leftrule.Height = cellHeight
		leftrule.Width = cell.BorderLeftWidth
		leftrule.Pre = pdfdraw.New().Save().ColorNonstroking(*cell.BorderLeftColor).String()
		leftrule.Post = pdfdraw.New().Restore().String()
		leftrule.Attributes = node.H{"origin": "leftrule"}
		head = leftrule
	}
	head = node.InsertAfter(head, node.Tail(head), vl)

	if cell.BorderRightWidth > 0 {
		if cell.BorderRightColor == nil {
			cell.BorderRightColor = cell.row.table.doc.GetColor("black")
		}
		rightrule := node.NewRule()
		rightrule.Height = cellHeight
		rightrule.Width = cell.BorderRightWidth
		rightrule.Pre = pdfdraw.New().Save().ColorNonstroking(*cell.BorderRightColor).String()
		rightrule.Post = pdfdraw.New().Restore().String()
		rightrule.Attributes = node.H{"origin": "leftrule"}
		head = node.InsertAfter(head, node.Tail(head), rightrule)

	}

	hl := node.Hpack(head)
	hl.Attributes = node.H{"origin": "hpack cell"}

	vl = node.Vpack(hl)
	return vl, nil
}

// TableRow represents a row in a table.
type TableRow struct {
	Cells            []*TableCell
	CalculatedHeight bag.ScaledPoint
	table            *Table
}

func (row *TableRow) getNumberOfColumns() int {
	return len(row.Cells)
}

func (row *TableRow) setHeight() error {
	maxht := bag.ScaledPoint(0)
	for i, col := range row.Cells {
		col.CalculatedWidth = row.table.columnWidths[i]
		vl, err := col.build()
		if err != nil {
			return err
		}
		if ht := vl.Height + vl.Depth; ht > maxht {
			maxht = ht
		}
	}
	for _, cell := range row.Cells {
		cell.CalculatedHeight = maxht
	}
	return nil
}

func (row *TableRow) calculateWidths() ([]bag.ScaledPoint, []bag.ScaledPoint, error) {
	colwidthsMin := make([]bag.ScaledPoint, len(row.Cells))
	colwidthsMax := make([]bag.ScaledPoint, len(row.Cells))
	for i, c := range row.Cells {
		c.row = row
		mw, err := c.minWidth()
		if err != nil {
			return nil, nil, err
		}
		colwidthsMin[i] = mw

		mw, err = c.maxWidth()
		if err != nil {
			return nil, nil, err
		}
		colwidthsMax[i] = mw
	}
	return colwidthsMin, colwidthsMax, nil
}

func (row *TableRow) build() (*node.HList, error) {
	var head node.Node
	var tail node.Node
	for i, c := range row.Cells {
		c.CalculatedWidth = row.table.columnWidths[i]
		vl, err := c.build()
		if err != nil {
			return nil, err
		}
		head = node.InsertAfter(head, tail, vl)
		tail = vl
	}

	hl := node.Hpack(head)
	hl.Attributes = make(node.H)
	hl.Attributes["origin"] = "table row"

	return hl, nil
}

type TableRows []*TableRow

func (tr TableRows) getNumberOfColumns() int {
	nCol := 0
	for _, row := range tr {
		return row.getNumberOfColumns()
	}
	return nCol
}

func (tr *TableRows) calculateHeights() {
	for _, row := range *tr {
		row.setHeight()
	}
}

// Table represents tabular material to be typeset.
type Table struct {
	MaxWidth     bag.ScaledPoint
	Stretch      bool
	Rows         TableRows
	doc          *Document
	columnWidths []bag.ScaledPoint
}

// TableOption controls the table typesetting.
type TableOption func(*Table)

// BuildTable creates one or more vertical lists to be placed into the PDF.
func (fe *Document) BuildTable(tbl *Table, opts ...TableOption) ([]*node.VList, error) {
	tbl.doc = fe
	var head, tail node.Node
	nCols := tbl.Rows.getNumberOfColumns()
	colmax := make([]bag.ScaledPoint, nCols)
	colmin := make([]bag.ScaledPoint, nCols)
	for _, r := range tbl.Rows {
		r.table = tbl
		rowmin, rowmax, err := r.calculateWidths()
		if err != nil {
			return nil, err
		}
		for i, max := range rowmax {
			if m := colmax[i]; max > m {
				colmax[i] = max
			}
		}
		for i, min := range rowmin {
			if m := colmin[i]; min > m {
				colmin[i] = min
			}
		}
	}
	sumCols := bag.ScaledPoint(0)

	for _, max := range colmax {
		sumCols += max
	}

	tbl.columnWidths = make([]bag.ScaledPoint, nCols)

	if tbl.MaxWidth < sumCols {
		// shrink
		r := tbl.MaxWidth.ToPT() / sumCols.ToPT()
		shrinkTbl := make([]float64, nCols)

		sumShrinkfactor := 0.0
		excess := bag.ScaledPoint(0)

		for i, colwd := range colmax {
			tbl.columnWidths[i] = bag.ScaledPointFromFloat(colwd.ToPT() * r)
			if a := tbl.columnWidths[i] - colmin[i]; a < 0 {
				excess += a
				tbl.columnWidths[i] = colmin[i]
			} else if a > 0 {
				shrinkTbl[i] = tbl.columnWidths[i].ToPT() / colmin[i].ToPT()
				sumShrinkfactor += shrinkTbl[i]
			}
		}
		for i := 0; i < nCols; i++ {
			if shrinkTbl[i] != 0 {
				tbl.columnWidths[i] += bag.ScaledPointFromFloat(shrinkTbl[i] / sumShrinkfactor * excess.ToPT())
			}
		}
	} else if tbl.MaxWidth == sumCols {
		// equal size
		for i, colwd := range colmax {
			tbl.columnWidths[i] = colwd
		}
	} else if tbl.MaxWidth > sumCols {
		// stretch
		r := tbl.MaxWidth.ToPT() / sumCols.ToPT()
		for i, colwd := range colmax {
			tbl.columnWidths[i] = bag.ScaledPointFromFloat(colwd.ToPT() * r)
		}
	}

	// now that the column widths are known, the row heights can be calculated
	tbl.Rows.calculateHeights()

	for _, r := range tbl.Rows {
		hl, err := r.build()
		if err != nil {
			return nil, err
		}
		head = node.InsertAfter(head, tail, hl)
		tail = hl
	}
	vl := node.Vpack(head)

	return []*node.VList{vl}, nil
}
