package document

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// scaledGlyphs appends the glyphs of text with the fixed horizontal scale
// to the list, their widths scaled as Mknodes scales them.
func scaledGlyphs(head, cur node.Node, fnt *font.Font, text string, scale float64) (node.Node, node.Node) {
	for _, a := range fnt.Shape(text, nil, nil) {
		g := node.NewGlyph()
		g.Font = fnt
		g.Codepoint = a.Codepoint
		g.Components = a.Components
		g.Width = bag.MultiplyFloat(a.Advance, scale)
		g.HorizontalScale = scale
		head = node.InsertAfter(head, cur, g)
		cur = g
	}
	return head, cur
}

// scaledLine is a line of runs, each with its own scale. With withColor,
// the color switch renderLines looks for sits in front of the last run.
func scaledLine(fnt *font.Font, expand int, withColor bool, runs ...any) *node.VList {
	var head, cur node.Node
	for i := 0; i < len(runs); i += 2 {
		if withColor && i == len(runs)-2 {
			col := node.NewStartStop()
			col.Position = node.PDFOutputPage
			col.ShipoutCallback = func(node.Node) string { return " " + colorSwitch + " " }
			head = node.InsertAfter(head, cur, col)
			cur = col
		}
		head, cur = scaledGlyphs(head, cur, fnt, runs[i].(string), runs[i+1].(float64))
	}
	hl := node.Hpack(head)
	if expand != 0 {
		hl.Attributes = node.H{"expand": expand}
	}
	return node.Vpack(hl)
}

// tzBefore returns the Tz operands written in front of the first glyph of
// each TJ array in the stream.
func tzBefore(stream string) []string {
	var ret []string
	cur := "100"
	var prev string
	for _, tok := range strings.Fields(stream) {
		if tok == "Tz" {
			cur = prev
		}
		if strings.HasPrefix(tok, "[") {
			ret = append(ret, cur)
		}
		prev = tok
	}
	return ret
}

// A scaled glyph keeps its scaled advance: the PDF draws it at that width
// through Tz, so the TJ array needs no correction that would take the
// scale back.
func TestScaledGlyphEmitsNoAdjustment(t *testing.T) {
	out, _, atoms := renderGlyphs(t, "AV", func(g []*node.Glyph) {
		for _, gl := range g {
			gl.Width = bag.MultiplyFloat(gl.Width, 0.9)
			gl.HorizontalScale = 0.9
		}
	})
	if !strings.Contains(out, "90 Tz") {
		t.Errorf("want 90 Tz in the content stream, got:\n%s", out)
	}
	want := fmt.Sprintf("[<%04x%04x>]TJ", atoms[0].Codepoint, atoms[1].Codepoint)
	if !strings.Contains(out, want) {
		t.Errorf("want %q in the content stream, got:\n%s", want, tjLines(out))
	}
}

// A scaled glyph the layout gave another width is corrected in text space:
// Tz scales the TJ value too, so a zero width takes back the unscaled
// advance.
func TestScaledZeroWidthGlyphTakesTheAdvanceBack(t *testing.T) {
	out, fnt, atoms := renderGlyphs(t, "A.", func(g []*node.Glyph) {
		for _, gl := range g {
			gl.Width = bag.MultiplyFloat(gl.Width, 0.9)
			gl.HorizontalScale = 0.9
		}
		g[1].Width = 0
	})
	back := int(math.Round(advanceTJ(fnt, atoms[1].Codepoint)))
	want := fmt.Sprintf("[<%04x%04x> %d ]TJ", atoms[0].Codepoint, atoms[1].Codepoint, back)
	if !strings.Contains(out, want) {
		t.Errorf("want %q in the content stream, got:\n%s", want, tjLines(out))
	}
}

// Font expansion works on the scaled width: Tz is scale * (100 + expand),
// and a factor such as 0.905 needs a Tz that is not a whole number.
func TestScaleAndExpansionMultiply(t *testing.T) {
	stream := renderLines(t, func(fnt *font.Font) []*node.VList {
		return []*node.VList{scaledLine(fnt, 3, true, "bc", 0.905)}
	})
	if tz, _ := textStateAfterColor(stream); tz != "93.215" {
		t.Errorf("Tz %s, want 93.215 (0.905 * 103):\n%s", tz, stream)
	}
}

// The scale belongs to the run, not to the line: two runs on one line each
// get their own Tz.
func TestTwoScalesOnOneLine(t *testing.T) {
	stream := renderLines(t, func(fnt *font.Font) []*node.VList {
		return []*node.VList{scaledLine(fnt, 0, true, "bc", 0.9, "bc", 1.1)}
	})
	if got := tzBefore(stream); len(got) != 2 || got[0] != "90" || got[1] != "110" {
		t.Errorf("Tz before the two runs %v, want [90 110]:\n%s", got, stream)
	}
}

// The scale an object leaves behind must not reach the next one.
func TestScaleDoesNotLeakIntoNextObject(t *testing.T) {
	stream := renderLines(t, func(fnt *font.Font) []*node.VList {
		return []*node.VList{
			scaledLine(fnt, 0, false, "bc", 0.9),
			textLine(fnt, 0, 0, true),
		}
	})
	if tz, _ := textStateAfterColor(stream); tz != "100" {
		t.Errorf("the second object inherits Tz %s, want 100:\n%s", tz, stream)
	}
}

// A space or a kern inside a scaled run moves the text position by its own
// width: its TJ value is in text space, which Tz scales.
func TestScaledGlueAndKernAreNotScaledTwice(t *testing.T) {
	render := func(scale float64) string {
		return renderLines(t, func(fnt *font.Font) []*node.VList {
			head, cur := scaledGlyphs(nil, nil, fnt, "b", scale)
			g := node.NewGlue()
			g.Width = bag.MultiplyFloat(fnt.Space, scale)
			head = node.InsertAfter(head, cur, g)
			k := node.NewKern()
			k.Kern = bag.MultiplyFloat(bag.MustSP("2pt"), scale)
			head = node.InsertAfter(head, g, k)
			col := node.NewStartStop()
			col.Position = node.PDFOutputPage
			col.ShipoutCallback = func(node.Node) string { return " " + colorSwitch + " " }
			node.InsertAfter(head, k, col)
			return []*node.VList{node.Vpack(node.Hpack(head))}
		})
	}
	plain, scaled := tjLines(render(1)), tjLines(render(0.9))
	if plain != scaled {
		t.Errorf("the scaled run moves differently in text space:\n%s\nwant\n%s", scaled, plain)
	}
}
