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
	var start, stop node.Node
	start = hl.List
	stop = node.Tail(hl.List)
	for e := hl.List; e != nil; e = e.Next() {
		stop = e
		if e.Type() == node.TypeGlue {
			if gl := e.(*node.Glue); gl.StretchOrder > 0 {
				break
			}
		}
	}

	wd, _, _ := node.Dimensions(start, stop, node.Horizontal)
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
	var start, stop node.Node
	start = hl.List
	stop = node.Tail(hl.List)
	for e := hl.List; e != nil; e = e.Next() {
		stop = e
		if e.Type() == node.TypeGlue {
			if gl := e.(*node.Glue); gl.StretchOrder > 0 {
				break
			}
		}
	}

	wd, _, _ := node.Dimensions(start, stop, node.Horizontal)
	return wd
}
