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
	GlueSet   float64   // The ratio of the glue. Positive means stretching, negative shrinking.
	GlueOrder GlueOrder // The level of infinity
	// Shift moves the HList's outer baseline vertically when its parent
	// renders it. Positive shifts toward the top of the page (PDF +Y,
	// matching Glyph.YOffset). Symmetric with VList.Shift. Note: the box's
	// declared Height / Depth are NOT adjusted by Shift — callers that
	// need the outer container to "see" the shifted box must pre-compute
	// the right Height / Depth values, the way the math engine does in
	// frontend/math. Mirrors TeX's `box.shift_amount` (TeXbook S. 64).
	Shift bag.ScaledPoint
	// ShiftX shifts the HList's outer origin to the right when its
	// parent renders it. Symmetric with VList.ShiftX — every box-like
	// node can be moved, regardless of progressing direction (TeX's
	// \moveright applies to both \hbox and \vbox).
	ShiftX   bag.ScaledPoint
	VAlign   VerticalAlignment
	GlueSign uint8 // 0 = normal, 1 = stretching, 2 = shrinking
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
	n.ShiftX = h.ShiftX
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
