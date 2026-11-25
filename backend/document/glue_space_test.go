package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// countGlyph returns how often the glyph id want is in glyphs.
func countGlyph(glyphs []string, want string) int {
	n := 0
	for _, g := range glyphs {
		if g == want {
			n++
		}
	}
	return n
}

// A gap of several glues, a word space and the right skip at the end of a
// paragraph for one, writes one space glyph, not one per glue.
func TestOneSpaceGlyphPerGap(t *testing.T) {
	for name, build := range map[string]func(f *font.Font) []node.Node{
		"between words": func(f *font.Font) []node.Node {
			return append(append(glyphs(f, "b"), glueOf("4pt"), glueOf("0pt")), append(glyphs(f, "c"), colorSwitchNode())...)
		},
		"at the end of a line": func(f *font.Font) []node.Node {
			return append(glyphs(f, "b"), glueOf("4pt"), glueOf("0pt"), colorSwitchNode())
		},
	} {
		stream, fnt := renderList(t, build)
		if n := countGlyph(shownGlyphs(stream), gid(fnt, " ")); n != 1 {
			t.Errorf("%s: %d space glyphs, want 1:\n%s", name, n, stream)
		}
	}
	// The second glue still moves by its width.
	stream, fnt := renderList(t, func(f *font.Font) []node.Node {
		return append(append(glyphs(f, "b"), glueOf("4pt"), glueOf("3pt")), append(glyphs(f, "c"), colorSwitchNode())...)
	})
	nums := tjNumbers(t, stream)
	if len(nums) != 2 {
		t.Fatalf("TJ moves %v, want one per glue:\n%s", nums, stream)
	}
	drawn := drawnGlue(fnt, nums[0], 1) + float64(-nums[1])*fnt.Size.ToPT()/1000
	if drawn-7 > 0.012 || drawn-7 < -0.012 {
		t.Errorf("the PDF draws the gap %.3fpt wide, want 7pt:\n%s", drawn, stream)
	}
}

// renderWithColorFont renders the lines build returns, with Crimson Pro for
// text and colr-nospace.ttf, a COLRv0 font without a space glyph, for the
// color glyph U+E900.
func renderWithColorFont(t *testing.T, build func(text, color *font.Font) [][]node.Node) (string, *font.Font) {
	t.Helper()
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.CompressLevel = 0
	textFace, err := d.LoadFace("../../qa/fonts/upem/fonts/CrimsonPro-Regular.ttf", 0)
	if err != nil {
		t.Fatal(err)
	}
	colorFace, err := d.LoadFace("testdata/colr-nospace.ttf", 0)
	if err != nil {
		t.Fatal(err)
	}
	text := font.NewFont(textFace, bag.MustSP("12pt"))
	color := font.NewFont(colorFace, bag.MustSP("12pt"))
	p := d.NewPage()
	for i, line := range build(text, color) {
		var head node.Node
		for _, n := range line {
			head = node.InsertAfter(head, node.Tail(head), n)
		}
		p.OutputAt(bag.MustSP("1cm"), bag.MustSP("10cm")-bag.ScaledPoint(i)*bag.MustSP("1cm"), node.Vpack(node.Hpack(head)))
	}
	p.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	if err := d.PDFWriter.FinishAndClose(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	at := strings.Index(out, colorSwitch)
	if at < 0 {
		t.Fatalf("no color switch in the output:\n%s", out)
	}
	return out[strings.LastIndex(out[:at], "stream") : at+strings.Index(out[at:], "endstream")], text
}

// colorGlyphNode returns the color glyph U+E900 of the test font.
func colorGlyphNode(color *font.Font) *node.Glyph {
	g := node.NewGlyph()
	g.Font = color
	g.Codepoint = color.Face.Codepoint('\uE900')
	g.Components = "\uE900"
	g.Width = color.Size
	return g
}

// A word space after a color glyph writes its space glyph: the color glyph
// left the text object, and without the space glyph text read in content
// stream order runs the color glyph into the next word. The color font has
// no space glyph, so it comes from the text font, at the end of the color
// glyph.
func TestASpaceAfterAColorGlyph(t *testing.T) {
	var bWidth bag.ScaledPoint
	stream, text := renderWithColorFont(t, func(text, color *font.Font) [][]node.Node {
		b := glyphs(text, "b")
		bWidth = b[0].(*node.Glyph).Width
		return [][]node.Node{append(append(b, colorGlyphNode(color), glueOf("4pt")), append(glyphs(text, "c"), colorSwitchNode())...)}
	})
	got := shownGlyphs(stream)
	space := gid(text, " ")
	if n := countGlyph(got, space); n != 1 {
		t.Fatalf("%d space glyphs in %v, want 1:\n%s", n, got, stream)
	}
	emc := strings.LastIndex(stream, "EMC")
	at := strings.Index(stream, "<"+space+">")
	if at < emc {
		t.Errorf("the space glyph is inside the ActualText span of the color glyph:\n%s", stream)
	}
	if !strings.Contains(stream[emc:at], text.Face.InternalName()+" ") {
		t.Errorf("the space is not set in the text font:\n%s", stream)
	}
	xs := tmX(stream[emc:])
	want := (bag.MustSP("1cm") + bWidth + bag.MustSP("12pt")).String()
	if len(xs) == 0 || xs[0] != want {
		t.Errorf("the space starts at x %v, want %s:\n%s", xs, want, stream)
	}
}

// A color glyph right before a word, or at the end of the line without a
// glue, owes no space.
func TestNoSpaceAfterAColorGlyphWithoutGlue(t *testing.T) {
	stream, text := renderWithColorFont(t, func(text, color *font.Font) [][]node.Node {
		return [][]node.Node{append([]node.Node{colorGlyphNode(color)}, append(glyphs(text, "c"), colorSwitchNode())...)}
	})
	if n := countGlyph(shownGlyphs(stream), gid(text, " ")); n != 0 {
		t.Errorf("%d space glyphs, want none:\n%s", n, stream)
	}
}

// In a line of nothing but color glyphs the space between them comes from
// the font of the last space glyph the document wrote, here the line before.
func TestASpaceBetweenColorGlyphs(t *testing.T) {
	stream, text := renderWithColorFont(t, func(text, color *font.Font) [][]node.Node {
		return [][]node.Node{
			append(append(glyphs(text, "b"), glueOf("4pt")), glyphs(text, "c")...),
			{colorGlyphNode(color), glueOf("4pt"), colorGlyphNode(color), colorSwitchNode()},
		}
	})
	if n := countGlyph(shownGlyphs(stream), gid(text, " ")); n != 2 {
		t.Errorf("%d space glyphs, want one per line:\n%s", n, stream)
	}
}
