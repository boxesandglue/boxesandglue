package document

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// vbox stacks nodes in a VList.
func vbox(nodes ...node.Node) *node.VList {
	var head node.Node
	for _, n := range nodes {
		head = node.InsertAfter(head, node.Tail(head), n)
	}
	return node.Vpack(head)
}

// ruleOfWidth is a VList holding a 1cm high rule of width wd, which tells
// its fill apart from the others in a content stream.
func ruleOfWidth(wd string) *node.VList {
	r := node.NewRule()
	r.Width = bag.MustSP(wd)
	r.Height = bag.MustSP("1cm")
	return node.Vpack(r)
}

// paintLast marks vl PaintLast.
func paintLast(vl *node.VList) *node.VList {
	if vl.Attributes == nil {
		vl.Attributes = node.H{}
	}
	vl.Attributes[PaintLast] = true
	return vl
}

// pageStream ships vl out at 2cm, 20cm on a page of d and returns the
// uncompressed content stream of that page.
func pageStream(t *testing.T, d *PDFDocument, buf *bytes.Buffer, vl *node.VList) string {
	t.Helper()
	d.CompressLevel = 0
	p := d.NewPage()
	p.OutputAt(bag.MustSP("2cm"), bag.MustSP("20cm"), vl)
	p.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	at := strings.Index(out, " cm ")
	if at < 0 {
		t.Fatalf("no content stream in\n%s", out)
	}
	start := strings.LastIndex(out[:at], "stream")
	end := at + strings.Index(out[at:], "endstream")
	return out[start:end]
}

func renderStream(t *testing.T, vl *node.VList) string {
	t.Helper()
	var buf bytes.Buffer
	return pageStream(t, NewDocument(&buf), &buf, vl)
}

// fillPos returns where the fill of the rule of width wd starts in stream,
// and the instructions that place and draw it.
func fillPos(t *testing.T, stream, wd string) (int, string) {
	t.Helper()
	re := regexp.MustCompile(fmt.Sprintf(`1 0 0 1 \S+ \S+ cm q 0 0 %s \S+ re f Q`, regexp.QuoteMeta(bag.MustSP(wd).String())))
	loc := re.FindStringIndex(stream)
	if loc == nil {
		t.Fatalf("no fill of width %s in\n%s", wd, stream)
	}
	return loc[0], stream[loc[0]:loc[1]]
}

// A marked VList is painted after its siblings, at the place it has in the
// list, and the siblings stay where they are.
func TestPaintLastPaintsAfterSiblings(t *testing.T) {
	plain := renderStream(t, vbox(ruleOfWidth("1cm"), ruleOfWidth("2cm"), ruleOfWidth("3cm")))
	marked := renderStream(t, vbox(paintLast(ruleOfWidth("1cm")), ruleOfWidth("2cm"), ruleOfWidth("3cm")))

	first, firstAt := fillPos(t, marked, "1cm")
	second, secondAt := fillPos(t, marked, "2cm")
	third, thirdAt := fillPos(t, marked, "3cm")
	if !(second < third && third < first) {
		t.Errorf("paint order: want 2cm, 3cm, then the marked 1cm; got positions %d, %d, %d in\n%s", first, second, third, marked)
	}
	for wd, got := range map[string]string{"1cm": firstAt, "2cm": secondAt, "3cm": thirdAt} {
		if _, want := fillPos(t, plain, wd); got != want {
			t.Errorf("the %s rule is drawn as %q, want %q as without the mark", wd, got, want)
		}
	}
}

// A marked VList inside a marked VList is painted after the outer one.
func TestPaintLastNested(t *testing.T) {
	outer := paintLast(vbox(paintLast(ruleOfWidth("1cm")), ruleOfWidth("2cm")))
	stream := renderStream(t, vbox(outer, ruleOfWidth("3cm")))

	inner, _ := fillPos(t, stream, "1cm")
	rest, _ := fillPos(t, stream, "2cm")
	sibling, _ := fillPos(t, stream, "3cm")
	if !(sibling < rest && rest < inner) {
		t.Errorf("paint order: want 3cm, the outer box's 2cm, then the inner 1cm; got positions %d, %d, %d in\n%s", sibling, rest, inner, stream)
	}
}

// Under PDF/UA, a marked VList keeps its place in the reading order, and the
// content of a marked VList without a tag of its own is still marked as its
// parent's.
func TestPaintLastReadingOrder(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.Format = FormatPDFUA
	d.SuppressInfo = true
	d.DefaultLanguageTag = "en"
	d.Title = "PaintLast reading order"

	root := &StructureElement{Role: "Document"}
	div := &StructureElement{Role: "Div"}
	fig := &StructureElement{Role: "Figure", Alt: "a rule"}
	para := &StructureElement{Role: "P"}
	root.AddChild(div)
	div.AddChild(fig)
	div.AddChild(para)
	d.RootStructureElement = root

	figBox := paintLast(ruleOfWidth("1cm"))
	figBox.Attributes["tag"] = fig
	paraBox := ruleOfWidth("2cm")
	paraBox.Attributes = node.H{"tag": para}
	untagged := paintLast(ruleOfWidth("3cm"))
	container := vbox(figBox, paraBox, untagged)
	container.Attributes = node.H{"tag": div}

	stream := pageStream(t, d, &buf, container)
	if err := d.PDFWriter.FinishAndClose(); err != nil {
		t.Fatalf("FinishAndClose: %v", err)
	}

	figAt := strings.Index(stream, "/Figure <</MCID")
	paraAt := strings.Index(stream, "/P <</MCID")
	if figAt < 0 || paraAt < 0 || paraAt > figAt {
		t.Fatalf("want P marked before the deferred Figure in the stream, got positions %d, %d in\n%s", paraAt, figAt, stream)
	}

	kStr, ok := div.Obj.Dictionary["K"].(string)
	if !ok {
		t.Fatalf("Div /K is not a string: %T", div.Obj.Dictionary["K"])
	}
	figRef := strings.Index(kStr, fig.Obj.ObjectNumber.Ref())
	paraRef := strings.Index(kStr, para.Obj.ObjectNumber.Ref())
	if figRef < 0 || paraRef < 0 || figRef > paraRef {
		t.Errorf("Div /K: want the Figure before the P as in the list, got %q", kStr)
	}

	// The untagged marked box is painted in a Div sequence of its own.
	if len(div.mcids) != 2 {
		t.Fatalf("Div has %d marked-content sequences, want 2 (its own and the deferred box's)", len(div.mcids))
	}
	divSeq := fmt.Sprintf("/Div <</MCID %d>> BDC", div.mcids[1].mcid)
	seqAt := strings.Index(stream, divSeq)
	untaggedAt, _ := fillPos(t, stream, "3cm")
	if seqAt < 0 || seqAt > untaggedAt || strings.Contains(stream[seqAt:untaggedAt], "EMC") {
		t.Errorf("want the deferred untagged box inside %q, got\n%s", divSeq, stream)
	}
	if lastMCR := strings.LastIndex(kStr, fmt.Sprintf("/MCID %d", div.mcids[1].mcid)); lastMCR < paraRef {
		t.Errorf("Div /K: want the deferred box's sequence after the P, got %q", kStr)
	}
}

// hbox lines nodes up in an HList, packed in a VList.
func hbox(nodes ...node.Node) *node.VList {
	var head node.Node
	for _, n := range nodes {
		head = node.InsertAfter(head, node.Tail(head), n)
	}
	return node.Vpack(node.Hpack(head))
}

// bareRule is a 1cm high rule of width wd.
func bareRule(wd string) *node.Rule {
	r := node.NewRule()
	r.Width = bag.MustSP(wd)
	r.Height = bag.MustSP("1cm")
	return r
}

// paintLastRule marks r PaintLast.
func paintLastRule(r *node.Rule) *node.Rule {
	r.Attributes = node.H{PaintLast: true}
	return r
}

// assertPaintedLast checks that in marked the fill of width last comes after
// the fills of the widths in others, and that each of them is drawn as in
// plain.
func assertPaintedLast(t *testing.T, plain, marked, last string, others ...string) {
	t.Helper()
	lastAt, _ := fillPos(t, marked, last)
	for _, wd := range append(others, last) {
		at, got := fillPos(t, marked, wd)
		if wd != last && at > lastAt {
			t.Errorf("the %s fill comes after the marked %s fill in\n%s", wd, last, marked)
		}
		if _, want := fillPos(t, plain, wd); got != want {
			t.Errorf("the %s fill is drawn as %q, want %q as without the mark", wd, got, want)
		}
	}
}

// A marked rule in a vertical list is painted after its siblings.
func TestPaintLastRuleInVList(t *testing.T) {
	plain := renderStream(t, vbox(bareRule("1cm"), ruleOfWidth("2cm"), bareRule("3cm")))
	marked := renderStream(t, vbox(paintLastRule(bareRule("1cm")), ruleOfWidth("2cm"), bareRule("3cm")))
	assertPaintedLast(t, plain, marked, "1cm", "2cm", "3cm")
}

// A marked rule in a horizontal list is painted after its siblings.
func TestPaintLastRuleInHList(t *testing.T) {
	plain := renderStream(t, hbox(bareRule("1cm"), bareRule("2cm"), bareRule("3cm")))
	marked := renderStream(t, hbox(paintLastRule(bareRule("1cm")), bareRule("2cm"), bareRule("3cm")))
	assertPaintedLast(t, plain, marked, "1cm", "2cm", "3cm")
}

// A marked VList in a horizontal list is painted after its siblings.
func TestPaintLastVListInHList(t *testing.T) {
	plain := renderStream(t, hbox(ruleOfWidth("1cm"), bareRule("2cm"), ruleOfWidth("3cm")))
	marked := renderStream(t, hbox(paintLast(ruleOfWidth("1cm")), bareRule("2cm"), ruleOfWidth("3cm")))
	assertPaintedLast(t, plain, marked, "1cm", "2cm", "3cm")
}

var textMatrix = regexp.MustCompile(`(\S+ ){6}Tm`)

// A glyph after a marked rule in a line keeps its place: the text is placed
// anew after the rule's width as if the rule were drawn there.
func TestPaintLastGlyphAfterRule(t *testing.T) {
	render := func(mark bool) string {
		var buf bytes.Buffer
		d := NewDocument(&buf)
		face, err := d.LoadFace("../../qa/fonts/upem/fonts/CrimsonPro-Regular.ttf", 0)
		if err != nil {
			t.Fatal(err)
		}
		fnt := font.NewFont(face, bag.MustSP("12pt"))
		var nodes []node.Node
		for i, a := range fnt.Shape("bc", nil, nil) {
			if i == 1 {
				r := bareRule("1cm")
				if mark {
					paintLastRule(r)
				}
				nodes = append(nodes, r)
			}
			g := node.NewGlyph()
			g.Font = fnt
			g.Codepoint = a.Codepoint
			g.Components = a.Components
			g.Width = a.Advance
			nodes = append(nodes, g)
		}
		return pageStream(t, d, &buf, hbox(nodes...))
	}
	plain, marked := render(false), render(true)
	want := textMatrix.FindAllString(plain, -1)
	got := textMatrix.FindAllString(marked, -1)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("text matrices %q, want %q as without the mark in\n%s", got, want, marked)
	}
	assertPaintedLast(t, plain, marked, "1cm")
}

// Under PDF/UA a marked rule in a tagged list is painted in a sequence of
// the list's structure element, one in an untagged list as an artifact.
func TestPaintLastRuleMarkedContent(t *testing.T) {
	render := func(tagged bool) string {
		var buf bytes.Buffer
		d := NewDocument(&buf)
		d.Format = FormatPDFUA
		d.SuppressInfo = true
		d.DefaultLanguageTag = "en"
		d.Title = "PaintLast rule"
		root := &StructureElement{Role: "Document"}
		div := &StructureElement{Role: "Div"}
		root.AddChild(div)
		d.RootStructureElement = root

		container := vbox(paintLastRule(bareRule("1cm")), bareRule("2cm"))
		if tagged {
			container.Attributes = node.H{"tag": div}
		}
		stream := pageStream(t, d, &buf, container)
		if err := d.PDFWriter.FinishAndClose(); err != nil {
			t.Fatalf("FinishAndClose: %v", err)
		}
		return stream
	}
	for _, tc := range []struct {
		tagged bool
		seq    string
	}{
		{true, "/Div <</MCID 1>> BDC"},
		{false, "/Artifact BMC"},
	} {
		stream := render(tc.tagged)
		ruleAt, _ := fillPos(t, stream, "1cm")
		seqAt := strings.LastIndex(stream[:ruleAt], tc.seq)
		if seqAt < 0 || strings.Contains(stream[seqAt:ruleAt], "EMC") {
			t.Errorf("tagged %t: want the deferred rule inside %q, got\n%s", tc.tagged, tc.seq, stream)
		}
		if siblingAt, _ := fillPos(t, stream, "2cm"); siblingAt > ruleAt {
			t.Errorf("tagged %t: the sibling comes after the deferred rule in\n%s", tc.tagged, stream)
		}
	}
}
