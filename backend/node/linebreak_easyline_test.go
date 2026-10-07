package node

import (
	"slices"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// raggedWords is a paragraph of n five-glyph words, 30pt each, set ragged
// right in 100pt with its first indentRows rows indented by 30pt: three words
// to a line, two on an indented row.
func raggedWords(n, indentRows int) (Node, *LinebreakSettings) {
	words := make([]string, n)
	for i := range words {
		words[i] = "aaaaa"
	}
	settings := NewLinebreakSettings()
	le := NewGlue()
	le.Stretch = bag.Factor
	le.StretchOrder = StretchFill
	settings.LineEndGlue = le
	settings.HSize = 100 * bag.Factor
	settings.LineHeight = 12 * bag.Factor
	if indentRows > 0 {
		settings.Indent = 30 * bag.Factor
		settings.IndentRows = indentRows
	}
	return buildWords(words, 6*bag.Factor, 3*bag.Factor), settings
}

// A ragged line is feasible at any length short of its measure, so a break
// can be reached at almost any number of lines. The lines past the last row
// with a measure of its own share one class, as TeX's easy_line makes them
// (TeX: The Program §848, §835), so a break keeps at most one breakpoint per
// fitness class rather than one per number of lines.
func TestARaggedParagraphKeepsBreakpointsInProportionToItsWords(t *testing.T) {
	const words = 2000
	head, settings := raggedWords(words, 0)
	before := breakpointNextID.Load()
	Linebreak(head, settings)
	if made, most := breakpointNextID.Load()-before, int64(4*words); made > most {
		t.Errorf("made %d breakpoints for %d words, want at most %d", made, words, most)
	}
}

func TestARaggedParagraphFillsItsIndentedRowsAndThenTheRest(t *testing.T) {
	const words = 300
	head, settings := raggedWords(words, 2)
	before := breakpointNextID.Load()
	vlist, _ := Linebreak(head, settings)
	want := []int{2, 2}
	for left := words - 4; left > 0; left -= 3 {
		want = append(want, min(left, 3))
	}
	if got := wordsPerLine(vlist); !slices.Equal(got, want) {
		t.Errorf("words per line = %v, want %v", got, want)
	}
	if made, most := breakpointNextID.Load()-before, int64(4*words); made > most {
		t.Errorf("made %d breakpoints for %d words, want at most %d", made, words, most)
	}
}

// indentedWords sets words of 5pt glyphs ragged right in 100pt, the rows
// IndentRows names indented by indent.
func indentedWords(words []string, indent bag.ScaledPoint, rows int) (*VList, []*Breakpoint) {
	settings := NewLinebreakSettings()
	le := NewGlue()
	le.Stretch = bag.Factor
	le.StretchOrder = StretchFill
	settings.LineEndGlue = le
	settings.HSize = 100 * bag.Factor
	settings.LineHeight = 12 * bag.Factor
	settings.Indent, settings.IndentRows = indent, rows
	return Linebreak(buildWords(words, 5*bag.Factor, 3*bag.Factor), settings)
}

// The first three rows are 30pt, too narrow for the last word, so the
// paragraph sets a word on each of them to reach the fourth. Each indented
// row keeps a class of its own: merged into the last class, the fewest
// lines at each break would set the last word overfull on the third row.
func TestAWordTooWideForTheIndentedRowsWaitsForAFullRow(t *testing.T) {
	vlist, _ := indentedWords([]string{"xx", "xxx", "x", "xxxxxxx"}, 70*bag.Factor, 3)
	if got, want := wordsPerLine(vlist), []int{1, 1, 1, 1}; !slices.Equal(got, want) {
		t.Errorf("words per line = %v, want %v", got, want)
	}
}

// Rows after the first are narrower, so the break after the first word goes
// before the paragraph's start does. The paragraph still starts with its
// first line, and its start is not one of the breakpoints.
func TestAParagraphWhoseStartOutlivesABreakStartsWithItsFirstLine(t *testing.T) {
	word := "xxxxxx"
	vlist, bps := indentedWords([]string{word, word, word, word, word}, 60*bag.Factor, -1)
	if got, want := wordsPerLine(vlist), []int{3, 1, 1}; !slices.Equal(got, want) {
		t.Errorf("words per line = %v, want %v", got, want)
	}
	if _, ok := vlist.List.(*HList); !ok {
		t.Errorf("the paragraph starts with a %T, want its first line", vlist.List)
	}
	for _, bp := range bps {
		if bp.from == nil {
			t.Error("the paragraph's start is one of its breakpoints")
		}
	}
}
