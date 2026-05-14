package node

import (
	"maps"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// A HList is a container for a list which items are placed horizontally next to
// each other. The most convenient way to create a hlist is using node.HPack.
// The width, height, depth, badness and the glue settings are calculated when
// using node.HPack.
type HList struct {
	basenode
	List      Node // The list itself.
	Width     bag.ScaledPoint
	Height    bag.ScaledPoint
	Depth     bag.ScaledPoint
	Badness   int
	GlueSet   float64         // The ratio of the glue. Positive means stretching, negative shrinking.
	GlueOrder GlueOrder       // The level of infinity
	Shift     bag.ScaledPoint // The displacement perpendicular to the progressing direction. Not used.
	VAlign    VerticalAlignment
	GlueSign  uint8 // 0 = normal, 1 = stretching, 2 = shrinking
}

func (h *HList) String() string {
	return String(h)
}

// Sizes returns the HList's packaged dimensions.
func (h *HList) Sizes(Direction) (w, ht, dp bag.ScaledPoint) {
	return h.Width, h.Height, h.Depth
}

// DebugAttributes returns the HList's geometry plus the glue ratio.
func (h *HList) DebugAttributes() ([]kv, H) {
	return []kv{
		{key: "id", value: h.ID},
		{key: "wd", value: h.Width},
		{key: "ht", value: h.Height},
		{key: "dp", value: h.Depth},
		{key: "r", value: h.GlueSet},
	}, h.Attributes
}

// Copy creates a deep copy of the node.
func (h *HList) Copy() Node {
	n := NewHList()
	n.Width = h.Width
	n.Height = h.Height
	n.Depth = h.Depth
	n.GlueSet = h.GlueSet
	n.GlueSign = h.GlueSign
	n.Shift = h.Shift
	n.List = CopyList(h.List)
	if h.Attributes != nil {
		n.Attributes = maps.Clone(h.Attributes)
	}
	return n
}

// NewHList creates an initialized HList node
func NewHList() *HList {
	n := hlistSlab.alloc()
	n.ID = newID()
	n.typ = TypeHList
	return n
}
