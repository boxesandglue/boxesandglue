package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// taggedDoc returns a document in format with a Document root element that
// holds the structure elements children.
func taggedDoc(buf *bytes.Buffer, format Format, children ...*StructureElement) *PDFDocument {
	d := NewDocument(buf)
	d.Format = format
	d.SuppressInfo = true
	d.DefaultLanguageTag = "en"
	d.Title = "Figures"
	root := &StructureElement{Role: "Document"}
	for _, c := range children {
		root.AddChild(c)
	}
	d.RootStructureElement = root
	return d
}

// wantBBox checks that se has the bounding box llx lly urx ury, in cm.
func wantBBox(t *testing.T, se *StructureElement, llx, lly, urx, ury string) {
	t.Helper()
	if !se.HasBBox {
		t.Fatalf("%s has no BBox", se.Role)
	}
	want := [4]float64{bag.MustSP(llx).ToPT(), bag.MustSP(lly).ToPT(), bag.MustSP(urx).ToPT(), bag.MustSP(ury).ToPT()}
	if se.BBox != want {
		t.Errorf("%s BBox = %v, want %v", se.Role, se.BBox, want)
	}
}

// The BBox of a Figure is where its box is drawn: a 1cm square output with its
// top left corner at 2cm, 20cm spans 19cm to 20cm, not the distance from the
// top of the page.
func TestFigureBBox(t *testing.T) {
	for _, tc := range []struct {
		name   string
		format Format
		role   string
	}{
		{"UA-1", FormatPDFUA, "Figure"},
		{"UA-2", FormatPDFUA2, "figure"},
	} {
		t.Run(tc.name+" page", func(t *testing.T) {
			var buf bytes.Buffer
			fig := &StructureElement{Role: tc.role, Alt: "a square"}
			d := taggedDoc(&buf, tc.format, fig)
			box := ruleOfWidth("1cm")
			box.Attributes = node.H{"tag": fig}
			pageStream(t, d, &buf, box)
			wantBBox(t, fig, "2cm", "19cm", "3cm", "20cm")
		})
		t.Run(tc.name+" in a VList", func(t *testing.T) {
			var buf bytes.Buffer
			fig := &StructureElement{Role: tc.role, Alt: "a square"}
			d := taggedDoc(&buf, tc.format, fig)
			box := ruleOfWidth("1cm")
			box.Attributes = node.H{"tag": fig}
			pageStream(t, d, &buf, vbox(ruleOfWidth("1cm"), box))
			wantBBox(t, fig, "2cm", "18cm", "3cm", "19cm")
		})
		t.Run(tc.name+" in an HList", func(t *testing.T) {
			var buf bytes.Buffer
			fig := &StructureElement{Role: tc.role, Alt: "a square"}
			d := taggedDoc(&buf, tc.format, fig)
			box := ruleOfWidth("1cm")
			box.Attributes = node.H{"tag": fig}
			pageStream(t, d, &buf, hbox(bareRule("1cm"), box))
			wantBBox(t, fig, "3cm", "19cm", "4cm", "20cm")
		})
	}
}

// A box with a Figure of its own in a line of a paragraph interrupts the
// paragraph's marked-content sequence: a sequence with an MCID must not hold
// another. The paragraph continues in a new sequence after the box.
func TestInlineFigureInterruptsRun(t *testing.T) {
	var buf bytes.Buffer
	para := &StructureElement{Role: "P"}
	fig := &StructureElement{Role: "Figure", Alt: "a square"}
	para.AddChild(fig)
	d := taggedDoc(&buf, FormatPDFUA, para)
	box := ruleOfWidth("1cm")
	box.Attributes = node.H{"tag": fig}
	p := hbox(bareRule("2cm"), box, bareRule("3cm"))
	p.Attributes = node.H{"tag": para}
	stream := pageStream(t, d, &buf, p)

	open := strings.Index(stream, "/P <</MCID 0>> BDC")
	figAt := strings.Index(stream, "/Figure <</MCID 1>> BDC")
	resume := strings.Index(stream, "/P <</MCID 2>> BDC")
	if open < 0 || figAt < 0 || resume < 0 || !(open < figAt && figAt < resume) {
		t.Fatalf("want P, Figure, P sequences in order, got\n%s", stream)
	}
	if strings.Count(stream[open:figAt], "EMC") != 1 || strings.Count(stream[figAt:resume], "EMC") != 1 {
		t.Errorf("want the P sequence closed before the Figure and the Figure closed before the P resumes, got\n%s", stream)
	}
	if _, at := fillPos(t, stream, "1cm"); at == "" || !strings.Contains(stream[figAt:resume], at) {
		t.Errorf("want the box drawn in the Figure sequence, got\n%s", stream)
	}
	if len(para.mcids) != 2 || len(fig.mcids) != 1 {
		t.Errorf("P has %d sequences, Figure %d; want 2 and 1", len(para.mcids), len(fig.mcids))
	}
	wantBBox(t, fig, "4cm", "19cm", "5cm", "20cm")
}
