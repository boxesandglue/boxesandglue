package frontend

import (
	"fmt"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/color"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/frontend/pdfdraw"
)

// Table represents tabular material to be typeset.
type Table struct {
	MaxWidth     bag.ScaledPoint
	Stretch      bool
	FontFamily   *FontFamily
	Rows         TableRows
	ColSpec      []ColSpec
	doc          *Document
	columnWidths []bag.ScaledPoint
	rowHeights   []bag.ScaledPoint
	nCol         int
	nRow         int
	cellMatrix   matrix
}

// TableRow represents a row in a table.
type TableRow struct {
	Cells            []*TableCell
	CalculatedHeight bag.ScaledPoint
	VAlign           VerticalAlignment
	table            *Table
	row              int
}

// TableCell represents a table cell
type TableCell struct {
	BorderTopWidth              bag.ScaledPoint
	BorderBottomWidth           bag.ScaledPoint
	BorderLeftWidth             bag.ScaledPoint
	BorderRightWidth            bag.ScaledPoint
	BorderTopColor              *color.Color
	BorderBottomColor           *color.Color
	BorderLeftColor             *color.Color
	BorderRightColor            *color.Color
	CalculatedWidth             bag.ScaledPoint
	CalculatedHeight            bag.ScaledPoint
	HAlign                      HorizontalAlignment
	VAlign                      VerticalAlignment
	Contents                    []*Paragraph
	ExtraColspan                int
	ExtraRowspan                int
	calculatedBorderLeftWidth   bag.ScaledPoint
	calculatedBorderRightWidth  bag.ScaledPoint
	calculatedBorderTopWidth    bag.ScaledPoint
	calculatedBorderBottomWidth bag.ScaledPoint
	row                         *TableRow
	rowStart                    int // top left corner
	colStart                    int // top left corner
	nextCell                    []*TableCell
	nextRow                     []*TableCell
	vlist                       *node.VList
}

// ColSpec represents common traits for a column such as width.
type ColSpec struct {
	ColumnWidth *node.Glue
}

type cellptr struct {
	cell    *TableCell
	colspan int
	rowspan int
}

type span struct {
	start int
	end   int
	size  bag.ScaledPoint
}

func (cell *TableCell) String() string {
	return fmt.Sprintf("%d/%d", cell.colStart, cell.rowStart)
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
	paraWidth := cell.CalculatedWidth - cell.calculatedBorderLeftWidth - cell.calculatedBorderRightWidth
	var head node.Node
	var vl *node.VList
	for _, cc := range cell.Contents {
		para, _, err := cell.row.table.doc.FormatParagraph(cc, paraWidth, Family(cell.row.table.FontFamily), HorizontalAlign(cell.HAlign))
		if err != nil {
			return nil, err
		}
		head = node.InsertAfter(head, node.Tail(head), para)
	}
	vl = node.Vpack(head)
	cellHeight := cell.CalculatedHeight
	if cellHeight == 0 {
		cellHeight = vl.Height + vl.Depth + cell.calculatedBorderTopWidth + cell.calculatedBorderBottomWidth
	}

	vl.Attributes = make(node.H)
	vl.Attributes["origin"] = "cell mknodes"
	head = nil
	topBottomRuleWidth := cell.CalculatedWidth - cell.calculatedBorderLeftWidth - cell.calculatedBorderRightWidth
	if cell.calculatedBorderTopWidth > 0 {
		if cell.BorderTopColor == nil {
			cell.BorderTopColor = cell.row.table.doc.GetColor("black")
		}
		toprule := node.NewRule()
		toprule.Width = topBottomRuleWidth
		toprule.Height = cell.calculatedBorderTopWidth
		toprule.Pre = pdfdraw.New().Save().ColorNonstroking(*cell.BorderTopColor).String()
		toprule.Post = pdfdraw.New().Restore().String()
		toprule.Attributes = node.H{"origin": "toprule"}
		head = toprule
	}
	glueHeight := cellHeight - cell.calculatedBorderTopWidth - cell.calculatedBorderBottomWidth - vl.Height - vl.Depth

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
	} else {
		head = node.InsertAfter(head, vl, node.NewGlue())
	}

	if cell.calculatedBorderBottomWidth > 0 {
		if cell.BorderBottomColor == nil {
			cell.BorderBottomColor = cell.row.table.doc.GetColor("black")
		}
		bottomrule := node.NewRule()
		bottomrule.Width = topBottomRuleWidth
		bottomrule.Height = cell.calculatedBorderBottomWidth
		bottomrule.Pre = pdfdraw.New().Save().ColorNonstroking(*cell.BorderBottomColor).String()
		bottomrule.Post = pdfdraw.New().Restore().String()
		head = node.InsertAfter(head, node.Tail(vl), bottomrule)
	}

	vl = node.Vpack(head)
	vl.Attributes = node.H{"origin": "vertical cell part"}
	head = nil
	if cell.calculatedBorderLeftWidth > 0 {
		if cell.BorderLeftColor == nil {
			cell.BorderLeftColor = cell.row.table.doc.GetColor("black")
		}
		leftrule := node.NewRule()
		leftrule.Height = cellHeight
		leftrule.Width = cell.calculatedBorderLeftWidth
		leftrule.Pre = pdfdraw.New().Save().ColorNonstroking(*cell.BorderLeftColor).String()
		leftrule.Post = pdfdraw.New().Restore().String()
		leftrule.Attributes = node.H{"origin": "leftrule"}
		head = leftrule
	}
	head = node.InsertAfter(head, node.Tail(head), vl)

	if cell.calculatedBorderRightWidth > 0 {
		if cell.BorderRightColor == nil {
			cell.BorderRightColor = cell.row.table.doc.GetColor("black")
		}
		rightrule := node.NewRule()
		rightrule.Height = cellHeight
		rightrule.Width = cell.calculatedBorderRightWidth
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

func (row *TableRow) setHeight() ([]span, error) {
	maxht := bag.ScaledPoint(0)
	for _, cell := range row.Cells {
		cell.CalculatedWidth = 0
		for j := 0; j <= cell.ExtraColspan; j++ {
			cell.CalculatedWidth += row.table.columnWidths[cell.colStart+j]
		}
	}
	rowspans := []span{}
	for _, cell := range row.Cells {
		vl, err := cell.build()
		if err != nil {
			return nil, err
		}
		ht := vl.Height + vl.Depth
		if cell.ExtraRowspan == 0 {
			if ht > maxht {
				maxht = ht
			}
		} else {
			rowspans = append(rowspans, span{start: cell.rowStart, end: cell.rowStart + cell.ExtraColspan, size: ht})
		}
	}
	row.table.rowHeights[row.row] = maxht
	return rowspans, nil
}

func (row *TableRow) calculateWidths() ([]bag.ScaledPoint, []bag.ScaledPoint, []span, error) {
	colspans := []span{}
	colwidthsMin := make([]bag.ScaledPoint, len(row.Cells))
	colwidthsMax := make([]bag.ScaledPoint, len(row.Cells))
	for i, c := range row.Cells {
		c.row = row
		minwd, err := c.minWidth()
		if err != nil {
			return nil, nil, nil, err
		}
		maxwd, err := c.maxWidth()
		if err != nil {
			return nil, nil, nil, err
		}
		if c.ExtraColspan == 0 {
			colwidthsMin[i] = minwd
			colwidthsMax[i] = maxwd
		} else {
			colspans = append(colspans, span{start: c.colStart, end: c.colStart + c.ExtraColspan, size: maxwd})
		}
	}
	return colwidthsMin, colwidthsMax, colspans, nil
}

func (row *TableRow) build() (*node.HList, error) {
	var head node.Node
	var tail node.Node
	for x := 0; x < row.table.nCol; x++ {
		cellptr := row.table.cellMatrix[x][row.row]
		if cellptr.cell.rowStart == row.row {
			vl, err := cellptr.cell.build()
			if err != nil {
				return nil, err
			}
			head = node.InsertAfter(head, tail, vl)
			tail = vl
			x += cellptr.cell.ExtraColspan
		} else {
			g := node.NewGlue()
			g.Stretch = bag.Factor
			g.StretchOrder = 1
			hl := node.HpackTo(g, row.table.columnWidths[x])
			vl := node.Vpack(hl)
			head = node.InsertAfter(head, tail, vl)
			tail = vl
		}
	}
	hl := node.Hpack(head)
	hl.Attributes = make(node.H)
	hl.Attributes["origin"] = "table row"

	return hl, nil
}

// TableRows is a collection of table rows.
type TableRows []*TableRow

func (tr *TableRows) calculateHeights() error {
	rowspans := []span{}
	var tbl *Table
	for i, row := range *tr {
		if i == 0 {
			tbl = row.table
		}
		rs, err := row.setHeight()
		if err != nil {
			return err
		}
		rowspans = append(rowspans, rs...)
	}
	for _, rs := range rowspans {
		sumHT := bag.ScaledPoint(0)
		for r := rs.start; r <= rs.end; r++ {
			sumHT += tbl.rowHeights[r]
		}
		if rs.size > sumHT {
			stretch := (rs.size - sumHT) / bag.ScaledPoint(rs.end-rs.start+1)
			for r := rs.start; r <= rs.end; r++ {
				tbl.rowHeights[r] = tbl.rowHeights[r] + stretch
			}
		}
	}

	for _, row := range *tr {
		for _, cell := range row.Cells {
			cell.CalculatedHeight = 0
			for rs := 0; rs <= cell.ExtraRowspan; rs++ {
				cell.CalculatedHeight += row.table.rowHeights[cell.rowStart+rs]
			}
		}
	}
	return nil
}

func (cp cellptr) String() string {
	return cp.cell.String()
}

type matrix [][]cellptr

func (m matrix) String() string {
	var b strings.Builder
	nRows := len(m[0])
	nCols := len(m)
	for y := 0; y < nRows; y++ {
		fmt.Fprintln(&b, strings.Repeat("---------------------+-", nCols))
		for x := 0; x < nCols; x++ {
			nextCells := fmt.Sprint(m[x][y].cell.nextCell)
			fmt.Fprintf(&b, "%5s [-> %9s] | ", m[x][y].cell, nextCells)
		}
		fmt.Fprintln(&b)
		for x := 0; x < nCols; x++ {
			nextRows := fmt.Sprint(m[x][y].cell.nextRow)
			fmt.Fprintf(&b, "%5s [-> %9s] | ", "", nextRows)
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, strings.Repeat("---------------------+-", nCols))
	return b.String()
}

// analyzeTable builds some helper data structures for the table to calculate
// row span and col span and border widths (in case of border collapse).
func (tbl *Table) analyzeTable() {
	// calculate number of rows and columns
	tbl.nRow = len(tbl.Rows)
	for i, row := range tbl.Rows {
		row.table = tbl
		row.row = i
		if n := len(row.Cells); n > tbl.nCol {
			tbl.nCol = n
		}
	}

	// build n*m matrix where each entry points to the table cell which it displays.
	tbl.cellMatrix = make(matrix, tbl.nCol)

	for x := 0; x < tbl.nCol; x++ {
		tbl.cellMatrix[x] = make([]cellptr, tbl.nRow)
	}

	for y, row := range tbl.Rows {
		extraCol := 0
		for x := 0; x < len(row.Cells); x++ {
			cell := row.Cells[x]
			cell.row = row
			for tbl.cellMatrix[x+extraCol][y].cell != nil {
				extraCol++
			}
			cell.colStart = x + extraCol
			cell.rowStart = y
			cell.calculatedBorderLeftWidth = cell.BorderLeftWidth
			cell.calculatedBorderRightWidth = cell.BorderRightWidth
			cell.calculatedBorderBottomWidth = cell.BorderBottomWidth
			cell.calculatedBorderTopWidth = cell.BorderTopWidth
			for i := 0; i <= cell.ExtraColspan; i++ {
				for r := 0; r <= cell.ExtraRowspan; r++ {
					tbl.cellMatrix[x+i+extraCol][y+r] = cellptr{cell: cell, colspan: cell.ExtraColspan - i, rowspan: cell.ExtraRowspan - r}
				}
			}
			extraCol += cell.ExtraColspan
		}
	}

	// Use this matrix to get the next pointers for table cells.

	// Get next table cells
	for row := 0; row < tbl.nRow; row++ {
		for col := 0; col < tbl.nCol; col++ {
			cellp := tbl.cellMatrix[col][row]
			for r := 0; r < cellp.rowspan+1; r++ {
				cellp = tbl.cellMatrix[col][row+r]
				col += cellp.colspan
				if col < tbl.nCol-1 {
					nc := tbl.cellMatrix[col+1][row+r].cell
					// only append the next cell value if we have not appended it yet
					found := false
					for _, c := range cellp.cell.nextCell {
						if nc == c {
							found = true
							break
						}
					}
					if !found {
						cellp.cell.nextCell = append(cellp.cell.nextCell, nc)
					}
				}
			}
		}
	}

	// Get next table rows.
	for col := 0; col < tbl.nCol; col++ {
		for row := 0; row < tbl.nRow; row++ {
			cellp := tbl.cellMatrix[col][row]
			for c := 0; c < cellp.colspan+1; c++ {
				cellp = tbl.cellMatrix[col+c][row]
				row += cellp.rowspan
				if row < tbl.nRow-1 {
					nr := tbl.cellMatrix[col+c][row+1].cell
					// only append the next cell value if we have not appended it yet
					found := false
					for _, r := range cellp.cell.nextRow {
						if nr == r {
							found = true
							break
						}
					}
					if !found {
						cellp.cell.nextRow = append(cellp.cell.nextRow, nr)
					}
				}
			}
		}
	}
	// border collapse
	for _, row := range tbl.Rows {
		for _, cell := range row.Cells {
			maxBorderLeft := bag.ScaledPoint(0)
			for _, nc := range cell.nextCell {
				if nc.BorderLeftWidth > maxBorderLeft {
					maxBorderLeft = nc.BorderLeftWidth
				}
			}
			borderWidthWant := cell.BorderRightWidth
			for _, nc := range cell.nextCell {
				if nc.BorderLeftWidth > cell.BorderRightWidth {
					borderWidthWant = nc.BorderLeftWidth
				}
			}
			if cell.nextCell != nil {
				borderWidthWant /= 2
			}
			if borderWidthWant <= cell.calculatedBorderRightWidth {
				cell.calculatedBorderRightWidth = borderWidthWant
				for _, nc := range cell.nextCell {
					nc.calculatedBorderLeftWidth = borderWidthWant
				}
			} else {
				cell.calculatedBorderRightWidth = borderWidthWant
			}

			maxBorderTop := bag.ScaledPoint(0)
			for _, nr := range cell.nextRow {
				if nr.BorderTopWidth > maxBorderTop {
					maxBorderTop = nr.BorderTopWidth
				}
			}
			borderWidthWant = cell.BorderBottomWidth
			for _, nr := range cell.nextRow {
				if nr.BorderTopWidth > cell.BorderBottomWidth {
					borderWidthWant = nr.BorderTopWidth
				}
			}
			if borderWidthWant <= cell.calculatedBorderBottomWidth {
				for _, nr := range cell.nextRow {
					nr.calculatedBorderTopWidth = 0
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

	tbl.columnWidths = make([]bag.ScaledPoint, tbl.nCol)
	tbl.rowHeights = make([]bag.ScaledPoint, tbl.nRow)
	colspans := []span{}
	if tbl.ColSpec == nil || len(tbl.ColSpec) == 0 {
		colmax := make([]bag.ScaledPoint, tbl.nCol)
		colmin := make([]bag.ScaledPoint, tbl.nCol)
		for _, r := range tbl.Rows {
			rowmin, rowmax, colspan, err := r.calculateWidths()
			if err != nil {
				return nil, err
			}
			colspans = append(colspans, colspan...)
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

		// handle colspan
		for _, cs := range colspans {
			sumWd := bag.ScaledPoint(0)
			for i := cs.start; i <= cs.end; i++ {
				sumWd += colmax[i]
			}
			if cs.size > sumWd {
				stretch := (cs.size - sumWd) / bag.ScaledPoint(cs.end-cs.start+1)
				for r := cs.start; r <= cs.end; r++ {
					colmax[r] = colmax[r] + stretch
				}
			}
		}

		sumCols := bag.ScaledPoint(0)
		for _, max := range colmax {
			sumCols += max
		}

		if tbl.MaxWidth < sumCols {
			// shrink
			r := tbl.MaxWidth.ToPT() / sumCols.ToPT()
			shrinkTbl := make([]float64, tbl.nCol)

			sumShrinkFactor := 0.0
			excess := bag.ScaledPoint(0)

			for i, colwd := range colmax {
				tbl.columnWidths[i] = bag.ScaledPointFromFloat(colwd.ToPT() * r)
				if a := tbl.columnWidths[i] - colmin[i]; a < 0 {
					excess += a
					tbl.columnWidths[i] = colmin[i]
				} else if a > 0 {
					shrinkTbl[i] = tbl.columnWidths[i].ToPT() / colmin[i].ToPT()
					sumShrinkFactor += shrinkTbl[i]
				}
			}
			for i := 0; i < tbl.nCol; i++ {
				if shrinkTbl[i] != 0 {
					tbl.columnWidths[i] += bag.ScaledPointFromFloat(shrinkTbl[i] / sumShrinkFactor * excess.ToPT())
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
	} else {
		for _, colspec := range tbl.ColSpec {
			head = node.InsertAfter(head, tail, colspec.ColumnWidth)
			tail = colspec.ColumnWidth
		}
		hl := node.HpackTo(head, tbl.MaxWidth)
		i := 0
		for e := hl.List; e != nil; e = e.Next() {
			if g, ok := e.(*node.Glue); ok {
				tbl.columnWidths[i] = g.Width
			}
			i++
		}
	}
	head = nil
	tail = nil
	// now that the column widths are known, the row heights can be calculated
	err := tbl.Rows.calculateHeights()
	if err != nil {
		return nil, err
	}

	for i, row := range tbl.Rows {
		hl, err := row.build()
		if err != nil {
			return nil, err
		}
		// rows with rowspan might have a different height than requested by the
		// calculated row height, so we need to adjust
		hl.Height = tbl.rowHeights[i]
		head = node.InsertAfter(head, tail, hl)
		tail = hl
	}
	vl := node.Vpack(head)
	vl.Attributes = node.H{"origin": "table"}
	return []*node.VList{vl}, nil
}
