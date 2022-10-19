package frontend

import (
	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/color"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/frontend/pdfdraw"
)

// HorizontalAlignment is the horizontal alignment.
type HorizontalAlignment int

// VerticalAlignment is the vertical alignment.
type VerticalAlignment int

const (
	// HAlignDefault is an undefined alignment.
	HAlignDefault HorizontalAlignment = iota
	// HAlignLeft makes a table cell ragged right.
	HAlignLeft
	// HAlignRight makes a table cell ragged left.
	HAlignRight
	// HAlignCenter has ragged left and right alignment.
	HAlignCenter
)
const (
	// VAlignDefault is an undefined vertical alignment.
	VAlignDefault VerticalAlignment = iota
	// VAlignTop aligns the contents of the cell at the top.
	VAlignTop
	// VAlignMiddle aligns the contents of the cell in the vertical middle.
	VAlignMiddle
	// VAlignBottom aligns the contents of the cell at the bottom.
	VAlignBottom
)

// Table represents tabular material to be typeset.
type Table struct {
	MaxWidth     bag.ScaledPoint
	Stretch      bool
	FontFamily   *FontFamily
	Rows         TableRows
	doc          *Document
	columnWidths []bag.ScaledPoint
	nCol         int
	nRow         int
}

// TableRow represents a row in a table.
type TableRow struct {
	Cells            []*TableCell
	CalculatedHeight bag.ScaledPoint
	VAlign           VerticalAlignment
	table            *Table
}

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
	HAlign            HorizontalAlignment
	VAlign            VerticalAlignment
	Contents          []*Paragraph
	row               *TableRow
	vlist             *node.VList
}

func (cell *TableCell) minWidth() (bag.ScaledPoint, error) {
	minwd := bag.ScaledPoint(0)

	for _, cc := range cell.Contents {
		_, info, err := cell.row.table.doc.FormatParagraph(cc, 1*bag.Factor, Family(cell.row.table.FontFamily))
		if err != nil {
			return 0, err
		}
		for _, inf := range info {
			if wd := inf.Width; wd > minwd {
				minwd = wd
			}
		}
	}
	return minwd + cell.BorderLeftWidth + cell.BorderRightWidth, nil
}

func (cell *TableCell) maxWidth() (bag.ScaledPoint, error) {
	maxwd := bag.ScaledPoint(0)
	for _, cc := range cell.Contents {
		_, info, err := cell.row.table.doc.FormatParagraph(cc, bag.MaxSP, Family(cell.row.table.FontFamily))
		if err != nil {
			return 0, err
		}
		for _, inf := range info {
			if wd := inf.Width; wd > maxwd {
				maxwd = wd
			}
		}
	}

	return maxwd + cell.BorderLeftWidth + cell.BorderRightWidth, nil
}

func (cell *TableCell) build() (*node.VList, error) {
	paraWidth := cell.CalculatedWidth - cell.BorderLeftWidth - cell.BorderRightWidth
	var head node.Node
	var vl *node.VList
	for _, cc := range cell.Contents {
		para, _, err := cell.row.table.doc.FormatParagraph(cc, paraWidth, Family(cell.row.table.FontFamily))
		if err != nil {
			return nil, err
		}
		head = node.InsertAfter(head, node.Tail(head), para)
	}
	vl = node.Vpack(head)

	cellHeight := cell.CalculatedHeight
	if cellHeight == 0 {
		cellHeight = vl.Height + vl.Depth + cell.BorderTopWidth + cell.BorderBottomWidth
	}
	vl.Attributes = make(node.H)
	vl.Attributes["origin"] = "cell mknodes"
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
	glueHeight := cellHeight - cell.BorderTopWidth - cell.BorderBottomWidth - vl.Height - vl.Depth

	valign := cell.VAlign
	if valign == VAlignDefault {
		valign = cell.row.VAlign
	}

	if valign == VAlignDefault || valign == VAlignMiddle {
		glueHeight /= 2
	}

	if valign != VAlignTop {
		topglue := node.NewGlue()
		topglue.Width = glueHeight
		head = node.InsertAfter(head, node.Tail(head), topglue)
		head = node.InsertAfter(head, topglue, vl)
	} else {
		head = node.InsertAfter(head, head, vl)
	}

	if valign != VAlignBottom {
		bottomglue := node.NewGlue()
		bottomglue.Width = glueHeight
		head = node.InsertAfter(head, vl, bottomglue)
	}

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

func (row *TableRow) getNumberOfColumns() int {
	return len(row.Cells)
}

func (row *TableRow) setHeight() error {
	maxht := bag.ScaledPoint(0)
	for i, cell := range row.Cells {
		cell.CalculatedWidth = row.table.columnWidths[i]
		vl, err := cell.build()
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

// TableRows is a collection of table rows.
type TableRows []*TableRow

func (tr *TableRows) calculateHeights() {
	for _, row := range *tr {
		row.setHeight()
	}
}

func (tbl *Table) analyzeTable() {
	// calculate number of rows and columns
	tbl.nRow = len(tbl.Rows)
	for _, row := range tbl.Rows {
		if n := len(row.Cells); n > tbl.nCol {
			tbl.nCol = n
		}
	}

	// border collapse
	for j := 0; j < tbl.nRow; j++ {
		row := tbl.Rows[j]

		for i := 0; i < tbl.nCol; i++ {
			cell := row.Cells[i]

			if j < tbl.nRow-1 {
				nextRow := tbl.Rows[j+1].Cells[i]
				if b, t := cell.BorderBottomWidth, nextRow.BorderTopWidth; b > 0 && t > 0 {
					if b > t {
						nextRow.BorderTopWidth = 0
					} else {
						cell.BorderBottomWidth = 0
					}
				}
			}
			if i < tbl.nCol-1 {
				nextCell := row.Cells[i+1]
				if r, l := cell.BorderRightWidth, nextCell.BorderLeftWidth; r > 0 && l > 0 {
					if r > l {
						nextCell.BorderLeftWidth = 0
					} else {
						cell.BorderRightWidth = l
						nextCell.BorderLeftWidth = 0
					}
				}
			}
		}
	}
}

// BuildTable creates one or more vertical lists to be placed into the PDF.
func (fe *Document) BuildTable(tbl *Table) ([]*node.VList, error) {
	tbl.doc = fe
	var head, tail node.Node
	tbl.analyzeTable()
	colmax := make([]bag.ScaledPoint, tbl.nCol)
	colmin := make([]bag.ScaledPoint, tbl.nCol)

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

	tbl.columnWidths = make([]bag.ScaledPoint, tbl.nCol)

	if tbl.MaxWidth < sumCols {
		// shrink
		r := tbl.MaxWidth.ToPT() / sumCols.ToPT()
		shrinkTbl := make([]float64, tbl.nCol)

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
		for i := 0; i < tbl.nCol; i++ {
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
	vl.Attributes = node.H{"origin": "table"}
	return []*node.VList{vl}, nil
}
