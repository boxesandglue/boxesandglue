package node

// A Disc represents a hyphenation point. Currently only the Penalty field is
// used.
type Disc struct {
	basenode
	Pre     Node
	Post    Node
	Replace Node
	Penalty int // Added to the hyphen penalty
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
	n.Penalty = d.Penalty
	return n
}

// NewDiscWithContents creates an initialized Disc node with the given contents
func NewDiscWithContents(d *Disc) *Disc {
	n := discSlab.alloc()
	*n = *d
	n.ID = newID()
	return n
}
