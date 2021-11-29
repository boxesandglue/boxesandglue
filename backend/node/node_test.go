package node

import (
	"fmt"
	"strings"
	"testing"

	"github.com/speedata/boxesandglue/backend/bag"
)

type gluTestData struct {
	gluewd       int
	gluestretch  int
	glueshrink   int
	stretchOrder GlueOrder
	shrinkOrder  GlueOrder
}

func (g gluTestData) String() string {
	ret := []string{fmt.Sprintf("%spt", bag.ScaledPoint(g.gluewd)*bag.Factor)}

	if g.stretchOrder > 0 {
		ret = append(ret, " plus ")
		ret = append(ret, fmt.Sprintf("%d", int(g.gluestretch/65536)))
		ret = append(ret, "fi"+strings.Repeat("l", int(g.stretchOrder)))
	}

	if g.shrinkOrder > 0 {
		ret = append(ret, " minus ")
		ret = append(ret, fmt.Sprintf("%d", int(g.shrinkOrder/65536)))
		ret = append(ret, "fi"+strings.Repeat("l", int(g.shrinkOrder)))
	}
	return strings.Join(ret, "")
}

type testdata struct {
	desiredWidth bag.ScaledPoint
	badness      int
	glues        []gluTestData
}

func (td testdata) String() string {
	ret := []string{fmt.Sprintf("badness: %d, dw: %s", td.badness, td.desiredWidth)}
	for _, g := range td.glues {
		ret = append(ret, g.String())
	}
	return strings.Join(ret, ", ")
}

func TestHpack(t *testing.T) {
	data := []testdata{
		{8 * bag.Factor, 30, []gluTestData{{4, 6, 0, 0, 0}}},
		{10 * bag.Factor, 10000, []gluTestData{{20, 6, 3, 0, 0}}},
		{100 * bag.Factor, 10000, []gluTestData{{12, 4, 3, 0, 0}}},
		{18 * bag.Factor, 338, []gluTestData{{12, 4, 0, 0, 0}}},
		{9 * bag.Factor, 58, []gluTestData{{4, 6, 0, 0, 0}}},
		{19 * bag.Factor, 1563, []gluTestData{{4, 6, 0, 0, 0}}},
		{33 * bag.Factor, 10000, []gluTestData{{4, 6, 0, 0, 0}}},
		{100 * bag.Factor, 0, []gluTestData{{4, 6, 0, 0, 0}, {4, 65536, 0, 1, 0}}},
		{100 * bag.Factor, 0, []gluTestData{{4, 6, 0, 0, 0}, {4, 65536, 65536, 1, 2}}},
	}
	for _, d := range data {
		var head, cur Node
		for _, g := range d.glues {
			gluenode := NewGlue()
			gluenode.Width = bag.ScaledPoint(g.gluewd) * bag.Factor
			gluenode.Stretch = bag.ScaledPoint(g.gluestretch) * bag.Factor
			gluenode.Shrink = bag.ScaledPoint(g.glueshrink) * bag.Factor
			gluenode.ShrinkOrder = g.shrinkOrder
			gluenode.StretchOrder = g.stretchOrder
			head = InsertAfter(head, cur, gluenode)
			cur = gluenode
		}
		hl, badness := HpackTo(head, d.desiredWidth)

		if hl.Width != d.desiredWidth {
			t.Errorf("hl.Width %d want %d (%s)", hl.Width, d.desiredWidth, d)
		}
		if badness != d.badness {
			t.Errorf("badness = %d, want %d", badness, d.badness)
		}
	}
}

func TestLinebreak(t *testing.T) {
	str := `In olden times when wish|ing still helped one, there lived a king whose daugh|ters
were all beau|ti|ful; and the young|est was so beau|ti|ful that the sun it|self, which
has seen so much, was aston|ished when|ever it shone in her face. Close by the
king's castle lay a great dark for|est, and un|der an old lime-*tree in the for|est
was a well, and when the day was very warm, the king's child went out into the
for|est and sat down by the side of the cool foun|tain; and when she was bored she
took a golden ball, and threw it up on high and caught it; and this ball was her
favor|ite play|thing.`
	widths := map[rune]int{
		'a':  9,
		'b':  10,
		'c':  8,
		'd':  10,
		'e':  8,
		'f':  6,
		'g':  9,
		'h':  10,
		'i':  5,
		'j':  6,
		'k':  10,
		'l':  5,
		'm':  15,
		'n':  10,
		'o':  9,
		'p':  10,
		'q':  10,
		'r':  7,
		's':  7,
		't':  7,
		'u':  10,
		'v':  9,
		'w':  13,
		'x':  10,
		'y':  10,
		'z':  8,
		'C':  13,
		'I':  6,
		'-':  6,
		',':  5,
		';':  5,
		'\'': 5,
		'.':  5,
	}

	hyphenchar := NewGlyph()
	hyphenchar.Width = 6 * bag.Factor

	var cur, head Node
	startGlue := NewGlyph()
	startGlue.Width = 18 * bag.Factor
	head = startGlue
	cur = head
	sumwd := bag.ScaledPoint(18)

	var prevGlyph rune
	for _, r := range str {
		if r == 32 || r == 10 || r == 9 {
			g := NewGlue()
			switch prevGlyph {
			case ',':
				g.Width = 6 * bag.Factor
				g.Stretch = 4 * bag.Factor
				g.Shrink = 2 * bag.Factor
			case ';':
				g.Width = 6 * bag.Factor
				g.Stretch = 4 * bag.Factor
				g.Shrink = 1 * bag.Factor
			case '.':
				g.Width = 8 * bag.Factor
				g.Stretch = 6 * bag.Factor
				g.Shrink = 1 * bag.Factor
			default:
				g.Width = 6 * bag.Factor
				g.Stretch = 3 * bag.Factor
				g.Shrink = 2 * bag.Factor
			}
			head = InsertAfter(head, cur, g)
			cur = g
			sumwd += g.Width
		} else if r == '|' {
			p := NewDisc()
			p.Pre = hyphenchar.Copy()
			InsertAfter(head, cur, p)
			cur = p
		} else if r == '*' {
			p := NewDisc()
			InsertAfter(head, cur, p)
			cur = p
		} else {
			g := NewGlyph()
			g.Width = bag.ScaledPoint(widths[r]) * bag.Factor
			head = InsertAfter(head, cur, g)
			cur = g
			sumwd += g.Width
			g.Components = string(r)
			prevGlyph = r
		}
	}

	AppendLineEndAfter(cur)

	settings := NewLinebreakSettings()
	settings.HSize = 390 * bag.Factor
	settings.LineHeight = 12 * bag.Factor
	settings.Hyphenpenalty = 50

	_, bps := Linebreak(head, settings)

	data := []float64{0.8571428571428571, 0, 0.28, 1, 0.06666666666666667, -0.2777777777777778, 0.5357142857142857, -0.16666666666666666, 0.7, -0.17647058823529413, 0.35714285714285715, 0}
	for i, bp := range bps {
		if bp.R != data[i] {
			t.Errorf("Line %d r = %f, want %f", i, bp.R, data[i])
		}
	}
}
