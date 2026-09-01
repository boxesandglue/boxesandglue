package frontend

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

func maxWidthWithoutStretch(vl *node.VList) bag.ScaledPoint {
	maxWd := bag.ScaledPoint(0)
	for e := vl.List; e != nil; e = e.Next() {
		switch t := e.(type) {
		case *node.VList:
			if wd := maxWidthWithoutStretch(t); wd > maxWd {
				maxWd = wd
			}
		case *node.HList:
			if wd := getMaxWidthHlistWithoutStretch(t); wd > maxWd {
				maxWd = wd
			}
		default:
			// fmt.Printf("t %#T\n", t)
		}
	}
	if maxWd == 0 {
		return vl.Width
	}
	return maxWd
}

func getMaxWidthHlistWithoutStretch(hl *node.HList) bag.ScaledPoint {
	return naturalWidthWithoutStretch(hl)
}

// naturalWidthWithoutStretch sums a line's horizontal extent, ignoring glue that
// carries infinite stretch. That glue is alignment filler: rightskip for ragged
// right, leftskip for text-align:right, and both for center.
//
// This scan used to stop at the first such glue, which is only correct when the
// filler trails the content. With text-align:right or :center the filler comes
// first, so the scan measured nothing and the line reported a natural width of
// zero — and a table cell containing a centred or right-aligned paragraph
// collapsed the whole table to min-content.
func naturalWidthWithoutStretch(hl *node.HList) bag.ScaledPoint {
	var wd bag.ScaledPoint
	for e := hl.List; e != nil; e = e.Next() {
		if e.Type() == node.TypeGlue {
			if gl := e.(*node.Glue); gl.StretchOrder > 0 {
				continue
			}
		}
		w, _, _ := node.Dimensions(e, e, node.Horizontal)
		wd += w
	}
	return wd
}

func minWidthWithoutStretch(vl *node.VList) bag.ScaledPoint {
	minWd := bag.ScaledPoint(0)
	for e := vl.List; e != nil; e = e.Next() {
		switch t := e.(type) {
		case *node.VList:
			return minWidthWithoutStretch(t)
		case *node.HList:
			if wd := getMinWidthHlistWithoutStretch(t); wd > minWd {
				minWd = wd
			}
		default:
			// fmt.Printf("t %#T\n", t)
		}
	}
	return minWd
}

func getMinWidthHlistWithoutStretch(hl *node.HList) bag.ScaledPoint {
	return naturalWidthWithoutStretch(hl)
}
