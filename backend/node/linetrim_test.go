package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
)

// trimModel sets every line 9pt + 5pt and records start and end trims on
// it when they are not nil.
type trimModel struct{ start, end *bag.ScaledPoint }

func (m trimModel) LineBox(hl *HList, _ *LinebreakSettings) (bag.ScaledPoint, bag.ScaledPoint) {
	if m.start != nil {
		hl.Attributes[LineTrimStart] = *m.start
	}
	if m.end != nil {
		hl.Attributes[LineTrimEnd] = *m.end
	}
	return 9 * bag.Factor, 5 * bag.Factor
}

func (trimModel) Leading(*HList, *LinebreakSettings) *Glue { return nil }

// trimOf is a line's attribute key, -1 where it has none.
func trimOf(hl *HList, key string) bag.ScaledPoint {
	if t, ok := hl.Attributes[key].(bag.ScaledPoint); ok {
		return t
	}
	return -1
}

// Linebreak records on each line how far its height reaches above the
// text-over edge of LinebreakSettings.Font and its depth below the
// text-under edge: from the half-leading, from a line model's line box, or
// as the model records them.
func TestLineTrim(t *testing.T) {
	blockFont := &font.Font{ContentAscent: 6 * bag.Factor, ContentDescent: 2 * bag.Factor}
	pt := func(n int) bag.ScaledPoint { return bag.ScaledPoint(n) * bag.Factor }
	sp := func(n int) *bag.ScaledPoint { v := pt(n); return &v }
	for _, c := range []struct {
		name       string
		half       bool
		model      LineModel
		font       *font.Font
		start, end bag.ScaledPoint // -1: no attribute
	}{
		// The words are 7pt + 3pt and the lines 12pt: half the 2pt of
		// leading goes above and half below, so the line is 8pt + 4pt.
		{"half-leading", true, nil, blockFont, pt(2), pt(2)},
		{"a line model's box", false, trimModel{}, blockFont, pt(3), pt(3)},
		{"recorded by the model", false, trimModel{start: sp(1), end: sp(1)}, blockFont, pt(1), pt(1)},
		{"0 recorded by the model", false, trimModel{start: sp(0), end: sp(0)}, blockFont, -1, -1},
		{"recorded by the model, no font", false, trimModel{end: sp(1)}, nil, -1, pt(1)},
		{"no font", true, nil, nil, -1, -1},
		{"at the edges", true, nil, &font.Font{ContentAscent: 8 * bag.Factor, ContentDescent: 4 * bag.Factor}, -1, -1},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := lineModelSettings(c.half, false, c.model)
			s.Font = c.font
			vl, _ := Linebreak(threeLineParagraph(), s)
			lines := linesOf(vl)
			if len(lines) != 3 {
				t.Fatalf("%d lines, want 3", len(lines))
			}
			for i, hl := range lines {
				if g := trimOf(hl, LineTrimStart); g != c.start {
					t.Errorf("line %d: start trim %s, want %s", i+1, g, c.start)
				}
				if g := trimOf(hl, LineTrimEnd); g != c.end {
					t.Errorf("line %d: end trim %s, want %s", i+1, g, c.end)
				}
			}
		})
	}
}
