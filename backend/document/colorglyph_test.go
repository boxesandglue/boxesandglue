package document

import (
	"bytes"
	"strings"
	"testing"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

var (
	testPalette = []ot.BGRAColor{
		{Red: 255, Alpha: 255},
		{Green: 255, Alpha: 128},
		{Blue: 255, Alpha: 0},
	}
	testLayers = []ot.ColorLayer{
		{GlyphID: 1, ColorIndex: 0},
		{GlyphID: 2, ColorIndex: 1},
		{GlyphID: 3, ColorIndex: 2},
		{GlyphID: 4, ColorIndex: ot.ForegroundColorIndex},
	}
)

// colorGlyph writes the test layers and returns the content stream and the
// graphics states for the page. transparency is the one of the text color.
func colorGlyph(t *testing.T, format Format, transparency float64) (string, []pdf.ExtGState) {
	t.Helper()
	var doc, out bytes.Buffer
	d := NewDocument(&doc)
	d.Format = format
	face, err := d.LoadFace("../../qa/fonts/upem/fonts/CrimsonPro-Regular.ttf", 0)
	if err != nil {
		t.Fatal(err)
	}
	g := node.NewGlyph()
	g.Font = font.NewFont(face, bag.MustSP("12pt"))
	g.Codepoint = 5
	oc := &objectContext{
		s:                &out,
		textmode:         ScopeText,
		p:                &Page{document: d},
		textTransparency: transparency,
	}
	oc.text.font = g.Font
	oc.emitColorGlyph(g, 0, 0, testLayers, testPalette)
	if oc.textmode != ScopePage {
		t.Errorf("text mode %d after the glyph, want ScopePage", oc.textmode)
	}
	return out.String(), oc.pendingExtGStates
}

// TestColorGlyphAlpha: each layer of a color glyph has the alpha of its
// palette entry. A layer with alpha 0 is not written at all.
func TestColorGlyphAlpha(t *testing.T) {
	out, states := colorGlyph(t, Format{}, 0)
	if strings.Contains(out, "0003") {
		t.Errorf("the layer with alpha 0 is written:\n%s", out)
	}
	if !strings.Contains(out, "q 1 0 0 rg") {
		t.Errorf("the opaque layer has an alpha or no color:\n%s", out)
	}
	if !strings.Contains(out, "/GSca0_502 gs 0 1 0 rg") {
		t.Errorf("the alpha of the palette entry is missing:\n%s", out)
	}
	names := make(map[pdf.Name]bool)
	for _, gs := range states {
		names[gs.ResourceName()] = true
	}
	if !names["GSca0_502"] {
		t.Errorf("the page does not get the graphics state, got %v", names)
	}
}

// TestColorGlyphKeepsTextColor: the color of the text is unknown at the glyph,
// so the layers must not end by setting a color. A foreground layer writes no
// color and q and Q stay outside of the text objects, where they are illegal.
func TestColorGlyphKeepsTextColor(t *testing.T) {
	out, _ := colorGlyph(t, Format{}, 0)
	if q, Q := strings.Count(out, "q "), strings.Count(out, "Q\n"); q != 3 || Q != 3 {
		t.Errorf("%d q and %d Q, want 3 of each:\n%s", q, Q, out)
	}
	if strings.Contains(out, "0 g") {
		t.Errorf("the glyph sets a color for the text after it:\n%s", out)
	}
	end := strings.LastIndex(out, "Q\n")
	if end+2 != len(out) {
		t.Errorf("output after the last layer:\n%s", out[end:])
	}
	last := out[strings.LastIndex(out, "q "):]
	if strings.Contains(last, "rg") {
		t.Errorf("the foreground layer sets a color:\n%s", last)
	}
	for _, block := range strings.Split(out, "Q\n") {
		if block != "" && (strings.Index(block, "q ") > strings.Index(block, "BT") || !strings.HasSuffix(block, "ET\n")) {
			t.Errorf("q or Q inside a text object:\n%s", block)
		}
	}
}

// TestColorGlyphMultipliesTextAlpha: ca of a layer replaces the one of the text
// color, so it has to contain it.
func TestColorGlyphMultipliesTextAlpha(t *testing.T) {
	out, _ := colorGlyph(t, Format{}, 0.5)
	if !strings.Contains(out, "/GSca0_5 gs 1 0 0 rg") {
		t.Errorf("the opaque layer does not take the alpha of the text:\n%s", out)
	}
	if !strings.Contains(out, "/GSca0_251 gs 0 1 0 rg") {
		t.Errorf("the translucent layer does not multiply:\n%s", out)
	}
}

// TestColorGlyphOpaqueWithoutTransparency: PDF/X-3 forbids transparency, so
// the layers are opaque, and alpha 0 still paints nothing.
func TestColorGlyphOpaqueWithoutTransparency(t *testing.T) {
	out, states := colorGlyph(t, FormatPDFX3, 0)
	if strings.Contains(out, " gs ") || len(states) != 0 {
		t.Errorf("graphics states in a format without transparency:\n%s", out)
	}
	if strings.Contains(out, "0003") {
		t.Errorf("the layer with alpha 0 is written:\n%s", out)
	}
	if !strings.Contains(out, "q 0 1 0 rg") {
		t.Errorf("the translucent layer is missing:\n%s", out)
	}
}

// TestTextTransparencyFollowsTheColor: the start of a transparent color run
// tells the shipout its transparency, the stop takes it back.
func TestTextTransparencyFollowsTheColor(t *testing.T) {
	var doc, out bytes.Buffer
	d := NewDocument(&doc)
	oc := &objectContext{s: &out, textmode: ScopePage, p: &Page{document: d}}

	start := node.NewStartStop()
	start.Position = node.PDFOutputPage
	start.SetAttribute("transparency", 0.25)
	var seen float64
	probe := node.NewStartStop()
	probe.Position = node.PDFOutputPage
	probe.ShipoutCallback = func(node.Node) string {
		seen = oc.textTransparency
		return ""
	}
	stop := node.NewStartStop()
	stop.Position = node.PDFOutputPage
	stop.StartNode = start
	var head node.Node
	head = node.InsertAfter(head, head, start)
	node.InsertAfter(head, start, probe)
	node.InsertAfter(head, probe, stop)

	oc.outputHorizontalItems(0, 0, node.Hpack(head))
	if seen != 0.25 {
		t.Errorf("transparency %g in the run, want 0.25", seen)
	}
	if oc.textTransparency != 0 {
		t.Errorf("transparency %g after the run, want 0", oc.textTransparency)
	}
}
