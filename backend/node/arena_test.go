package node

import (
	"sync"
	"testing"
)

// TestSlabRelease ensures that releasing drops the current chunk and that
// nodes allocated before the release stay usable.
func TestSlabRelease(t *testing.T) {
	g := NewGlyph()
	g.Components = "before release"
	releaseChunks()
	if glyphSlab.cur != nil || glyphSlab.pos != 0 {
		t.Errorf("releaseChunks: glyphSlab not empty, cur=%v pos=%d", glyphSlab.cur, glyphSlab.pos)
	}
	if g.Components != "before release" {
		t.Errorf("node allocated before release got corrupted: %q", g.Components)
	}
	if NewGlyph() == nil {
		t.Error("alloc after release returned nil")
	}
}

// TestSlabConcurrency allocates nodes from multiple goroutines while another
// goroutine keeps releasing chunks, mirroring parallel document builds where
// one document calls Finish. Run with -race to check the locking; the
// pointer-uniqueness check below additionally catches two goroutines being
// handed the same slot.
func TestSlabConcurrency(t *testing.T) {
	const (
		goroutines = 8
		perG       = chunkSize + 100 // cross at least one chunk boundary
	)
	allocated := make([][]*Glyph, goroutines)
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			ptrs := make([]*Glyph, 0, perG)
			for range perG {
				ptrs = append(ptrs, NewGlyph())
			}
			allocated[g] = ptrs
		}(g)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 100 {
			releaseChunks()
		}
	}()
	wg.Wait()

	seen := make(map[*Glyph]struct{}, goroutines*perG)
	for _, ptrs := range allocated {
		for _, p := range ptrs {
			if _, ok := seen[p]; ok {
				t.Fatal("same slot handed out twice")
			}
			seen[p] = struct{}{}
		}
	}
}
