package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// buildWords lays out words of uniform glyphs separated by fixed glue, the
// shape the other linebreak tests use.
func buildWords(words []string, charWidth, spaceWidth bag.ScaledPoint) Node {
	var head, cur Node
	for i, w := range words {
		if i > 0 {
			sp := NewGlue()
			sp.Width = spaceWidth
			head = InsertAfter(head, cur, sp)
			cur = sp
		}
		head, cur = glyphRun(head, cur, w, charWidth)
	}
	head, _ = AppendLineEndAfter(head, cur)
	return head
}

// lineWidths returns the natural width of the content of each line: the packed
// hbox less the skips inserted around it. That is the measure the breaker
// actually had to work with.
func lineContentWidths(vlist *VList) []bag.ScaledPoint {
	var out []bag.ScaledPoint
	for n := vlist.List; n != nil; n = n.Next() {
		hl, ok := n.(*HList)
		if !ok {
			continue
		}
		var skips bag.ScaledPoint
		for m := hl.List; m != nil; m = m.Next() {
			if g, ok := m.(*Glue); ok {
				if o, _ := g.Attributes["origin"].(string); o == "leftskip" || o == "lineend" {
					skips += g.Width
				}
			}
		}
		out = append(out, hl.Width-skips)
	}
	return out
}

// TestIndentRightNarrowsFromTheEnd: the right inset takes width off the end of
// a line without moving where the line starts, which is what lets text sit
// beside something on the right.
func TestIndentRightNarrowsFromTheEnd(t *testing.T) {
	const charWidth = bag.ScaledPoint(6 * bag.Factor)
	const spaceWidth = bag.ScaledPoint(3 * bag.Factor)
	words := []string{"aaaa", "bbbb", "cccc", "dddd", "eeee", "ffff"}

	settings := func() *LinebreakSettings {
		s := NewLinebreakSettings()
		s.HSize = 120 * bag.Factor
		s.LineHeight = 12 * bag.Factor
		return s
	}

	plain, _ := Linebreak(buildWords(words, charWidth, spaceWidth), settings())

	narrowed := settings()
	narrowed.IndentRight = 60 * bag.Factor
	inset, _ := Linebreak(buildWords(words, charWidth, spaceWidth), narrowed)

	// Without the inset the text uses more than the narrowed measure; with it,
	// no line may. Asserted on the content widths rather than on a line count,
	// which depends on the breaker's tolerance rather than on the measure.
	limit := narrowed.HSize - narrowed.IndentRight
	if widest(lineContentWidths(plain)) <= limit {
		t.Fatalf("the unconstrained text already fits in %s; the test proves nothing", limit)
	}

	// The line box still spans the full hsize: the inset is width given to the
	// line-end glue, not a shorter box. Alignment inside the remaining measure
	// depends on this.
	for n := inset.List; n != nil; n = n.Next() {
		if hl, ok := n.(*HList); ok && hl.Width != narrowed.HSize {
			t.Errorf("line packed to %s, want the full hsize %s", hl.Width, narrowed.HSize)
		}
	}

	// And no line's content exceeds what is left of the measure.
	for i, w := range lineContentWidths(inset) {
		if w > limit {
			t.Errorf("line %d holds %s of content, more than the %s left beside the inset", i, w, limit)
		}
	}
}

// TestIndentRightRowsSelectsRows: the row selector means the same thing it
// means for the left inset — first n rows, or all rows but the first n.
func TestIndentRightRowsSelectsRows(t *testing.T) {
	const charWidth = bag.ScaledPoint(6 * bag.Factor)
	const spaceWidth = bag.ScaledPoint(3 * bag.Factor)
	words := []string{"aaaa", "bbbb", "cccc", "dddd", "eeee", "ffff", "gggg", "hhhh"}

	for _, tc := range []struct {
		name   string
		rows   int
		inset  bool // whether row 0 is expected to be narrowed
		lastIn bool // whether the last row is expected to be narrowed
	}{
		{"all rows", 0, true, true},
		{"first row only", 1, true, false},
		{"all but the first", -1, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewLinebreakSettings()
			s.HSize = 120 * bag.Factor
			s.LineHeight = 12 * bag.Factor
			s.IndentRight = 60 * bag.Factor
			s.IndentRightRows = tc.rows
			vlist, _ := Linebreak(buildWords(words, charWidth, spaceWidth), s)

			widths := lineContentWidths(vlist)
			if len(widths) < 2 {
				t.Fatalf("expected several lines, got %d", len(widths))
			}
			limit := s.HSize - s.IndentRight
			if got := widths[0] <= limit; got != tc.inset {
				t.Errorf("row 0 narrowed = %v (%s of content), want %v", got, widths[0], tc.inset)
			}
			if got := widths[len(widths)-1] <= limit; got != tc.lastIn {
				t.Errorf("last row narrowed = %v (%s of content), want %v", got, widths[len(widths)-1], tc.lastIn)
			}
		})
	}
}


func widest(widths []bag.ScaledPoint) bag.ScaledPoint {
	var max bag.ScaledPoint
	for _, w := range widths {
		if w > max {
			max = w
		}
	}
	return max
}
