package node

import (
	"slices"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// leftskipWidths returns each line's leftskip, which carries the left inset
// of its row.
func leftskipWidths(vlist *VList) []bag.ScaledPoint {
	var out []bag.ScaledPoint
	for n := vlist.List; n != nil; n = n.Next() {
		hl, ok := n.(*HList)
		if !ok {
			continue
		}
		for m := hl.List; m != nil; m = m.Next() {
			if g, ok := m.(*Glue); ok {
				if o, _ := g.Attributes["origin"].(string); o == "leftskip" {
					out = append(out, g.Width)
					break
				}
			}
		}
	}
	return out
}

func TestIndentStepsForRow(t *testing.T) {
	pt := func(n int) bag.ScaledPoint { return bag.ScaledPoint(n) * bag.Factor }
	for _, tc := range []struct {
		name  string
		steps []IndentStep
		want  []bag.ScaledPoint
	}{
		{"steps then nothing", []IndentStep{{pt(30), 2}, {pt(10), 1}}, []bag.ScaledPoint{pt(30), pt(30), pt(10), 0, 0}},
		{"a last step for every row", []IndentStep{{pt(30), 1}, {pt(10), 0}}, []bag.ScaledPoint{pt(30), pt(10), pt(10), pt(10), pt(10)}},
		{"a step of nothing in between", []IndentStep{{pt(30), 1}, {0, 2}, {pt(5), 1}}, []bag.ScaledPoint{pt(30), 0, 0, pt(5), 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ls := &LinebreakSettings{IndentSteps: tc.steps, IndentRightSteps: tc.steps}
			for row, want := range tc.want {
				if got := ls.IndentForRow(row); got != want {
					t.Errorf("left inset of row %d = %s, want %s", row, got, want)
				}
				if got := ls.IndentRightForRow(row); got != want {
					t.Errorf("right inset of row %d = %s, want %s", row, got, want)
				}
			}
		})
	}
}

// The steps reach the lines: each row's leftskip and line-end glue carry
// the inset the steps give that row. Justified, so the line-end glue has no
// stretch and is exactly the inset; the last line is left out, it takes the
// slack.
func TestIndentStepsSetTheRows(t *testing.T) {
	words := make([]string, 40)
	for i := range words {
		words[i] = "aa"
	}
	s := NewLinebreakSettings()
	s.HSize = 120 * bag.Factor
	s.LineHeight = 12 * bag.Factor
	s.IndentSteps = []IndentStep{{40 * bag.Factor, 2}, {20 * bag.Factor, 1}}
	s.IndentRightSteps = []IndentStep{{10 * bag.Factor, 1}, {30 * bag.Factor, 3}}
	vlist, _ := Linebreak(buildWords(words, 6*bag.Factor, 3*bag.Factor), s)

	lefts, ends := leftskipWidths(vlist), lineEndWidths(vlist)
	if len(lefts) < 6 {
		t.Fatalf("want at least six lines so every step has a row that is not the last, got %d", len(lefts))
	}
	for row := range len(lefts) - 1 {
		if got, want := lefts[row], s.IndentForRow(row); got != want {
			t.Errorf("row %d: leftskip %s, want %s", row, got, want)
		}
		if got, want := ends[row], s.IndentRightForRow(row); got != want {
			t.Errorf("row %d: line-end glue %s, want %s", row, got, want)
		}
	}
	// Packing a line rounds the glue it sets, by a few sp either way.
	const rounding = bag.ScaledPoint(16)
	for i, w := range lineContentWidths(vlist) {
		if limit := s.HSize - s.IndentForRow(i) - s.IndentRightForRow(i); w > limit+rounding {
			t.Errorf("row %d holds %s of content, more than the %s its insets leave", i, w, limit)
		}
	}
}

// Steps on one side replace the plain inset of that side only: the other
// side keeps its own, the way a float on the right leaves a text-indent on
// the left alone.
func TestIndentStepsLeaveTheOtherSideAlone(t *testing.T) {
	s := NewLinebreakSettings()
	s.Indent, s.IndentRows = 15*bag.Factor, 1
	s.IndentRight, s.IndentRightRows = 50*bag.Factor, 0
	s.IndentRightSteps = []IndentStep{{30 * bag.Factor, 2}}
	if got := s.IndentForRow(0); got != 15*bag.Factor {
		t.Errorf("left inset of row 0 = %s, want the plain 15pt", got)
	}
	for row, want := range []bag.ScaledPoint{30 * bag.Factor, 30 * bag.Factor, 0} {
		if got := s.IndentRightForRow(row); got != want {
			t.Errorf("right inset of row %d = %s, want %s from the steps", row, got, want)
		}
	}
}

// The rows of every step keep a class of their own (TeX's easy_line): the
// first three rows are 30pt in two steps, too narrow for the last word, so
// the paragraph sets a word on each of them to reach the fourth. Counting
// only the first step would merge the second step's rows into the full-width
// class and set the last word overfull.
func TestAWordTooWideForTheStepsWaitsForAFullRow(t *testing.T) {
	settings := NewLinebreakSettings()
	le := NewGlue()
	le.Stretch = bag.Factor
	le.StretchOrder = StretchFill
	settings.LineEndGlue = le
	settings.HSize = 100 * bag.Factor
	settings.LineHeight = 12 * bag.Factor
	settings.IndentSteps = []IndentStep{{70 * bag.Factor, 1}, {70 * bag.Factor, 2}}
	vlist, _ := Linebreak(buildWords([]string{"xx", "xxx", "x", "xxxxxxx"}, 5*bag.Factor, 3*bag.Factor), settings)
	if got, want := wordsPerLine(vlist), []int{1, 1, 1, 1}; !slices.Equal(got, want) {
		t.Errorf("words per line = %v, want %v", got, want)
	}
}
