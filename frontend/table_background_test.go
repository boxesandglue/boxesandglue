package frontend

import (
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// cellBackground returns the operators of the background of a cell, "" if it
// has none.
func cellBackground(t *testing.T, cell *TableCell) string {
	t.Helper()
	_, vls := buildRows(t, &TableRow{Cells: []*TableCell{cell}})
	for n := firstCell(t, vls).List; n != nil; n = n.Next() {
		if r, ok := n.(*node.Rule); ok && r.Attributes["origin"] == "cell background" {
			return r.Pre
		}
	}
	return ""
}

// BackgroundBelow is painted first, the lowest color at the bottom.
func TestCellBackgroundBelow(t *testing.T) {
	cell := boxCell("10pt")
	cell.BackgroundBelow = []*color.Color{{Space: color.ColorRGB, R: 1}, {Space: color.ColorRGB, G: 1}}
	cell.BackgroundColor = &color.Color{Space: color.ColorRGB, B: 1}
	pre := cellBackground(t, cell)
	red, green, blue := strings.Index(pre, "1 0 0 rg"), strings.Index(pre, "0 1 0 rg"), strings.Index(pre, "0 0 1 rg")
	if red < 0 || green < red || blue < green {
		t.Errorf("colors at %d, %d, %d in %q, want them in order", red, green, blue, pre)
	}
	if got := strings.Count(pre, " re f"); got != 3 {
		t.Errorf("%d rectangles, want 3", got)
	}
}

// The color "none" paints nothing. A rectangle without a color is black.
func TestCellBackgroundNone(t *testing.T) {
	cell := boxCell("10pt")
	cell.BackgroundColor = &color.Color{Space: color.ColorNone}
	if pre := cellBackground(t, cell); pre != "" {
		t.Errorf("background %q, want none", pre)
	}
}
