package document

import (
	"io"
	"sync"
	"testing"
)

// Documents made at the same time share no state: run with -race, this
// fails when NewDocument writes a package variable.
func TestNewDocumentConcurrently(t *testing.T) {
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := NewDocument(io.Discard)
			if d == nil {
				t.Error("NewDocument returned nil")
			}
		}()
	}
	wg.Wait()
}
