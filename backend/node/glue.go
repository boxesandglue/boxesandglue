package node

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// GlueOrder represents the stretch and shrink priority.
type GlueOrder int

const (
	// StretchNormal means no stretching
	StretchNormal GlueOrder = iota
	// StretchFil is the first order infinity
	StretchFil
	// StretchFill is the second order infinity
	StretchFill
	// StretchFilll is the third order infinity
	StretchFilll
)

// GlueSubtype is set wherever the glue comes from.
type GlueSubtype int

const (
	// GlueDefault when no subtype is set
	GlueDefault GlueSubtype = iota
	// GlueLineStart is inserted left of the hlist during the line breaking
	GlueLineStart
	// GlueLineEnd is added at the end of each line in a paragraph so that copy
	// and paste works in PDF.
	GlueLineEnd
)

// LeaderType determines how leader patterns are aligned.
type LeaderType int

const (
	// LeaderAligned repeats the pattern on a global grid so that
	// patterns in different lines align vertically (TeX's \leaders).
	LeaderAligned LeaderType = iota
	// LeaderCentered centers copies; excess space is split at both ends (TeX's \cleaders).
	LeaderCentered
	// LeaderExpanded distributes excess space equally between copies (TeX's \xleaders).
	LeaderExpanded
)

// A Glue node has the value of a shrinking and stretching space
type Glue struct {
	basenode
	Leader       *HList // Pattern to repeat; nil means normal glue.
	Subtype      GlueSubtype
	Width        bag.ScaledPoint // The natural width of the glue.
	Stretch      bag.ScaledPoint // The stretchability of the glue, where width plus stretch = maximum width.
	Shrink       bag.ScaledPoint // The shrinkability of the glue, where width minus shrink = minimum width.
	StretchOrder GlueOrder       // The order of infinity of stretching.
	ShrinkOrder  GlueOrder       // The order of infinity of shrinking.
	LeaderType   LeaderType      // Alignment mode for the repeated pattern.
}

func (g *Glue) String() string {
	return String(g)
}

// Sizes returns the glue's natural width on the progression axis: in
// horizontal mode it is the width contribution, in vertical mode it
// becomes the height contribution. Depth is always zero.
func (g *Glue) Sizes(dir Direction) (w, h, d bag.ScaledPoint) {
	if dir == Horizontal {
		return g.Width, 0, 0
	}
	return 0, g.Width, 0
}

// DebugAttributes returns the glue's width, stretch / shrink (with their
// orders) and subtype.
func (g *Glue) DebugAttributes() ([]kv, H) {
	return []kv{
		{key: "id", value: g.ID},
		{key: "wd", value: g.Width},
		{key: "stretch", value: g.Stretch},
		{key: "stretchorder", value: g.StretchOrder},
		{key: "shrink", value: g.Shrink},
		{key: "shrinkorder", value: g.ShrinkOrder},
		{key: "subtype", value: g.Subtype},
	}, g.Attributes
}

// Copy creates a deep copy of the node.
func (g *Glue) Copy() Node {
	n := NewGlue()
	n.Width = g.Width
	n.Stretch = g.Stretch
	n.Shrink = g.Shrink
	n.StretchOrder = g.StretchOrder
	n.ShrinkOrder = g.ShrinkOrder
	return n
}

// NewGlue creates an initialized Glue node
func NewGlue() *Glue {
	n := glueSlab.alloc()
	n.ID = newID()
	n.typ = TypeGlue
	return n
}
