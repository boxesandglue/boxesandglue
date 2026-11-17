package node

import "github.com/boxesandglue/boxesandglue/backend/bag"

// A Disc represents a hyphenation point. A line that breaks here ends with
// Pre and the next one starts with Post. Without a break the Disc sets
// nothing, and the nodes after it stay as they are.
type Disc struct {
	basenode
	Pre  Node
	Post Node
	// Replace is not used by the line breaker.
	Replace Node
	// ReplaceCount is the number of nodes after the Disc that a break here
	// drops, as TeX's replace_count: a ligature that Pre and Post set
	// apart when the word breaks inside it.
	ReplaceCount int
	Penalty      int // Added to the hyphen penalty
}

func (d *Disc) String() string {
	return String(d)
}

// NewDisc creates an initialized Disc node
func NewDisc() *Disc {
	n := discSlab.alloc()
	n.ID = newID()
	n.typ = TypeDisc
	return n
}

// Copy creates a deep copy of the node.
func (d *Disc) Copy() Node {
	n := NewDisc()
	n.Pre = CopyList(d.Pre)
	n.Post = CopyList(d.Post)
	n.Replace = CopyList(d.Replace)
	n.ReplaceCount = d.ReplaceCount
	n.Penalty = d.Penalty
	n.Attributes = cloneAttributes(d.Attributes)
	return n
}

// NewDiscWithContents creates an initialized Disc node with the given contents
func NewDiscWithContents(d *Disc) *Disc {
	n := discSlab.alloc()
	*n = *d
	n.ID = newID()
	return n
}

// PreWidth is the width that Pre takes at the end of a line. Pre is the
// hyphen, or the glyphs before the break and the hyphen when the Disc
// replaces them. With hang the hyphen at its end hangs into the margin, it
// and the kerns before it take no room, the glyphs before it do.
func (d *Disc) PreWidth(hang bool) bag.ScaledPoint {
	wd, _, _ := Dimensions(d.Pre, nil, Horizontal)
	if hang {
		wd -= hangingWidth(d.Pre)
	}
	return wd
}

// hangingWidth is the width of the glyph at the end of pre and of the kerns
// right before it, the part of Pre that hangs into the margin.
func hangingWidth(pre Node) bag.ScaledPoint {
	n := Tail(pre)
	g, ok := n.(*Glyph)
	if !ok {
		return 0
	}
	wd := g.Width
	for k := n.Prev(); k != nil; k = k.Prev() {
		kern, ok := k.(*Kern)
		if !ok {
			break
		}
		wd += kern.Kern
	}
	return wd
}
