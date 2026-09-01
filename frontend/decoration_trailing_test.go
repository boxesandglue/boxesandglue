package frontend

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// TestLastInkedStopsAtText checks where a decoration left open at a line break
// ends. The nodes at the end of a broken line are the break's own glue and the
// alignment filler, and drawing through them ran the underline out to the
// margin: a wrapped underlined heading came out underlined across the gap
// after its last word.
func TestLastInkedStopsAtText(t *testing.T) {
	glyph := func() node.Node {
		r := node.NewRule() // stands in for text: any node that is not glue
		r.Width = bag.MustSP("10pt")
		return r
	}
	glue := func() node.Node {
		g := node.NewGlue()
		g.Width = bag.MustSP("4pt")
		return g
	}

	build := func(items ...node.Node) (node.Node, node.Node) {
		var head, tail node.Node
		for _, it := range items {
			head = node.InsertAfter(head, tail, it)
			tail = it
		}
		return head, tail
	}

	t.Run("trailing glue is trimmed", func(t *testing.T) {
		want := glyph()
		head, tail := build(glyph(), glue(), want, glue())
		if got := lastInked(head, tail); got != want {
			t.Errorf("stopped at %T, want the last non-glue node", got)
		}
	})

	t.Run("a line ending in text is unchanged", func(t *testing.T) {
		want := glyph()
		head, tail := build(glyph(), glue(), want)
		if got := lastInked(head, tail); got != want {
			t.Errorf("stopped at %T, want the final node itself", got)
		}
	})

	t.Run("interior glue is kept", func(t *testing.T) {
		// The spaces between words stay underlined; only what trails does not.
		want := glyph()
		head, tail := build(glyph(), glue(), glue(), want)
		if got := lastInked(head, tail); got != want {
			t.Errorf("stopped at %T, want the last node", got)
		}
	})

	t.Run("a line of nothing but glue falls back to the end", func(t *testing.T) {
		head, tail := build(glue(), glue())
		if got := lastInked(head, tail); got != tail {
			t.Errorf("got %T, want the original stop rather than nothing", got)
		}
	})
}
