package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// renderColorSwitch writes "bc" with a page level color switch between the
// two glyphs, the way a <span style="color: red"> ends up in the node list.
// Both glyphs are raised by rise, and the line carries the font expansion
// expand. It returns the content stream after the color operator.
func renderColorSwitch(t *testing.T, rise bag.ScaledPoint, expand int) string {
	t.Helper()
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.CompressLevel = 0
	face, err := d.LoadFace("../../qa/fonts/upem/fonts/CrimsonPro-Regular.ttf", 0)
	if err != nil {
		t.Fatal(err)
	}
	fnt := font.NewFont(face, bag.MustSP("12pt"))
	var head, cur node.Node
	for i, a := range fnt.Shape("bc", nil, nil) {
		if i == 1 {
			col := node.NewStartStop()
			col.Position = node.PDFOutputPage
			col.ShipoutCallback = func(node.Node) string { return " 1 0 0 rg " }
			head = node.InsertAfter(head, cur, col)
			cur = col
		}
		g := node.NewGlyph()
		g.Font = fnt
		g.Codepoint = a.Codepoint
		g.Components = a.Components
		g.Width = a.Advance
		g.YOffset = rise
		head = node.InsertAfter(head, cur, g)
		cur = g
	}
	hl := node.Hpack(head)
	if expand != 0 {
		hl.Attributes = node.H{"expand": expand}
	}
	p := d.NewPage()
	p.OutputAt(bag.MustSP("1cm"), bag.MustSP("10cm"), node.Vpack(hl))
	p.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	at := strings.Index(out, "1 0 0 rg")
	if at < 0 {
		t.Fatalf("no color switch in the output:\n%s", out)
	}
	return out[at:]
}

// A color switch closes the text object, and the next BT resets the text
// rise. The glyph after the switch must get its rise back.
func TestRiseSurvivesColorSwitch(t *testing.T) {
	after := renderColorSwitch(t, bag.MustSP("12pt"), 0)
	tj := strings.Index(after, "TJ")
	if !strings.Contains(after[:tj], "12 Ts") {
		t.Errorf("the glyph after the color switch lost its rise:\n%s", after[:tj])
	}
}

// Same for the horizontal scaling of font expansion.
func TestExpansionSurvivesColorSwitch(t *testing.T) {
	after := renderColorSwitch(t, 0, 3)
	tj := strings.Index(after, "TJ")
	if !strings.Contains(after[:tj], "103 Tz") {
		t.Errorf("the glyph after the color switch lost its expansion:\n%s", after[:tj])
	}
}
