package node

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
)

// Glyph nodes represents a single visible entity such as a letter or a
// ligature.
type Glyph struct {
	basenode
	Font *font.Font
	// A codepoint can contain more than one rune, for example a fi ligature
	// contains f + i. Filling the components string is optional.
	Components string
	// The font specific glyph id
	Codepoint int
	// The advance width of the box.
	Width bag.ScaledPoint
	// The height is the length above the base line.
	Height bag.ScaledPoint
	// The Depth is the length below the base line. For example the letter g has
	// a depth > 0.
	Depth bag.ScaledPoint
	// Horizontal displacement. Positive values move the glyph to the right.
	// Used for GPOS mark positioning without affecting text flow.
	XOffset bag.ScaledPoint
	// Vertical displacement. Positive values move the glyph towards the top of
	// the page.
	YOffset bag.ScaledPoint
	// This allows the glyph to be part of word hyphenation.
	Hyphenate bool
}

func (g *Glyph) String() string {
	return String(g)
}

// Sizes returns the glyph's advance width, height and depth.
func (g *Glyph) Sizes(Direction) (w, h, d bag.ScaledPoint) {
	return g.Width, g.Height, g.Depth
}

// DebugAttributes returns the glyph's full descriptive attribute set
// (components, geometry, codepoint, font face id).
func (g *Glyph) DebugAttributes() ([]kv, H) {
	var fontid int
	if g.Font != nil && g.Font.Face != nil {
		fontid = g.Font.Face.FaceID
	}
	return []kv{
		{key: "id", value: g.ID},
		{key: "components", value: g.Components},
		{key: "wd", value: g.Width},
		{key: "ht", value: g.Height},
		{key: "dp", value: g.Depth},
		{key: "codepoint", value: g.Codepoint},
		{key: "face", value: fontid},
	}, g.Attributes
}

// Copy creates a deep copy of the node.
func (g *Glyph) Copy() Node {
	n := NewGlyph()
	n.Font = g.Font
	n.Codepoint = g.Codepoint
	n.Components = g.Components
	n.Width = g.Width
	n.Height = g.Height
	n.Depth = g.Depth
	n.Hyphenate = g.Hyphenate
	n.YOffset = g.YOffset
	n.Hyphenate = g.Hyphenate
	return n
}

// NewGlyph returns an initialized Glyph
func NewGlyph() *Glyph {
	n := glyphSlab.alloc()
	n.ID = newID()
	n.typ = TypeGlyph
	return n
}
