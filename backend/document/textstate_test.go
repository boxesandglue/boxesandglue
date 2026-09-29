package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

const colorSwitch = "1 0 0 rg"

// textLine shapes "bc" into a line whose glyphs are raised by rise, with the
// font expansion expand. With withColor, a page level color switch sits
// between the two glyphs, the way a <span style="color: red"> ends up in
// the node list.
func textLine(fnt *font.Font, rise bag.ScaledPoint, expand int, withColor bool) *node.VList {
	var head, cur node.Node
	for i, a := range fnt.Shape("bc", nil, nil) {
		if i == 1 && withColor {
			col := node.NewStartStop()
			col.Position = node.PDFOutputPage
			col.ShipoutCallback = func(node.Node) string { return " " + colorSwitch + " " }
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
	return node.Vpack(hl)
}

// renderLines places each line as its own object on one page and returns
// the uncompressed page content stream.
func renderLines(t *testing.T, lines func(*font.Font) []*node.VList) string {
	t.Helper()
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.CompressLevel = 0
	face, err := d.LoadFace("../../qa/fonts/upem/fonts/CrimsonPro-Regular.ttf", 0)
	if err != nil {
		t.Fatal(err)
	}
	p := d.NewPage()
	for i, vl := range lines(font.NewFont(face, bag.MustSP("12pt"))) {
		p.OutputAt(bag.MustSP("1cm"), bag.MustSP("10cm")-bag.ScaledPoint(i)*bag.MustSP("1cm"), vl)
	}
	p.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	at := strings.Index(out, colorSwitch)
	if at < 0 {
		t.Fatalf("no color switch in the output:\n%s", out)
	}
	start := strings.LastIndex(out[:at], "stream")
	end := at + strings.Index(out[at:], "endstream")
	return out[start:end]
}

// textStateAfterColor returns the operands of Tz and Ts in effect for the
// glyph after the color switch. It follows the PDF rules: the text state
// starts at 100 Tz and 0 Ts, survives ET and BT, and only Q restores it.
func textStateAfterColor(stream string) (tz, ts string) {
	at := strings.Index(stream, colorSwitch)
	glyph := at + strings.Index(stream[at:], "<")
	type state struct{ tz, ts string }
	cur := state{"100", "0"}
	var saved []state
	var prev string
	for _, tok := range strings.Fields(stream[:glyph]) {
		switch tok {
		case "Tz":
			cur.tz = prev
		case "Ts":
			cur.ts = prev
		case "q":
			saved = append(saved, cur)
		case "Q":
			cur = saved[len(saved)-1]
			saved = saved[:len(saved)-1]
		}
		prev = tok
	}
	return cur.tz, cur.ts
}

// A color switch closes the text object. The glyph after it keeps its rise.
func TestRiseSurvivesColorSwitch(t *testing.T) {
	stream := renderLines(t, func(fnt *font.Font) []*node.VList {
		return []*node.VList{textLine(fnt, bag.MustSP("12pt"), 0, true)}
	})
	if _, ts := textStateAfterColor(stream); ts != "12" {
		t.Errorf("the glyph after the color switch has rise %s, want 12:\n%s", ts, stream)
	}
}

// Same for the horizontal scaling of font expansion.
func TestExpansionSurvivesColorSwitch(t *testing.T) {
	stream := renderLines(t, func(fnt *font.Font) []*node.VList {
		return []*node.VList{textLine(fnt, 0, 3, true)}
	})
	if tz, _ := textStateAfterColor(stream); tz != "103" {
		t.Errorf("the glyph after the color switch has Tz %s, want 103:\n%s", tz, stream)
	}
}

// All objects of a page share one content stream. The rise and expansion
// the first object leaves behind must not reach the glyphs of the next.
func TestTextStateDoesNotLeakIntoNextObject(t *testing.T) {
	stream := renderLines(t, func(fnt *font.Font) []*node.VList {
		return []*node.VList{
			textLine(fnt, bag.MustSP("12pt"), 3, false),
			textLine(fnt, 0, 0, true),
		}
	})
	if tz, ts := textStateAfterColor(stream); tz != "100" || ts != "0" {
		t.Errorf("the second object inherits Tz %s and Ts %s, want 100 and 0:\n%s", tz, ts, stream)
	}
}
