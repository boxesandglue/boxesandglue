package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// buildWords lays out words of uniform glyphs separated by interword glue.
//
// The glue stretches and shrinks: with rigid spaces there is often no feasible
// breakpoint in a narrowed measure at all, and the breaker emits overfull lines
// whether or not the inset is doing anything — a fixture that cannot show the
// feature working also cannot show it broken.
func buildWords(words []string, charWidth, spaceWidth bag.ScaledPoint) Node {
	var head, cur Node
	for i, w := range words {
		if i > 0 {
			sp := NewGlue()
			sp.Width = spaceWidth
			sp.Stretch = spaceWidth
			sp.Shrink = spaceWidth / 3
			head = InsertAfter(head, cur, sp)
			cur = sp
		}
		head, cur = glyphRun(head, cur, w, charWidth)
	}
	head, _ = AppendLineEndAfter(head, cur)
	return head
}

// lineContentWidths sums each line's content, skipping the leftskip and
// line-end glue.
//
// Summed rather than taken as hl.Width minus the skips: HpackTo forces the box
// to the full HSize and writes the set glue widths back into the nodes, so that
// subtraction yields a constant that cannot exceed the measure whatever the
// breaker did.
func lineContentWidths(vlist *VList) []bag.ScaledPoint {
	var out []bag.ScaledPoint
	for n := vlist.List; n != nil; n = n.Next() {
		hl, ok := n.(*HList)
		if !ok {
			continue
		}
		var content bag.ScaledPoint
		for m := hl.List; m != nil; m = m.Next() {
			if g, ok := m.(*Glue); ok {
				if o, _ := g.Attributes["origin"].(string); o == "leftskip" || o == "lineend" {
					continue
				}
			}
			w, _, _ := m.Sizes(Horizontal)
			content += w
		}
		out = append(out, content)
	}
	return out
}

// lineEndWidths returns each line's line-end glue: the base glue plus whatever
// right inset that row was given. The row selection is asserted on this rather
// than on content width, where a naturally short last line is indistinguishable
// from a narrowed one.
func lineEndWidths(vlist *VList) []bag.ScaledPoint {
	var out []bag.ScaledPoint
	for n := vlist.List; n != nil; n = n.Next() {
		hl, ok := n.(*HList)
		if !ok {
			continue
		}
		var end bag.ScaledPoint
		for m := hl.List; m != nil; m = m.Next() {
			if g, ok := m.(*Glue); ok {
				if o, _ := g.Attributes["origin"].(string); o == "lineend" {
					end += g.Width
				}
			}
		}
		out = append(out, end)
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
//
// Asserted on the line-end glue, which carries the inset for the rows it applies
// to. The last line is excluded: it absorbs whatever slack is left, so its glue
// is wide whether or not the row was narrowed.
func TestIndentRightRowsSelectsRows(t *testing.T) {
	const charWidth = bag.ScaledPoint(6 * bag.Factor)
	const spaceWidth = bag.ScaledPoint(3 * bag.Factor)
	words := []string{"aaaa", "bbbb", "cccc", "dddd", "eeee", "ffff", "gggg", "hhhh"}

	for _, tc := range []struct {
		name   string
		rows   int
		first  bool // row 0 narrowed?
		second bool // row 1 narrowed?
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

			ends := lineEndWidths(vlist)
			if len(ends) < 3 {
				t.Fatalf("want at least three lines so neither asserted row is the last, got %d", len(ends))
			}
			for i, want := range []bool{tc.first, tc.second} {
				if got := ends[i] >= s.IndentRight; got != want {
					t.Errorf("row %d narrowed = %v (line-end glue %s), want %v", i, got, ends[i], want)
				}
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
