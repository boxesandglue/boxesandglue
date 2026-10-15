package node

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
