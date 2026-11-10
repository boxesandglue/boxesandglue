package document

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/node"
)

// emptySequence matches a marked-content sequence with an MCID and nothing in
// it.
var emptySequence = regexp.MustCompile(`<</MCID \d+>> BDC\s*EMC`)

// figureBox is a 1cm box with the Figure fig.
func figureBox(fig *StructureElement) *node.VList {
	box := ruleOfWidth("1cm")
	box.Attributes = node.H{"tag": fig}
	return box
}

// A paragraph whose line starts and ends with a box with a Figure of its own
// opens its sequence only for the content between them, and a paragraph with
// nothing but such a box has no sequence at all (issue #106). Before, the
// paragraph opened one before the first box and after the last, with nothing
// in them.
func TestInlineFigureAtLineEdges(t *testing.T) {
	var buf bytes.Buffer
	para := &StructureElement{Role: "P"}
	first := &StructureElement{Role: "Figure", Alt: "first"}
	last := &StructureElement{Role: "Figure", Alt: "last"}
	para.AddChild(first)
	para.AddChild(last)
	only := &StructureElement{Role: "P"}
	alone := &StructureElement{Role: "Figure", Alt: "alone"}
	only.AddChild(alone)
	d := taggedDoc(&buf, FormatPDFUA, para, only)

	p := hbox(figureBox(first), bareRule("2cm"), figureBox(last))
	p.Attributes = node.H{"tag": para}
	q := hbox(figureBox(alone))
	q.Attributes = node.H{"tag": only}
	stream := pageStream(t, d, &buf, vbox(p, q))

	if m := emptySequence.FindString(stream); m != "" {
		t.Errorf("empty sequence %q in\n%s", m, stream)
	}
	if len(para.mcids) != 1 || len(only.mcids) != 0 {
		t.Errorf("the paragraphs have %d and %d sequences, want 1 and 0", len(para.mcids), len(only.mcids))
	}
	for _, fig := range []*StructureElement{first, last, alone} {
		if len(fig.mcids) != 1 {
			t.Errorf("Figure %q has %d sequences, want 1", fig.Alt, len(fig.mcids))
		}
	}
	firstAt := strings.Index(stream, "/Figure <</MCID 0>> BDC")
	paraAt := strings.Index(stream, "/P <</MCID 1>> BDC")
	lastAt := strings.Index(stream, "/Figure <</MCID 2>> BDC")
	if firstAt < 0 || paraAt < 0 || lastAt < 0 || !(firstAt < paraAt && paraAt < lastAt) {
		t.Fatalf("want Figure, P, Figure sequences in order, got\n%s", stream)
	}
	if _, at := fillPos(t, stream, "2cm"); at == "" || !strings.Contains(stream[paraAt:lastAt], at) {
		t.Errorf("want the rule drawn in the P sequence, got\n%s", stream)
	}
}

// A group with an opacity at the end of a line interrupts the paragraph on
// the page and continues it in its form. Where no content of the paragraph
// follows, in the form before a Figure and on the page after the group, no
// sequence opens.
func TestOpacityGroupAtLineEnd(t *testing.T) {
	var para, fig *StructureElement
	out := opacityPDFWith(t, FormatPDFUA, func(d *PDFDocument) *node.VList {
		para = &StructureElement{Role: "P"}
		fig = &StructureElement{Role: "Figure", Alt: "faded"}
		para.AddChild(fig)
		group := withOpacity(vbox(figureBox(fig)), 0.5)
		p := hbox(bareRule("2cm"), group)
		p.Attributes = node.H{"tag": para}
		return p
	})
	if m := emptySequence.FindString(out); m != "" {
		t.Errorf("empty sequence %q in\n%s", m, out)
	}
	if len(para.mcids) != 1 || len(fig.mcids) != 1 {
		t.Errorf("P has %d sequences, Figure %d; want 1 and 1", len(para.mcids), len(fig.mcids))
	}
	if !strings.Contains(pdfObject(t, out, "/S /Transparency"), "/Figure <</MCID 0>> BDC") {
		t.Errorf("the Figure has no sequence in the form of the group:\n%s", out)
	}
}
