package frontend

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// RowSplitter breaks a table row so the first part is at most avail high. The
// rest is what is left of every cell; it carries a RowSplitter of its own in
// Attributes["_split"], so a row can run over as many pages as it needs. ok is
// false when not even a line of the row fits in avail.
type RowSplitter func(avail bag.ScaledPoint) (first, rest *node.HList, ok bool)

// rowPart is what is still to be set of a row that may break inside: each of
// its cells' contents from where the last break left them.
type rowPart struct {
	row      *TableRow
	contents []*node.VList
	// cont is set on a part after a break, whose top edge draws the line
	// above the row again.
	cont      bool
	minHeight bag.ScaledPoint
	spacingV  bag.ScaledPoint
}

// splitsInside reports whether row i may break inside: it asks to, and no
// rowspan reaches into or out of it.
func (tbl *Table) splitsInside(i int) bool {
	row := tbl.Rows[i]
	if !row.BreakInside {
		return false
	}
	for x := 0; x < tbl.nCol; x++ {
		c := tbl.cellMatrix[x][i].cell
		if c == nil || c.rowStart != i || c.ExtraRowspan > 0 {
			return false
		}
	}
	return true
}

// rowSplitter is the RowSplitter of row i as BuildTable built it.
func (tbl *Table) rowSplitter(i int, spacingV bag.ScaledPoint) RowSplitter {
	return func(avail bag.ScaledPoint) (*node.HList, *node.HList, bool) {
		row := tbl.Rows[i]
		p := &rowPart{row: row, minHeight: tbl.rowHeights[i], spacingV: spacingV}
		for _, cell := range row.Cells {
			vl, err := cell.buildContents(cell.paraWidth())
			if err != nil {
				return nil, nil, false
			}
			p.contents = append(p.contents, vl.Copy().(*node.VList))
		}
		return p.split(avail)
	}
}

func (cell *TableCell) paraWidth() bag.ScaledPoint {
	return cell.CalculatedWidth - cell.calculatedBorderLeftWidth - cell.calculatedBorderRightWidth - cell.PaddingLeft - cell.PaddingRight
}

// topEdge is the line a part draws above a cell: its own, or after a break the
// line the row above draws under it in the collapsing model.
func (p *rowPart) topEdge(cell *TableCell) (bag.ScaledPoint, *color.Color) {
	tbl := p.row.table
	if !p.cont || tbl.BorderModel == BorderModelSeparate || p.row.row == 0 {
		if p.cont && cell.BorderTopWidth > cell.calculatedBorderTopWidth {
			return cell.BorderTopWidth, cell.BorderTopColor
		}
		return cell.calculatedBorderTopWidth, cell.BorderTopColor
	}
	if up := tbl.cellMatrix[cell.colStart][p.row.row-1].cell; up != nil {
		return up.calculatedBorderBottomWidth, up.BorderBottomColor
	}
	return 0, nil
}

func (p *rowPart) frame(cell *TableCell) bag.ScaledPoint {
	top, _ := p.topEdge(cell)
	return top + cell.PaddingTop + cell.PaddingBottom + cell.calculatedBorderBottomWidth
}

func (p *rowPart) split(avail bag.ScaledPoint) (*node.HList, *node.HList, bool) {
	avail -= p.spacingV
	if p.row.row == 0 && !p.cont {
		avail -= p.spacingV
	}
	first := &rowPart{row: p.row, cont: p.cont, spacingV: p.spacingV}
	rest := &rowPart{row: p.row, cont: true, spacingV: p.spacingV}
	any, left := false, false
	for k, cell := range p.row.Cells {
		// splitVList relinks the list it is given, and p.contents are the
		// lists this part's cells show, so it splits a copy.
		a, b := splitVList(p.contents[k].Copy().(*node.VList), avail-p.frame(cell))
		if hasContent(a) {
			any = true
		}
		if hasContent(b) {
			left = true
		}
		first.contents = append(first.contents, orEmpty(a))
		rest.contents = append(rest.contents, orEmpty(b))
	}
	if !any || !left {
		return nil, nil, false
	}
	hl, err := first.build()
	if err != nil {
		return nil, nil, false
	}
	rest.minHeight = p.minHeight - hl.Height - hl.Depth
	rl, err := rest.build()
	if err != nil {
		return nil, nil, false
	}
	return hl, rl, true
}

// build packs the part as a row, every cell as tall as the tallest.
func (p *rowPart) build() (*node.HList, error) {
	tbl := p.row.table
	ht := p.minHeight
	for k, cell := range p.row.Cells {
		ht = max(ht, p.contents[k].Height+p.contents[k].Depth+p.frame(cell))
	}
	var spacing bag.ScaledPoint
	if tbl.BorderModel == BorderModelSeparate {
		spacing = tbl.BorderSpacingHorizontal
	}
	var head, tail node.Node
	add := func(n node.Node) {
		head = node.InsertAfter(head, tail, n)
		tail = n
	}
	kern := func() {
		if spacing > 0 {
			k := node.NewKern()
			k.Kern = spacing
			k.Attributes = node.H{"origin": "border spacing"}
			add(k)
		}
	}
	kern()
	for k, cell := range p.row.Cells {
		vl, err := p.buildCell(cell, p.contents[k], ht)
		if err != nil {
			return nil, err
		}
		add(vl)
		kern()
	}
	hl := node.Hpack(head)
	hl.Attributes = node.H{"origin": "table row"}
	if p.row.ID != "" {
		hl.Attributes["id"] = p.row.ID
	}
	hl.Height = ht
	hl.Depth = p.spacingV
	if p.row.row == 0 && !p.cont {
		hl.Height += p.spacingV
	}
	var sp RowSplitter = p.split
	hl.Attributes["_split"] = sp
	return hl, nil
}

// buildCell builds cell with contents as its content, height high, drawing the
// line above as topEdge says. build is reused as it is: the cell's own fields
// are what it reads, so they are set for the call and restored after.
//
// The restore is a shallow copy of the cell. It is only safe while build
// changes nothing through a pointer, slice or map in the cell (its colors,
// Contents, the row or the table): such a change would outlive the call.
func (p *rowPart) buildCell(cell *TableCell, contents *node.VList, height bag.ScaledPoint) (*node.VList, error) {
	saved := *cell
	defer func() { *cell = saved }()
	cell.contentCache = contents
	cell.contentCacheWidth = cell.paraWidth()
	cell.CalculatedHeight = height
	cell.calculatedBorderTopWidth, cell.BorderTopColor = p.topEdge(cell)
	return cell.build()
}

func orEmpty(vl *node.VList) *node.VList {
	if vl == nil {
		vl = node.NewVList()
	}
	return vl
}

// hasContent reports whether a list holds anything but discardable space. An
// item with no height, such as a paragraph's zero-size anchor rule, is not a
// line: a part holding only that would split a row with nothing above the break.
func hasContent(vl *node.VList) bool {
	if vl == nil {
		return false
	}
	for n := vl.List; n != nil; n = n.Next() {
		if !discardable(n) && vsize(n) > 0 {
			return true
		}
	}
	return false
}

func discardable(n node.Node) bool {
	switch n.(type) {
	case *node.Glue, *node.Kern, *node.Penalty:
		return true
	}
	return false
}

// splitVList breaks a vertical list so its first part is at most h high,
// between its items or inside one: a list stacked at its natural height (a
// paragraph, a nested table) or a row that may break inside, but never
// between the rows a rowspan joins. The space at the break is dropped, as TeX
// drops it. first is nil when nothing fits, rest when
// everything does.
func splitVList(vl *node.VList, h bag.ScaledPoint) (first, rest *node.VList) {
	var items []node.Node
	var natural bag.ScaledPoint
	for n := vl.List; n != nil; n = n.Next() {
		items = append(items, n)
		natural += vsize(n)
	}
	if natural <= h {
		return vl, nil
	}
	var used bag.ScaledPoint
	cut := len(items)
	var a, b node.Node
	for i, n := range items {
		s := vsize(n)
		if used+s <= h {
			used += s
			continue
		}
		cut = i
		switch t := n.(type) {
		case *node.VList:
			if stacked(t) {
				fa, fb := splitVList(t, h-used)
				if hasContent(fa) && fb != nil {
					a, b = fa, fb
				}
			}
		case *node.HList:
			if sp, _ := t.Attributes["_split"].(RowSplitter); sp != nil {
				if fa, fb, ok := sp(h - used); ok {
					a, b = fa, fb
				}
			}
		}
		break
	}
	if a == nil {
		cut = groupStart(items, cut)
	}
	head := slicesOf(items[:cut], a)
	tail := items[cut:]
	if b != nil {
		tail = append([]node.Node{b}, items[cut+1:]...)
	}
	for len(head) > 0 && discardable(head[len(head)-1]) {
		head = head[:len(head)-1]
	}
	for len(tail) > 0 && discardable(tail[0]) {
		tail = tail[1:]
	}
	if len(head) == 0 {
		return nil, vl
	}
	return piece(vl, head), piece(vl, tail)
}

// groupStart moves a break before items[cut] back before the first row of
// the rowspan group it would fall in, found by _keepWithNext.
func groupStart(items []node.Node, cut int) int {
	for i := cut - 1; i >= 0; i-- {
		if discardable(items[i]) {
			continue
		}
		hl, ok := items[i].(*node.HList)
		if !ok {
			break
		}
		if keep, _ := hl.Attributes["_keepWithNext"].(bool); !keep {
			break
		}
		cut = i
	}
	return cut
}

func slicesOf(items []node.Node, extra node.Node) []node.Node {
	out := append([]node.Node(nil), items...)
	if extra != nil {
		out = append(out, extra)
	}
	return out
}

// piece is a list of items packed like vl: its width, shift and attributes.
func piece(vl *node.VList, items []node.Node) *node.VList {
	var head, tail node.Node
	for _, n := range items {
		n.SetPrev(nil)
		n.SetNext(nil)
		head = node.InsertAfter(head, tail, n)
		tail = n
	}
	p := node.Vpack(head)
	p.Width = vl.Width
	p.Shift, p.ShiftX = vl.Shift, vl.ShiftX
	if vl.Attributes != nil {
		p.Attributes = node.H{}
		for k, v := range vl.Attributes {
			p.Attributes[k] = v
		}
	}
	return p
}

// stacked reports whether a list is its items at their natural height, so
// breaking it moves nothing: one packed to a set height is left whole.
func stacked(vl *node.VList) bool {
	var sum bag.ScaledPoint
	for n := vl.List; n != nil; n = n.Next() {
		sum += vsize(n)
	}
	d := sum - vl.Height - vl.Depth
	return d > -bag.ScaledPoint(1000) && d < bag.ScaledPoint(1000)
}

func vsize(n node.Node) bag.ScaledPoint {
	_, ht, dp := n.Sizes(node.Vertical)
	return ht + dp
}
