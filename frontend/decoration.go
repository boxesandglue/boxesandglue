package frontend

import (
	"fmt"

	"github.com/speedata/boxesandglue/backend/node"
)

type styles struct {
	isUnderline bool
}

func postLinebreakHL(n node.Node, st *styles) {
	// var underlineStart, underlineStop node.Node
	for e := n; e != nil; e = e.Next() {
		if hl, ok := e.(*node.HList); ok {
			postLinebreakHL(hl.List, st)
		} else if vl, ok := e.(*node.VList); ok {
			postLinebreakHL(vl.List, st)
		} else if ss, ok := e.(*node.StartStop); ok {
			if val := node.GetAttribute(ss, "underline"); val == true {
				st.isUnderline = true
				// underlineStart = ss
			} else if val == false {
				st.isUnderline = false
				// underlineStop = ss
			} else {
				fmt.Printf("%T\n", val)
			}
		}
	}
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
