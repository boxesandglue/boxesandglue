package node

// Slab allocator for node types. Instead of allocating each node individually
// on the heap, nodes are allocated from fixed-size chunks. This reduces GC
// pressure significantly because the garbage collector sees one large array
// per chunk instead of hundreds of individual small objects.
//
// The chunks use fixed-size arrays ([chunkSize]T), so pointers into them
// remain stable (unlike slices, which can be relocated by append).
//
// Only the current chunk is referenced from here. Once a chunk is full it is
// abandoned: its lifetime is then governed solely by the node pointers handed
// out to callers, so dropping a document releases its nodes (and everything
// they retain, such as fonts and images) to the garbage collector. Keeping a
// list of all chunks instead would pin every node ever allocated for the
// lifetime of the process (issue #21).

const chunkSize = 8192

// slab is a generic chunked allocator. It allocates objects of type T from
// fixed-size array chunks. Each chunk is a single heap allocation containing
// chunkSize elements.
type slab[T any] struct {
	cur *[chunkSize]T
	pos int // next free slot in the current chunk
}

// alloc returns a pointer to a zero-valued T from the slab.
func (s *slab[T]) alloc() *T {
	if s.cur == nil || s.pos >= chunkSize {
		s.cur = new([chunkSize]T)
		s.pos = 0
	}
	ptr := &s.cur[s.pos]
	s.pos++
	return ptr
}

// Package-level slabs for each node type.
var (
	glyphSlab     slab[Glyph]
	glueSlab      slab[Glue]
	kernSlab      slab[Kern]
	hlistSlab     slab[HList]
	vlistSlab     slab[VList]
	penaltySlab   slab[Penalty]
	ruleSlab      slab[Rule]
	discSlab      slab[Disc]
	langSlab      slab[Lang]
	startStopSlab slab[StartStop]
	imageSlab     slab[Image]
	hardBreakSlab slab[HardBreak]
)
