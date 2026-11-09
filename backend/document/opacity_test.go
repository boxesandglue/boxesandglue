package document

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// withOpacity gives vl the Opacity opacity.
func withOpacity(vl *node.VList, opacity float64) *node.VList {
	if vl.Attributes == nil {
		vl.Attributes = node.H{}
	}
	vl.Attributes[Opacity] = opacity
	return vl
}

// opacityPDF ships vl out at 2cm, 20cm on a page of a document in format and
// returns the uncompressed PDF.
func opacityPDF(t *testing.T, format Format, vl *node.VList) string {
	t.Helper()
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.Format = format
	d.SuppressInfo = true
	d.CompressLevel = 0
	if format.IsPDFUA() {
		d.DefaultLanguageTag = "en"
		d.Title = "Opacity"
		d.RootStructureElement = &StructureElement{Role: "Document"}
	}
	collectTags(d.RootStructureElement, vl)
	p := d.NewPage()
	p.OutputAt(bag.MustSP("2cm"), bag.MustSP("20cm"), vl)
	p.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// collectTags makes the structure elements in the "tag" attributes of vl and
// its descendants children of root.
func collectTags(root *StructureElement, vl *node.VList) {
	if root == nil {
		return
	}
	var walk func(n node.Node)
	walk = func(n node.Node) {
		for ; n != nil; n = n.Next() {
			switch v := n.(type) {
			case *node.VList:
				if se, ok := v.Attributes["tag"].(*StructureElement); ok {
					root.AddChild(se)
				}
				walk(v.List)
			case *node.HList:
				walk(v.List)
			}
		}
	}
	walk(vl)
}

// objectByNumber returns object num of the uncompressed PDF out, from "obj"
// to "endobj".
func objectByNumber(t *testing.T, out, num string) string {
	t.Helper()
	at := strings.Index(out, "\n"+num+" 0 obj")
	if at < 0 {
		t.Fatalf("no object %s in\n%s", num, out)
	}
	end := at + strings.Index(out[at:], "endobj")
	return out[at:end]
}

// pageContent returns the content stream of the first page of out.
func pageContent(t *testing.T, out string) string {
	t.Helper()
	m := regexp.MustCompile(`/Contents (\d+) 0 R`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no page contents in\n%s", out)
	}
	return objectByNumber(t, out, m[1])
}

// hasFill reports whether stream has a fill of width wd, see fillPos.
func hasFill(stream, wd string) bool {
	re := regexp.MustCompile(fmt.Sprintf(`cm q 0 0 %s \S+ re f Q`, regexp.QuoteMeta(bag.MustSP(wd).String())))
	return re.MatchString(stream)
}

// pdfObject returns the object of the uncompressed PDF out that contains
// marker, from "obj" to "endobj".
func pdfObject(t *testing.T, out, marker string) string {
	t.Helper()
	at := strings.Index(out, marker)
	if at < 0 {
		t.Fatalf("no %q in\n%s", marker, out)
	}
	start := strings.LastIndex(out[:at], " obj")
	end := at + strings.Index(out[at:], "endobj")
	return out[start:end]
}

// A VList with an Opacity is painted from a form with a transparency group,
// which the page paints with the opacity as the alpha constant of a graphics
// state. Its siblings stay on the page.
func TestOpacityGroup(t *testing.T) {
	out := opacityPDF(t, FormatPDF, vbox(ruleOfWidth("1cm"), withOpacity(ruleOfWidth("2cm"), 0.5), ruleOfWidth("3cm")))
	page := pageContent(t, out)
	fillPos(t, page, "1cm")
	fillPos(t, page, "3cm")
	if hasFill(page, "2cm") {
		t.Errorf("the 2cm fill of the group is on the page:\n%s", page)
	}
	if !strings.Contains(page, "q /GSca0_5CA0_5 gs /FmOp1 Do Q") {
		t.Errorf("the page does not paint the form with alpha 0.5:\n%s", page)
	}
	form := pdfObject(t, out, "/Subtype /Form")
	fillPos(t, form, "2cm")
	if !strings.Contains(form, "/S /Transparency") {
		t.Errorf("the form has no transparency group:\n%s", form)
	}
	// Not isolated, so that overprinting reaches the plates below.
	if strings.Contains(form, "/I true") {
		t.Errorf("the group is isolated:\n%s", form)
	}
	if !regexp.MustCompile(`/FmOp1 \d+ 0 R`).MatchString(out) {
		t.Errorf("the page resources do not list the form")
	}
	gs := pdfObject(t, out, "/ca 0.5")
	if !strings.Contains(gs, "/CA 0.5") {
		t.Errorf("the graphics state has no stroking alpha:\n%s", gs)
	}
}

// The nodes marked PaintLast inside the group are painted at its end, in the
// form, not at the end of the page without the opacity.
func TestOpacityPaintLast(t *testing.T) {
	group := withOpacity(vbox(paintLast(ruleOfWidth("1cm")), ruleOfWidth("2cm")), 0.5)
	out := opacityPDF(t, FormatPDF, vbox(group, ruleOfWidth("3cm")))
	page := pageContent(t, out)
	if hasFill(page, "1cm") {
		t.Errorf("the marked 1cm fill of the group is on the page:\n%s", page)
	}
	form := pdfObject(t, out, "/Subtype /Form")
	marked, _ := fillPos(t, form, "1cm")
	other, _ := fillPos(t, form, "2cm")
	if marked < other {
		t.Errorf("the marked fill is not painted last in the form:\n%s", form)
	}
}

// A group in a group is a form in the form: the page refers only to the outer
// one.
func TestOpacityNested(t *testing.T) {
	inner := withOpacity(ruleOfWidth("2cm"), 0.5)
	outer := withOpacity(vbox(ruleOfWidth("1cm"), inner), 0.25)
	out := opacityPDF(t, FormatPDF, outer)
	page := pageContent(t, out)
	if got := strings.Count(page, " Do Q"); got != 1 {
		t.Fatalf("the page paints %d forms, want 1:\n%s", got, page)
	}
	if !strings.Contains(page, "/GSca0_25CA0_25 gs /FmOp2 Do Q") {
		t.Errorf("the page does not paint the outer form:\n%s", page)
	}
	outerForm := pdfObject(t, out, "/FmOp1 Do Q")
	fillPos(t, outerForm, "1cm")
	if !strings.Contains(outerForm, "/GSca0_5CA0_5 gs /FmOp1 Do Q") {
		t.Errorf("the outer form does not paint the inner one:\n%s", outerForm)
	}
}

// The marked content of a group lives in the content stream of its form, with
// MCIDs of its own, which the structure element reaches with /Stm and the
// ParentTree through the /StructParents of the form. A sequence open around
// the group closes on the page and continues in the form.
func TestOpacityMarkedContent(t *testing.T) {
	t.Run("tagged group", func(t *testing.T) {
		p := &StructureElement{Role: "P"}
		group := withOpacity(ruleOfWidth("2cm"), 0.5)
		group.Attributes["tag"] = p
		out := opacityPDF(t, FormatPDFUA, vbox(group))
		page := pageContent(t, out)
		if strings.Contains(page, "MCID") {
			t.Errorf("marked content on the page:\n%s", page)
		}
		form := pdfObject(t, out, "/Subtype /Form")
		if !strings.Contains(form, "/P <</MCID 0>> BDC") {
			t.Errorf("no sequence of the P in the form:\n%s", form)
		}
		if !strings.Contains(form, "/StructParents 0") {
			t.Errorf("the form has no /StructParents:\n%s", form)
		}
		if !regexp.MustCompile(`/Stm \d+ 0 R /MCID 0`).MatchString(out) {
			t.Errorf("the P does not refer to the form")
		}
		if !regexp.MustCompile(`/Nums \[ 0 \[\d+ 0 R\]`).MatchString(out) {
			t.Errorf("the ParentTree has no entry for the form")
		}
	})
	t.Run("group inside an element", func(t *testing.T) {
		p := &StructureElement{Role: "P"}
		group := withOpacity(ruleOfWidth("2cm"), 0.5)
		outer := vbox(ruleOfWidth("1cm"), group, ruleOfWidth("3cm"))
		outer.Attributes = node.H{"tag": p}
		out := opacityPDF(t, FormatPDFUA, vbox(outer))
		page := pageContent(t, out)
		want := regexp.MustCompile(`(?s)/P <</MCID 0>> BDC.*EMC.*Do Q.*/P <</MCID 1>> BDC`)
		if !want.MatchString(page) {
			t.Errorf("the sequence of the P does not close before the form and continue after it:\n%s", page)
		}
		form := pdfObject(t, out, "/Subtype /Form")
		if !strings.Contains(form, "/P <</MCID 0>> BDC") {
			t.Errorf("the P does not continue in the form:\n%s", form)
		}
		if len(p.mcids) != 3 {
			t.Errorf("the P has %d sequences, want 3", len(p.mcids))
		}
	})
}

// PDF/X-3 forbids transparency: the group is painted opaque on the page.
func TestOpacityWithoutTransparency(t *testing.T) {
	out := opacityPDF(t, FormatPDFX3, vbox(withOpacity(ruleOfWidth("2cm"), 0.5)))
	page := pageContent(t, out)
	fillPos(t, page, "2cm")
	if strings.Contains(out, "/Subtype /Form") {
		t.Errorf("PDF/X-3 with a form")
	}
}
