package frontend

import (
	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/node"
)

func maxWidthWithoutStretch(vl *node.VList) bag.ScaledPoint {
	for e := vl.List; e != nil; e = e.Next() {
		switch t := e.(type) {
		case *node.VList:
			return maxWidthWithoutStretch(t)
		case *node.HList:
			return getMaxWidthHlistWithoutStretch(t)
		default:
			// fmt.Printf("t %#T\n", t)
		}
	}
	return vl.Width
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
