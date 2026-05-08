package node

// HardBreak forces a line break. The line breaker treats it like a
// Penalty(-10000) of zero width: the current line ends here without any
// in-line stretch glue, so per-paragraph LineStartGlue / LineEndGlue
// continue to drive alignment unaffected. HardBreak corresponds to HTML
// <br> and to "\n" in source text.
type HardBreak struct {
	basenode
}

func (hb *HardBreak) String() string {
	return String(hb)
}

// Copy creates a deep copy of the node.
func (hb *HardBreak) Copy() Node {
	return NewHardBreak()
}

// NewHardBreak creates an initialized HardBreak node.
func NewHardBreak() *HardBreak {
	n := hardBreakSlab.alloc()
	n.ID = newID()
	n.typ = TypeHardBreak
	return n
}
