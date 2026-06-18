package math

import (
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// walkAll recursively visits every node reachable from the given root,
// descending into HList and VList children. Used by the integration tests
// to assert presence/absence of Rule/Kern/Glyph nodes anywhere in the
// produced tree.
func walkAll(root node.Node, fn func(node.Node)) {
	for n := root; n != nil; n = n.Next() {
		fn(n)
		switch x := n.(type) {
		case *node.HList:
			walkAll(x.List, fn)
		case *node.VList:
			walkAll(x.List, fn)
		}
	}
}

func hasRule(hl *node.HList) bool {
	if hl == nil {
		return false
	}
	found := false
	walkAll(hl.List, func(n node.Node) {
		if _, ok := n.(*node.Rule); ok {
			found = true
		}
	})
	return found
}
