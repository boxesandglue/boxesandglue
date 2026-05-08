package node

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// A Rule is a node represents a colored rectangular area.
type Rule struct {
	basenode
	// PDF code that gets output before the rule.
	Pre string
	// PDF Code after drawing the rule.
	Post string
	// Hide makes the rule invisible, no colored area is drawn. Used to make Pre
	// and Post appear in the output with the given dimensions.
	Hide   bool
	Width  bag.ScaledPoint
	Height bag.ScaledPoint
	Depth  bag.ScaledPoint
}

func (r *Rule) String() string {
	return String(r)
}

// Sizes returns the rule's dimensions.
func (r *Rule) Sizes(Direction) (w, h, d bag.ScaledPoint) {
	return r.Width, r.Height, r.Depth
}

// DebugAttributes returns the rule's geometry.
func (r *Rule) DebugAttributes() ([]kv, H) {
	return []kv{
		{key: "id", value: r.ID},
		{key: "wd", value: r.Width},
		{key: "ht", value: r.Height},
		{key: "dp", value: r.Depth},
	}, r.Attributes
}

// Copy creates a deep copy of the node.
func (r *Rule) Copy() Node {
	n := NewRule()
	n.Width = r.Width
	n.Height = r.Height
	n.Depth = r.Depth
	return n
}

// NewRule creates an initialized Rule node
func NewRule() *Rule {
	n := ruleSlab.alloc()
	n.ID = newID()
	n.typ = TypeRule
	return n
}
