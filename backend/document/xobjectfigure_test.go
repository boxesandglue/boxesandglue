package document

import (
	"regexp"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// inlinePDFFigures ships out a paragraph whose line holds a rule, an imported
// PDF page as the Figure "first", a rule, the same page as the Figure
// "second" and a rule, under PDF/UA. The first image gets opacity, if it is
// not 1. It returns the uncompressed PDF.
func inlinePDFFigures(t *testing.T, opacity float64) string {
	t.Helper()
	return opacityPDFWith(t, FormatPDFUA, func(d *PDFDocument) *node.VList {
		para := &StructureElement{Role: "P"}
		first := &StructureElement{Role: "Figure", Alt: "first"}
		second := &StructureElement{Role: "Figure", Alt: "second"}
		para.AddChild(first)
		para.AddChild(second)

		img1 := pdfImage(t, d, 20, 20)
		img1.Attributes = node.H{"tag": first}
		if opacity != 1 {
			imageWithOpacity(img1, opacity)
		}
		img2 := d.CreateImageNodeFromImagefile(img1.ImageFile, 1, "/MediaBox")
		img2.Attributes = node.H{"tag": second}

		var head node.Node
		for _, n := range []node.Node{inlineRule(), img1, inlineRule(), img2, inlineRule()} {
			head = node.InsertAfter(head, node.Tail(head), n)
		}
		vl := node.Vpack(node.Hpack(head))
		vl.Attributes = node.H{"tag": para}
		return vl
	})
}

// inlineRule is a 1cm wide and high rule for a line.
func inlineRule() *node.Rule {
	r := node.NewRule()
	r.Width = bag.MustSP("1cm")
	r.Height = bag.MustSP("1cm")
	return r
}

// An imported PDF page in a line of text is the content of its Figure via an
// OBJR, as a PDF image that is a block of its own: the /Do interrupts the
// sequence of the paragraph unmarked, the XObject gets the /StructParent, and
// the Figure its BBox (issue #107). The same page drawn a second time cannot
// belong to a second element, since an XObject has one /StructParent, and
// takes a marked-content sequence.
func TestInlinePDFImageObjRef(t *testing.T) {
	out := inlinePDFFigures(t, 1)
	page := pageContent(t, out)
	unmarked := regexp.MustCompile(`EMC\nq \S+ 0 0 \S+ \S+ \S+ cm /ImgBag1 Do Q\n/P <</MCID 1>> BDC`)
	if !unmarked.MatchString(page) {
		t.Errorf("the first image does not interrupt the paragraph unmarked:\n%s", page)
	}
	if !regexp.MustCompile(`/Figure <</MCID 2>> BDC\nq \S+ 0 0 \S+ \S+ \S+ cm /ImgBag1 Do Q\nEMC`).MatchString(page) {
		t.Errorf("the second use of the page has no sequence of its own:\n%s", page)
	}
	checkObjRef(t, out)
}

// With an opacity the image is painted from the form of a group, whose /Do
// interrupts the paragraph. In the form, the /Do of the page is unmarked, and
// the form needs no /StructParents.
func TestInlinePDFImageObjRefOpacity(t *testing.T) {
	out := inlinePDFFigures(t, 0.5)
	page := pageContent(t, out)
	if !regexp.MustCompile(`EMC\n+q /GSca0_5CA0_5 gs /FmOp1 Do Q\n/P <</MCID 1>> BDC`).MatchString(page) {
		t.Errorf("the group does not interrupt the paragraph unmarked:\n%s", page)
	}
	// The object of the form holds its content stream.
	form := pdfObject(t, out, "/S /Transparency")
	if strings.Contains(form, "/StructParents") {
		t.Errorf("the form of the group has /StructParents:\n%s", form)
	}
	if !strings.Contains(form, "/ImgBag1 Do") || strings.Contains(form, "BDC") {
		t.Errorf("the page in the form is not drawn unmarked:\n%s", form)
	}
	checkObjRef(t, out)
}

// checkObjRef checks that the Figure "first" refers to the XObject of the
// page with an OBJR and has a BBox, and that the XObject has the only
// /StructParent.
func checkObjRef(t *testing.T, out string) {
	t.Helper()
	fig := pdfObject(t, out, "/Alt (first)")
	m := regexp.MustCompile(`/Obj (\d+) 0 R`).FindStringSubmatch(fig)
	if m == nil || !strings.Contains(fig, "/Type /OBJR") {
		t.Fatalf("the Figure has no OBJR:\n%s", fig)
	}
	if !strings.Contains(fig, "/BBox") {
		t.Errorf("the Figure has no BBox:\n%s", fig)
	}
	if xobj := objectByNumber(t, out, m[1]); !strings.Contains(xobj, "/StructParent 0") {
		t.Errorf("the XObject has no /StructParent 0:\n%s", xobj)
	}
	if n := strings.Count(out, "/StructParent "); n != 1 {
		t.Errorf("%d objects with a /StructParent, want 1", n)
	}
	if second := pdfObject(t, out, "/Alt (second)"); strings.Contains(second, "/OBJR") {
		t.Errorf("the second use refers to the XObject too:\n%s", second)
	}
}
