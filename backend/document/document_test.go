package document

import (
	"bytes"
	"strings"
	"testing"

	pdf "github.com/boxesandglue/baseline-pdf"
)

func TestFormatPDFVersionMapping(t *testing.T) {
	cases := []struct {
		format Format
		want   pdf.Version
	}{
		{FormatPDF, pdf.Version17},
		{FormatPDFA3b, pdf.Version17},
		{FormatPDFX3, pdf.Version17},
		{FormatPDFX4, pdf.Version17},
		{FormatPDFUA, pdf.Version17},
		{FormatPDFUA2, pdf.Version20},
	}
	for _, c := range cases {
		if v := c.format.pdfVersion(); v != c.want {
			t.Errorf("format %v: got version %v, want %v", c.format, v, c.want)
		}
	}
}

func TestFormatIsPDFUA(t *testing.T) {
	cases := []struct {
		format Format
		want   bool
	}{
		{FormatPDF, false},
		{FormatPDFA3b, false},
		{FormatPDFX3, false},
		{FormatPDFX4, false},
		{FormatPDFUA, true},
		{FormatPDFUA2, true},
	}
	for _, c := range cases {
		if got := c.format.isPDFUA(); got != c.want {
			t.Errorf("format %v: isPDFUA() = %v, want %v", c.format, got, c.want)
		}
	}
}

func TestDeclareNamespaceIsIdempotent(t *testing.T) {
	d := NewDocument(&bytes.Buffer{})
	obj1 := d.DeclareNamespace(NamespaceHTML5)
	obj2 := d.DeclareNamespace(NamespaceHTML5)
	if obj1 != obj2 {
		t.Errorf("declareNamespace not idempotent: returned different objects for same URI")
	}
	obj3 := d.DeclareNamespace(NamespacePDF20SSN)
	if obj1 == obj3 {
		t.Errorf("declareNamespace conflated distinct URIs")
	}
	if len(d.namespaceObjs) != 2 {
		t.Errorf("expected 2 namespace objects, got %d", len(d.namespaceObjs))
	}
}

func TestUA1EmitsPdfuaidPartAndRev(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.Format = FormatPDFUA
	d.Title = "UA-1 smoke test"
	d.DefaultLanguageTag = "en"
	d.SuppressInfo = true
	root := &StructureElement{Role: "Document"}
	d.RootStructureElement = root
	page := d.NewPage()
	page.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := d.PDFWriter.FinishAndClose(); err != nil {
		t.Fatalf("FinishAndClose: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "pdfuaid:part>1") {
		t.Errorf("XMP pdfuaid:part not set to 1")
	}
	if !strings.Contains(out, "pdfuaid:rev>2014") {
		t.Errorf("XMP pdfuaid:rev (ISO 14289-1 publication year) not emitted as four-digit year")
	}
}

func TestUA2EmitsNamespacesAndHTML5Roles(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.Format = FormatPDFUA2
	d.Title = "UA-2 smoke test"
	d.DefaultLanguageTag = "en"
	d.SuppressInfo = true
	// Build a minimal structure tree: Document → p (HTML5)
	root := &StructureElement{Role: "Document", NS: NamespacePDF20SSN}
	para := &StructureElement{Role: "p", NS: NamespaceHTML5}
	root.AddChild(para)
	d.RootStructureElement = root
	// Add a page so Finish has something to anchor to
	page := d.NewPage()
	page.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := d.PDFWriter.FinishAndClose(); err != nil {
		t.Fatalf("FinishAndClose: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "%PDF-2.0\n") {
		t.Errorf("PDF header is not 2.0: %q", out[:min(20, len(out))])
	}
	if !strings.Contains(out, "/Marked true") {
		t.Errorf("MarkInfo not emitted with typed Boolean")
	}
	if strings.Contains(out, "/Suspects") {
		t.Errorf("/Suspects should be omitted (default false)")
	}
	if !strings.Contains(out, NamespacePDF20SSN) {
		t.Errorf("PDF 2.0 SSN namespace URI not present in output")
	}
	if !strings.Contains(out, NamespaceHTML5) {
		t.Errorf("HTML5 namespace URI not present in output")
	}
	if !strings.Contains(out, "/Namespaces") {
		t.Errorf("/Namespaces array not on StructTreeRoot")
	}
	if !strings.Contains(out, "/Type /Namespace") {
		t.Errorf("Namespace objects not serialized with /Type /Namespace")
	}
	if !strings.Contains(out, "pdfuaid:part>2") {
		t.Errorf("XMP pdfuaid:part not set to 2")
	}
	if !strings.Contains(out, "pdfuaid:rev>2024") {
		t.Errorf("XMP pdfuaid:rev (ISO 14289-2 §5) not emitted as four-digit year")
	}
}

func TestSetNamespaceRoleMapEmitsRoleMapNS(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.Format = FormatPDFUA2
	d.SuppressInfo = true
	d.DefaultLanguageTag = "en"
	d.Title = "RoleMapNS smoke test"
	// Declare HTML5 with a role map that targets PDF 2.0 SSN
	d.DeclareNamespace(NamespacePDF20SSN)
	d.SetNamespaceRoleMap(NamespaceHTML5, map[string]NamespaceRoleEntry{
		"h1": {TargetRole: "H1", TargetNS: NamespacePDF20SSN},
		"p":  {TargetRole: "P", TargetNS: NamespacePDF20SSN},
	})
	root := &StructureElement{Role: "Document", NS: NamespacePDF20SSN}
	h1 := &StructureElement{Role: "h1", NS: NamespaceHTML5}
	root.AddChild(h1)
	d.RootStructureElement = root
	d.NewPage().Shipout()
	if err := d.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := d.PDFWriter.FinishAndClose(); err != nil {
		t.Fatalf("FinishAndClose: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "/RoleMapNS") {
		t.Errorf("/RoleMapNS not present on the HTML5 namespace dict")
	}
	if !strings.Contains(out, "/h1 [ /H1") {
		t.Errorf("RoleMapNS entry for h1 → H1 not emitted in expected array form")
	}
}

func TestScopeHierarchy(t *testing.T) {
	// The code relies on the scope constants being ordered from innermost to outermost.
	// This test ensures that invariant is maintained.
	if !(ScopeGlyph < ScopeArray) {
		t.Error("ScopeGlyph must be less than ScopeArray")
	}
	if !(ScopeArray < ScopeText) {
		t.Error("ScopeArray must be less than ScopeText")
	}
	if !(ScopeText < ScopePage) {
		t.Error("ScopeText must be less than ScopePage")
	}
}
