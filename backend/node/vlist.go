package node

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// A VList is a vertical list.
type VList struct {
	basenode
	List   Node
	Width  bag.ScaledPoint
	Height bag.ScaledPoint
	Depth  bag.ScaledPoint
	// ShiftX shifts the VList's outer origin to the right when its
	// parent renders it. Symmetric with HList.ShiftX — every box-like
	// node can be moved, regardless of progressing direction.
	ShiftX bag.ScaledPoint
	// Shift moves the VList's outer reference point vertically when its
	// parent renders it. Positive shifts toward the top of the page (PDF
	// +Y). Symmetric with HList.Shift. Callers that need the parent box
	// to "see" the shifted bounding box must pre-compute Height / Depth
	// themselves — Shift is a pure rendering offset.
	Shift    bag.ScaledPoint
	GlueSet  float64
	GlueSign uint8
}

func (v *VList) String() string {
	return "vlist"
}

// Sizes returns the VList's packaged dimensions.
func (v *VList) Sizes(Direction) (w, h, d bag.ScaledPoint) {
	return v.Width, v.Height, v.Depth
}

// DebugAttributes returns the VList's geometry.
func (v *VList) DebugAttributes() ([]kv, H) {
	return []kv{
		{key: "id", value: v.ID},
		{key: "wd", value: v.Width},
		{key: "ht", value: v.Height},
		{key: "dp", value: v.Depth},
	}, v.Attributes
}

// Copy creates a deep copy of the node.
func (v *VList) Copy() Node {
	n := NewVList()
	n.Width = v.Width
	n.Height = v.Height
	n.Depth = v.Depth
	n.GlueSet = v.GlueSet
	n.GlueSign = v.GlueSign
	n.ShiftX = v.ShiftX
	n.Shift = v.Shift
	n.List = CopyList(v.List)
	return n
}

// NewVList creates an initialized VList node
func NewVList() *VList {
	n := vlistSlab.alloc()
	n.ID = newID()
	n.typ = TypeVList
	return n
}
