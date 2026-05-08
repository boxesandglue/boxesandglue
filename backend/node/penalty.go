package node

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// A Penalty is a valid horizontal or vertical break point. The higher the
// penalty the less likely the break occurs at this penalty. Anything below
// or equal -10000 is considered a forced break, anything higher than or
// equal to 10000 is considered a disallowed break.
type Penalty struct {
	basenode
	Penalty int             // Value
	Width   bag.ScaledPoint // Width of the penalty
}

func (p *Penalty) String() string {
	return String(p)
}

// Sizes returns the penalty's width as a width contribution; height and
// depth are zero.
func (p *Penalty) Sizes(Direction) (w, h, d bag.ScaledPoint) {
	return p.Width, 0, 0
}

// DebugAttributes returns the penalty value and width.
func (p *Penalty) DebugAttributes() ([]kv, H) {
	return []kv{
		{key: "id", value: p.ID},
		{key: "penalty", value: p.Penalty},
		{key: "width", value: p.Width},
	}, p.Attributes
}

// Copy creates a deep copy of the node.
func (p *Penalty) Copy() Node {
	n := NewPenalty()
	n.Penalty = p.Penalty
	n.Width = p.Width
	return n
}

// NewPenalty creates an initialized Penalty node
func NewPenalty() *Penalty {
	n := penaltySlab.alloc()
	n.ID = newID()
	n.typ = TypePenalty
	return n
}
