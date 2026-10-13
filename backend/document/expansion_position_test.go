package document

import (
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// tmX returns the x operand of every Tm in the stream.
func tmX(stream string) []string {
	var ret []string
	toks := strings.Fields(stream)
	for i, tok := range toks {
		if tok == "Tm" && i >= 2 {
			ret = append(ret, toks[i-2])
		}
	}
	return ret
}

// glyphRun appends the glyphs of text to the list at their advance.
func glyphRun(head, cur node.Node, fnt *font.Font, text string) (node.Node, node.Node) {
	for _, a := range fnt.Shape(text, nil, nil) {
		g := node.NewGlyph()
		g.Font = fnt
		g.Codepoint = a.Codepoint
		g.Components = a.Components
		g.Width = a.Advance
		head = node.InsertAfter(head, cur, g)
		cur = g
	}
	return head, cur
}

func colorSwitchNode() *node.StartStop {
	col := node.NewStartStop()
	col.Position = node.PDFOutputPage
	col.ShipoutCallback = func(node.Node) string { return " " + colorSwitch + " " }
	return col
}

// A line after a line with font expansion starts at its indent. The indent
// is a glue before the line's first glyph, which is placed with Tm; the
// expansion the line before left in the text state does not scale it
// (boxesandglue#85).
func TestAnIndentAfterAnExpandedLineIsNotScaled(t *testing.T) {
	indent := bag.MustSP("100pt")
	stream := renderLines(t, func(fnt *font.Font) []*node.VList {
		first, _ := glyphRun(nil, nil, fnt, "bc")
		line1 := node.Hpack(first)
		line1.Attributes = node.H{"expand": -3}

		leftskip := node.NewGlue()
		leftskip.Width = indent
		head, cur := glyphRun(leftskip, leftskip, fnt, "bc")
		node.InsertAfter(head, cur, colorSwitchNode())
		line2 := node.Hpack(head)

		node.InsertAfter(line1, line1, line2)
		return []*node.VList{node.Vpack(line1)}
	})
	xs := tmX(stream)
	if len(xs) < 2 {
		t.Fatalf("want a Tm for each line, got %v:\n%s", xs, stream)
	}
	if want := (bag.MustSP("1cm") + indent).String(); xs[1] != want {
		t.Errorf("the second line starts at x %s, want %s (1cm and its indent):\n%s", xs[1], want, stream)
	}
}

// A kern inside an expanded run is a move in the TJ array, which Tz scales
// with the glyphs. The position after it follows the PDF: the glyph placed
// anew after a color switch starts where the scaled run ended.
func TestAKernInAnExpandedRunMovesWithIt(t *testing.T) {
	kern := bag.MustSP("20pt")
	var advance bag.ScaledPoint
	stream := renderLines(t, func(fnt *font.Font) []*node.VList {
		head, cur := glyphRun(nil, nil, fnt, "b")
		advance = cur.(*node.Glyph).Width
		k := node.NewKern()
		k.Kern = kern
		head = node.InsertAfter(head, cur, k)
		col := colorSwitchNode()
		node.InsertAfter(head, k, col)
		glyphRun(head, col, fnt, "c")
		hl := node.Hpack(head)
		hl.Attributes = node.H{"expand": -3}
		return []*node.VList{node.Vpack(hl)}
	})
	xs := tmX(stream)
	if len(xs) < 2 {
		t.Fatalf("want a Tm before and after the color switch, got %v:\n%s", xs, stream)
	}
	want := bag.MustSP("1cm") + bag.MultiplyFloat(advance, 0.97) + bag.MultiplyFloat(kern, 0.97)
	if xs[1] != want.String() {
		t.Errorf("the glyph after the color switch starts at x %s, want %s:\n%s", xs[1], want, stream)
	}
}
