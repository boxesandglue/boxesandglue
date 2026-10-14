package document

import (
	"regexp"
	"strconv"
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

var hexString = regexp.MustCompile(`<[0-9a-fA-F]*>`)

// tjNumbers returns the numbers in the first TJ array of the stream.
func tjNumbers(t *testing.T, stream string) []int {
	t.Helper()
	start := strings.Index(stream, "[")
	end := strings.Index(stream, "]TJ")
	if start < 0 || end < start {
		t.Fatalf("no TJ array in the stream:\n%s", stream)
	}
	var ret []int
	for _, tok := range strings.Fields(hexString.ReplaceAllString(stream[start+1:end], " ")) {
		if n, err := strconv.Atoi(tok); err == nil {
			ret = append(ret, n)
		}
	}
	return ret
}

// expandedRun renders the glyph b, the node between and, after a color
// switch, the glyph c, in a line with a font expansion of -2.5 percent. It
// returns the stream, the font and the width of b.
func expandedRun(t *testing.T, between node.Node) (string, *font.Font, bag.ScaledPoint) {
	t.Helper()
	var fnt *font.Font
	var advance bag.ScaledPoint
	stream := renderLines(t, func(f *font.Font) []*node.VList {
		fnt = f
		head, cur := glyphRun(nil, nil, f, "b")
		advance = cur.(*node.Glyph).Width
		head = node.InsertAfter(head, cur, between)
		col := colorSwitchNode()
		node.InsertAfter(head, between, col)
		glyphRun(head, col, f, "c")
		hl := node.Hpack(head)
		hl.Attributes = node.H{"expand": -2.5}
		return []*node.VList{node.Vpack(hl)}
	})
	if !strings.Contains(stream, "97.5 Tz") {
		t.Errorf("want 97.5 Tz for an expansion of -2.5:\n%s", stream)
	}
	return stream, fnt, advance
}

// A kern inside an expanded run is a move in the TJ array. Tz would scale
// it with the glyphs, so the move is divided by the expansion and the kern
// keeps its width: only the glyphs are expanded (boxesandglue#86). The
// glyph placed anew after a color switch starts where the PDF drew the
// run's end.
func TestAKernInAnExpandedRunKeepsItsWidth(t *testing.T) {
	k := node.NewKern()
	k.Kern = bag.MustSP("20pt")
	stream, _, advance := expandedRun(t, k)
	nums := tjNumbers(t, stream)
	// 20pt at 12pt are 1666.67 thousandths of the font size, 1709.4 before
	// Tz takes off 2.5 percent.
	if len(nums) != 1 || nums[0] != -1709 {
		t.Errorf("TJ moves %v, want [-1709]:\n%s", nums, stream)
	}
	xs := tmX(stream)
	if len(xs) < 2 {
		t.Fatalf("want a Tm before and after the color switch, got %v:\n%s", xs, stream)
	}
	want := bag.MustSP("1cm") + bag.MultiplyFloat(advance, 0.975) + k.Kern
	if xs[1] != want.String() {
		t.Errorf("the glyph after the color switch starts at x %s, want %s:\n%s", xs[1], want, stream)
	}
}

// A glue inside an expanded run is a space glyph and a move in the TJ
// array, both scaled by Tz. The PDF draws the glue at its set width, as
// the line breaker assumes, so an expanded line ends at the right margin
// (boxesandglue#86).
func TestAGlueInAnExpandedRunKeepsItsWidth(t *testing.T) {
	g := node.NewGlue()
	g.Width = bag.MustSP("20pt")
	stream, fnt, advance := expandedRun(t, g)
	nums := tjNumbers(t, stream)
	if len(nums) != 1 {
		t.Fatalf("TJ moves %v, want one for the glue:\n%s", nums, stream)
	}
	// The space glyph advances by its width, the move takes off the rest;
	// TJ counts in thousandths of the size given to Tf.
	face := fnt.Face
	spaceAdv := face.AdvanceWidth(face.Codepoint(' '))
	drawn := (spaceAdv - float64(nums[0])*face.Scale/1000) * fnt.Size.ToPT() * 0.975
	if diff := drawn - 20; diff > 0.015 || diff < -0.015 {
		t.Errorf("the PDF draws the glue %.3fpt wide, want 20pt:\n%s", drawn, stream)
	}
	xs := tmX(stream)
	if len(xs) < 2 {
		t.Fatalf("want a Tm before and after the color switch, got %v:\n%s", xs, stream)
	}
	want := bag.MustSP("1cm") + bag.MultiplyFloat(advance, 0.975) + g.Width
	if xs[1] != want.String() {
		t.Errorf("the glyph after the color switch starts at x %s, want %s:\n%s", xs[1], want, stream)
	}
}
