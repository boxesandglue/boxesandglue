package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// markedList builds a paragraph with nothing to set: each glue sits between
// start/stop markers, as a span with only white space in it gives. A space
// between markers is what xts passes for an empty <Paragraph>.
func markedList(glues ...*Glue) Node {
	head := Node(NewStartStop())
	cur := head
	for _, g := range glues {
		head = InsertAfter(head, cur, g)
		cur = g
		stop := NewStartStop()
		head = InsertAfter(head, cur, stop)
		cur = stop
	}
	head, _ = AppendLineEndAfter(head, cur)
	return head
}

// An empty paragraph is one line however narrow the measure. The glue after
// a marker was a breakpoint, so a space wider than the measure broke into
// two lines, and a table row holding such a paragraph in a narrow column
// stood twice as tall.
func TestEmptyParagraphIsOneLine(t *testing.T) {
	sp := NewGlue()
	sp.Width = bag.MustSP("2.2pt")
	sp.Stretch = bag.MustSP("1.1pt")
	s := tabSettings()
	s.HSize = bag.MustSP("0.75pt")
	vlist, _ := Linebreak(markedList(sp), s)
	if n := countHLists(vlist); n != 1 {
		t.Errorf("got %d lines, want 1", n)
	}
}

// A paragraph of only a tab to a stop past the line end is one line too: the
// tab overflows its line rather than breaking before itself.
func TestLoneTabPastTheEndIsOneLine(t *testing.T) {
	s := tabSettings(TabStop{Position: 300 * bag.Factor, Align: TabAlignRight})
	vlist, _ := Linebreak(markedList(newTab()), s)
	if n := countHLists(vlist); n != 1 {
		t.Errorf("got %d lines, want 1", n)
	}
}
