package node

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// A Kern is a small space between glyphs.
type Kern struct {
	basenode
	// The displacement in progression direction.
	Kern bag.ScaledPoint
}

func (k *Kern) String() string {
	return String(k)
}

// Sizes returns the kern displacement as a width contribution.
func (k *Kern) Sizes(Direction) (w, h, d bag.ScaledPoint) {
	return k.Kern, 0, 0
}

// DebugAttributes returns the kern displacement.
func (k *Kern) DebugAttributes() ([]kv, H) {
	return []kv{
		{key: "id", value: k.ID},
		{key: "kern", value: k.Kern},
	}, k.Attributes
}

// Copy creates a deep copy of the node.
func (k *Kern) Copy() Node {
	n := NewKern()
	n.Kern = k.Kern
	return n
}

// NewKern creates an initialized Kern node
func NewKern() *Kern {
	n := kernSlab.alloc()
	n.ID = newID()
	n.typ = TypeKern
	return n
}
