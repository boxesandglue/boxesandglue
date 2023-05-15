package frontend

import (
	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/frontend/pdfdraw"
)

type styles struct {
	isUnderline bool
	ulpos       bag.ScaledPoint
	linewidth   bag.ScaledPoint
}

func doUnderline(head, start, stop node.Node, st *styles) node.Node {
	pdfUL := pdfdraw.NewStandalone().LineWidth(st.linewidth).Moveto(0, st.ulpos).Lineto(node.Dimensions(start, stop, node.Horizontal), st.ulpos).Stroke().String()
	r := node.NewRule()
	r.Hide = true
	r.Pre = pdfUL
	head = node.InsertBefore(head, start, r)
	return head
}

func postLinebreakHL(n node.Node, st *styles) node.Node {
	underlineFromStart := st.isUnderline
	var underlineStart, underlineStop node.Node
	var head, tail node.Node
	head = n
	for e := n; e != nil; e = e.Next() {
		tail = e
		if hl, ok := e.(*node.HList); ok {
			hl.List = postLinebreakHL(hl.List, st)
		} else if vl, ok := e.(*node.VList); ok {
			hl.List = postLinebreakHL(vl.List, st)
		} else if ss, ok := e.(*node.StartStop); ok {
			if val, ok := node.GetAttribute(ss, "underline"); ok {
				if val.(bool) {
					st.isUnderline = true
					underlineStart = ss
					if ulpos, ok := node.GetAttribute(ss, "underlinepos"); ok {
						if ulpos != nil {
							st.ulpos = ulpos.(bag.ScaledPoint)
						}
					}
					if lw, ok := node.GetAttribute(ss, "underlinelw"); ok {
						if lw != nil {
							st.linewidth = lw.(bag.ScaledPoint)
						}
					}
				} else {
					if underlineFromStart {
						head = doUnderline(head, head, e, st)
					}
					st.isUnderline = false
					underlineStop = ss
				}
			}
		}
		if underlineStart != nil && underlineStop != nil {
			head = doUnderline(head, underlineStart, underlineStop, st)
			underlineStart = nil
			underlineStop = nil
		}
	}
	if st.isUnderline {
		if underlineStart == nil && underlineStop == nil {
			// whole line
			head = doUnderline(head, head, tail, st)
		}
		if underlineStart != nil {
			// up to the end
			head = doUnderline(head, underlineStart, tail, st)
		}
	}
	return head
}

func postLinebreak(vl *node.VList) *node.VList {
	var e node.Node
	for e = vl.List; e != nil; e = e.Next() {
		if hl, ok := e.(*node.HList); ok {
			postLinebreakHL(hl, &styles{})
		}
	}
	return vl
}
