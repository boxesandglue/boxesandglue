package frontend

import (
	"fmt"

	"github.com/boxesandglue/boxesandglue/backend/node"
)

type callbackType int

const (
	// CallbackPostLinebreak gets called right after the line break algorithm
	// finishes.
	CallbackPostLinebreak callbackType = iota
)

// PostLinebreakCallbackFunc gets a vertical list and returns a vertical list
// that replaces the line break list. If nil is returned the list is discarded.
type PostLinebreakCallbackFunc func(*node.VList) *node.VList

// RegisterCallback adds the callback fn to the cb slice.
func (fe *Document) RegisterCallback(cb callbackType, fn any) error {
	var ok bool
	switch cb {
	case CallbackPostLinebreak:
		var c PostLinebreakCallbackFunc
		if c, ok = fn.(PostLinebreakCallbackFunc); !ok {
			return fmt.Errorf("incorrect callback type %T, want PostLinebreakCallbackFunc", fn)
		}
		fe.postLinebreakCallback = append(fe.postLinebreakCallback, c)
		return nil
	}
	return fmt.Errorf("unknown callback type %T", cb)
}
