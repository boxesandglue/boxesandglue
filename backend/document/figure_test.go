package document

import (
	"bytes"
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
