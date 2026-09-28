package node

import (
	"fmt"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// trailingModel and halfModel are the built-in leading as LineModels.
type trailingModel struct{}

func (trailingModel) LineBox(hl *HList, _ *LinebreakSettings) (bag.ScaledPoint, bag.ScaledPoint) {
	return hl.Height, hl.Depth
}

func (trailingModel) Leading(hl *HList, s *LinebreakSettings) *Glue {
	g := NewGlue()
	if total := hl.Height + hl.Depth; total < s.LineHeight {
		g.Width = s.LineHeight - total
	}
	return g
}

type halfModel struct{}

func (halfModel) LineBox(hl *HList, s *LinebreakSettings) (bag.ScaledPoint, bag.ScaledPoint) {
	if extra := s.LineHeight - hl.Height - hl.Depth; extra > 0 {
		return hl.Height + extra/2, hl.Depth + extra - extra/2
	}
	return hl.Height, hl.Depth
}

func (halfModel) Leading(*HList, *LinebreakSettings) *Glue { return nil }

// fixedModel sets every line 9pt + 4pt and puts 2pt between lines.
type fixedModel struct{ boxes, leadings int }

func (m *fixedModel) LineBox(*HList, *LinebreakSettings) (bag.ScaledPoint, bag.ScaledPoint) {
	m.boxes++
	return 9 * bag.Factor, 4 * bag.Factor
}

func (m *fixedModel) Leading(*HList, *LinebreakSettings) *Glue {
	m.leadings++
	g := NewGlue()
	g.Width = 2 * bag.Factor
	return g
}

// shape describes a paragraph's lines and glue in order.
func shape(vl *VList) string {
	var b strings.Builder
	for n := vl.List; n != nil; n = n.Next() {
		switch t := n.(type) {
		case *HList:
			fmt.Fprintf(&b, "line %s+%s; ", t.Height, t.Depth)
		case *Glue:
			fmt.Fprintf(&b, "%v %s; ", t.Attributes["origin"], t.Width)
		}
	}
	fmt.Fprintf(&b, "total %s+%s", vl.Height, vl.Depth)
	return b.String()
}

func lineModelSettings(half, omitLast bool, lm LineModel) *LinebreakSettings {
	s := NewLinebreakSettings()
	s.HSize = 51 * bag.Factor // two words and a space
	s.LineHeight = 12 * bag.Factor
	s.HalfLeading = half
	s.OmitLastLeading = omitLast
	s.LineModel = lm
	return s
}

func threeLineParagraph() Node {
	var head, cur Node
	for i := range 6 {
		if i > 0 {
			sp := NewGlue()
			sp.Width = 3 * bag.Factor
			sp.Stretch = bag.Factor
			head = InsertAfter(head, cur, sp)
			cur = sp
		}
		head, cur = glyphRunHD(head, cur, "word", 6*bag.Factor, 7*bag.Factor, 3*bag.Factor)
	}
	head, _ = AppendLineEndAfter(head, cur)
	return head
}

// The built-in leading, written as LineModels, sets a paragraph exactly as
// Linebreak does without one, so the seam carries both.
func TestLineModelReproducesBuiltins(t *testing.T) {
	for _, c := range []struct {
		name           string
		half, omitLast bool
		model          LineModel
	}{
		{"trailing", false, false, trailingModel{}},
		{"trailing, omit last", false, true, trailingModel{}},
		{"half", true, false, halfModel{}},
	} {
		want, _ := Linebreak(threeLineParagraph(), lineModelSettings(c.half, c.omitLast, nil))
		got, _ := Linebreak(threeLineParagraph(), lineModelSettings(false, c.omitLast, c.model))
		if len(linesOf(want)) != 3 {
			t.Fatalf("%s: got %d lines, want 3", c.name, len(linesOf(want)))
		}
		if g, w := shape(got), shape(want); g != w {
			t.Errorf("%s:\n got  %s\n want %s", c.name, g, w)
		}
	}
}

// A model's line box and leading are what Linebreak builds, and it takes
// precedence over HalfLeading.
func TestLineModelSetsLinesAndLeading(t *testing.T) {
	m := &fixedModel{}
	vl, _ := Linebreak(threeLineParagraph(), lineModelSettings(true, false, m))
	want := "line 9+4; lineskip 2; line 9+4; lineskip 2; line 9+4; last lineskip 2; total 45+0"
	if got := shape(vl); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	if m.boxes != 3 || m.leadings != 3 {
		t.Errorf("LineBox called %d times, Leading %d, want 3 and 3", m.boxes, m.leadings)
	}
	m = &fixedModel{}
	vl, _ = Linebreak(threeLineParagraph(), lineModelSettings(false, true, m))
	if got, want := shape(vl), "line 9+4; lineskip 2; line 9+4; lineskip 2; line 9+4; total 39+4"; got != want {
		t.Errorf("OmitLastLeading:\ngot  %s\nwant %s", got, want)
	}
}

// A model's glyphs carry their LineShift through a copy.
func TestGlyphCopyKeepsLineShift(t *testing.T) {
	g := NewGlyph()
	g.YOffset, g.LineShift = 5*bag.Factor, 3*bag.Factor
	c := g.Copy().(*Glyph)
	if c.LineShift != g.LineShift || c.YOffset != g.YOffset {
		t.Errorf("copy has LineShift %s YOffset %s, want %s and %s", c.LineShift, c.YOffset, g.LineShift, g.YOffset)
	}
}

// Without a model a paragraph is set as it was before LineModel: lineskip
// glue pads each line to LineHeight, or half the leading goes each side.
func TestNoLineModelIsUnchanged(t *testing.T) {
	for _, c := range []struct {
		half bool
		want string
	}{
		{false, "line 7+3; lineskip 2; line 7+3; lineskip 2; line 7+3; last lineskip 2; total 36+0"},
		{true, "line 8+4; line 8+4; line 8+4; total 32+4"},
	} {
		vl, _ := Linebreak(threeLineParagraph(), lineModelSettings(c.half, false, nil))
		if got := shape(vl); got != c.want {
			t.Errorf("half leading %t:\ngot  %s\nwant %s", c.half, got, c.want)
		}
	}
}
