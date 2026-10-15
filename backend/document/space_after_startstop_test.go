package document

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// shownGlyphs returns the glyph ids the stream shows, in content-stream
// order, as four digit hex numbers.
func shownGlyphs(stream string) []string {
	var ret []string
	for _, hex := range hexString.FindAllString(stream, -1) {
		hex = strings.Trim(hex, "<>")
		for i := 0; i+4 <= len(hex); i += 4 {
			ret = append(ret, hex[i:i+4])
		}
	}
	return ret
}

// gid returns the glyph id of the first glyph of text in fnt as shownGlyphs
// writes it.
func gid(fnt *font.Font, text string) string {
	return fmt.Sprintf("%04x", fnt.Shape(text, nil, nil)[0].Codepoint)
}

// glueOf returns a glue of width wd.
func glueOf(wd string) *node.Glue {
	g := node.NewGlue()
	g.Width = bag.MustSP(wd)
	return g
}

// renderList shows the nodes build returns in one line and returns the
// stream and the font.
func renderList(t *testing.T, build func(fnt *font.Font) []node.Node) (string, *font.Font) {
	t.Helper()
	var fnt *font.Font
	stream := renderLines(t, func(f *font.Font) []*node.VList {
		fnt = f
		var head node.Node
		for _, n := range build(f) {
			head = node.InsertAfter(head, node.Tail(head), n)
		}
		return []*node.VList{node.Vpack(node.Hpack(head))}
	})
	return stream, fnt
}

// glyphs returns the glyph nodes for text.
func glyphs(fnt *font.Font, text string) []node.Node {
	head, _ := glyphRun(nil, nil, fnt, text)
	var ret []node.Node
	for n := head; n != nil; n = n.Next() {
		ret = append(ret, n)
	}
	return ret
}

// A word space after a color change writes its space glyph as one inside
// the TJ array does, so that text read in content-stream order keeps the
// word boundary (boxesandglue#93). The space starts a TJ array at its own
// position and the next word continues in it at the glue's width.
func TestASpaceAfterAColorChange(t *testing.T) {
	var advance bag.ScaledPoint
	stream, fnt := renderList(t, func(f *font.Font) []node.Node {
		b := glyphs(f, "b")
		advance = b[0].(*node.Glyph).Width
		return append(append(b, colorSwitchNode(), glueOf("4pt")), glyphs(f, "c")...)
	})
	want := []string{gid(fnt, "b"), gid(fnt, " "), gid(fnt, "c")}
	if got := shownGlyphs(stream); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("glyphs %v, want %v (b, space, c):\n%s", got, want, stream)
	}
	xs := tmX(stream)
	if len(xs) != 2 {
		t.Fatalf("want a Tm for b and one for the space, got %v:\n%s", xs, stream)
	}
	if want := (bag.MustSP("1cm") + advance).String(); xs[1] != want {
		t.Errorf("the space starts at x %s, want %s:\n%s", xs[1], want, stream)
	}
	nums := tjNumbers(t, stream[strings.Index(stream, colorSwitch):])
	if len(nums) != 1 {
		t.Fatalf("TJ moves %v after the color change, want one for the glue:\n%s", nums, stream)
	}
	if drawn := drawnGlue(fnt, nums[0], 1); drawn-4 > 0.006 || drawn-4 < -0.006 {
		t.Errorf("the PDF draws the glue %.3fpt wide, want 4pt:\n%s", drawn, stream)
	}
}

// A line that ends in a colored word ends with the space glyph of its
// right skip, as a line in the text color does.
func TestASpaceAtTheEndOfALineAfterAColorChange(t *testing.T) {
	stream, fnt := renderList(t, func(f *font.Font) []node.Node {
		return append(glyphs(f, "b"), colorSwitchNode(), glueOf("0pt"))
	})
	want := []string{gid(fnt, "b"), gid(fnt, " ")}
	if got := shownGlyphs(stream); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("glyphs %v, want %v (b, space):\n%s", got, want, stream)
	}
}

// Only a color change after text owes a space: one before the first word
// of a line, or after a glue, does not.
func TestNoSpaceAfterAColorChangeWithoutText(t *testing.T) {
	for name, build := range map[string]func(f *font.Font) []node.Node{
		"line start": func(f *font.Font) []node.Node {
			return append([]node.Node{colorSwitchNode(), glueOf("4pt")}, glyphs(f, "c")...)
		},
		"after a glue": func(f *font.Font) []node.Node {
			return append(append(glyphs(f, "b"), glueOf("4pt"), colorSwitchNode(), glueOf("4pt")), glyphs(f, "c")...)
		},
	} {
		stream, fnt := renderList(t, build)
		got := shownGlyphs(stream)
		n := strings.Count(fmt.Sprint(got), gid(fnt, " "))
		if want := strings.Count(name, "glue"); n != want {
			t.Errorf("%s: %d space glyphs in %v, want %d:\n%s", name, n, got, want, stream)
		}
	}
}

// In tagged PDF the space a color change owes stays in the marked-content
// sequence of the paragraph: no glyph outside of one, as PDF/UA requires.
func TestTheOwedSpaceStaysInItsMarkedContent(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.Format = FormatPDFUA
	d.SuppressInfo = true
	d.DefaultLanguageTag = "en"
	d.Title = "Space after a color change"
	root := &StructureElement{Role: "Document"}
	para := &StructureElement{Role: "P"}
	root.AddChild(para)
	d.RootStructureElement = root
	face, err := d.LoadFace("../../qa/fonts/upem/fonts/CrimsonPro-Regular.ttf", 0)
	if err != nil {
		t.Fatal(err)
	}
	fnt := font.NewFont(face, bag.MustSP("12pt"))
	nodes := append(append(glyphs(fnt, "b"), colorSwitchNode(), glueOf("4pt")), glyphs(fnt, "c")...)
	line := hbox(nodes...)
	line.Attributes = node.H{"tag": para}
	d.CompressLevel = 0
	p := d.NewPage()
	p.OutputAt(bag.MustSP("2cm"), bag.MustSP("20cm"), line)
	p.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	if err := d.PDFWriter.FinishAndClose(); err != nil {
		t.Fatalf("FinishAndClose: %v", err)
	}
	out := buf.String()
	at := strings.Index(out, " BDC")
	if at < 0 {
		t.Fatalf("no marked content in\n%s", out)
	}
	stream := out[strings.LastIndex(out[:at], "stream") : at+strings.Index(out[at:], "endstream")]
	if got := shownGlyphs(stream); len(got) != 3 || got[1] != gid(fnt, " ") {
		t.Fatalf("glyphs %v, want b, space, c:\n%s", got, stream)
	}
	bdc := strings.Index(stream, "/P <</MCID")
	emc := strings.LastIndex(stream, "EMC")
	if strings.Count(stream, "BDC") != 1 || bdc < 0 || emc < bdc {
		t.Fatalf("want one marked-content sequence for P:\n%s", stream)
	}
	if first, last := hexString.FindStringIndex(stream), hexString.FindAllStringIndex(stream, -1); first[0] < bdc || last[len(last)-1][1] > emc {
		t.Errorf("a glyph lies outside the marked-content sequence of P:\n%s", stream)
	}
}
