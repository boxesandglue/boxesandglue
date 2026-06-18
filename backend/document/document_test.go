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
		if got := c.format.IsPDFUA(); got != c.want {
			t.Errorf("format %v: IsPDFUA() = %v, want %v", c.format, got, c.want)
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

func TestFormatPDFA3UA1CombinedXMP(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	// Combined conformance: PDF/A-3b AND PDF/UA-1 in the same document.
	// The two sub-conformances are orthogonal — both identifier sets must
	// land in the same rdf:Description block (xmpMM:RenditionClass once,
	// pdfaid:* and pdfuaid:* both present).
	d.Format = Format{
		PDFA:  &PDFAConf{Part: 3, Level: PDFALevelB},
		PDFUA: &PDFUAConf{Part: 1, Rev: "2014"},
	}
	d.Title = "PDF/A-3b + PDF/UA-1 smoke test"
	d.DefaultLanguageTag = "en"
	d.SuppressInfo = true
	d.RootStructureElement = &StructureElement{Role: "Document"}
	d.NewPage().Shipout()
	if err := d.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := d.PDFWriter.FinishAndClose(); err != nil {
		t.Fatalf("FinishAndClose: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"pdfaid:part>3",
		"pdfaid:conformance>B",
		"pdfuaid:part>1",
		"pdfuaid:rev>2014",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("combined PDF/A+UA XMP missing %q", want)
		}
	}
	// RenditionClass must appear exactly once, not duplicated by both
	// PDFA and PDFX branches firing.
	if got := strings.Count(out, "xmpMM:RenditionClass"); got != 2 {
		// open tag + close tag = 2 occurrences for a single emission
		t.Errorf("xmpMM:RenditionClass: want 2 occurrences (open+close), got %d", got)
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

// TestKArrayDocumentOrder guards the reading-order contract of the /K array
// (ISO 14289 §7.2): when a structure element interleaves its own marked
// content with a child element — the canonical case being an inline Formula
// that sits between two runs of paragraph text — the /K entries must appear
// in document order, not grouped by kind (all MCRs, then all children).
//
// We hand-build the tree a renderer would produce for "<p>text Formula
// text</p>": the P owns two marked-content runs (reading-order stamps 1 and
// 3) with a Formula child stamped 2 in between. The Formula ref must land
// between the two MCRs in P's /K array.
func TestKArrayDocumentOrder(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.Format = FormatPDFUA
	d.SuppressInfo = true
	d.DefaultLanguageTag = "en"
	d.Title = "K-order smoke test"

	root := &StructureElement{Role: "Document"}
	para := &StructureElement{Role: "P"}
	formula := &StructureElement{Role: "Formula", Alt: "a squared"}
	root.AddChild(para)
	para.AddChild(formula)
	d.RootStructureElement = root

	// Reading order: text run (mcid 0, seq 1) → formula glyphs (mcid 1,
	// seq 2) → trailing text run (mcid 2, seq 3). All on page index 0.
	para.mcids = []mcidEntry{{pageIndex: 0, mcid: 0, seq: 1}, {pageIndex: 0, mcid: 2, seq: 3}}
	formula.mcids = []mcidEntry{{pageIndex: 0, mcid: 1, seq: 2}}

	d.NewPage().Shipout()
	if err := d.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := d.PDFWriter.FinishAndClose(); err != nil {
		t.Fatalf("FinishAndClose: %v", err)
	}

	kStr, ok := para.Obj.Dictionary["K"].(string)
	if !ok {
		t.Fatalf("P /K entry is not a string: %T", para.Obj.Dictionary["K"])
	}
	formulaRef := formula.Obj.ObjectNumber.Ref()
	posMCID0 := strings.Index(kStr, "/MCID 0")
	posFormula := strings.Index(kStr, formulaRef)
	posMCID2 := strings.Index(kStr, "/MCID 2")
	if posMCID0 < 0 || posFormula < 0 || posMCID2 < 0 {
		t.Fatalf("P /K missing an expected entry: %q (formula ref %q)", kStr, formulaRef)
	}
	if !(posMCID0 < posFormula && posFormula < posMCID2) {
		t.Errorf("P /K not in document reading order: want MCID0 < Formula < MCID2, got positions %d, %d, %d in %q",
			posMCID0, posFormula, posMCID2, kStr)
	}
}

// TestReadingKey verifies that a structure element's reading-order key is the
// smallest reading-order stamp anywhere in its subtree, so a parent can sort
// it correctly even when its content is nested several levels deep.
func TestReadingKey(t *testing.T) {
	leaf := &StructureElement{Role: "Span"}
	leaf.mcids = []mcidEntry{{seq: 7}, {seq: 4}}
	mid := &StructureElement{Role: "P"}
	mid.objRefs = []objRefEntry{{seq: 9}}
	mid.AddChild(leaf)
	if got := readingKey(mid); got != 4 {
		t.Errorf("readingKey(mid) = %d, want 4 (smallest stamp in subtree)", got)
	}
	empty := &StructureElement{Role: "Div"}
	if got := readingKey(empty); got != int(^uint(0)>>1) {
		t.Errorf("readingKey(empty) = %d, want max int (sorts last)", got)
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
