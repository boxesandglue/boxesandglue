package document

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/svgreader"
)

// overflowingSVG has a square that runs past its 100 × 100 viewBox.
const overflowingSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">
<rect x="60" y="60" width="100" height="100" fill="#ff0000"/>
</svg>`

func parseSVG(t *testing.T, src string) *svgreader.Document {
	t.Helper()
	svgDoc, err := svgreader.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return svgDoc
}

// An SVG shows nothing outside its viewport: the stream is clipped to the
// rule, which hangs down from its origin.
func TestSVGClippedToViewport(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	wd, ht := bag.MustSP("3cm"), bag.MustSP("2cm")
	rule := d.CreateSVGNodeFromDocument(parseSVG(t, overflowingSVG), wd, ht)

	clip := "q 0 -56.69 85.04 56.69 re W n\n"
	if !strings.HasPrefix(rule.Pre, clip) || !strings.HasSuffix(rule.Pre, "\nQ") {
		t.Fatalf("rule.Pre is not clipped to the 85.04 × 56.69 viewport:\n%s", rule.Pre)
	}

	stream := pageStream(t, d, &buf, node.Vpack(rule))
	re := regexp.MustCompile(`(?s)1 0 0 1 \S+ \S+ cm ` + regexp.QuoteMeta(clip) + `q\n.*re\nf\n.*\nQ\nQ`)
	if !re.MatchString(stream) {
		t.Errorf("the square is not drawn inside the clip:\n%s", stream)
	}
}

// Width or height 0 takes the other from the aspect ratio; the clip follows
// the size the SVG is drawn at.
func TestSVGClipFollowsDerivedSize(t *testing.T) {
	d := NewDocument(&bytes.Buffer{})
	rule := d.CreateSVGNodeFromDocument(parseSVG(t, overflowingSVG), bag.MustSP("50pt"), 0)
	if want := "q 0 -50 50 50 re W n\n"; !strings.HasPrefix(rule.Pre, want) {
		t.Errorf("rule.Pre starts %q, want %q", rule.Pre[:min(len(rule.Pre), len(want))], want)
	}
	// The natural size: 100 user units, pixels, are 75pt.
	rule = d.CreateSVGNodeFromDocument(parseSVG(t, overflowingSVG), 0, 0)
	if want := "q 0 -75 75 75 re W n\n"; !strings.HasPrefix(rule.Pre, want) {
		t.Errorf("rule.Pre starts %q, want %q", rule.Pre[:min(len(rule.Pre), len(want))], want)
	}
}

// An SVG without width, height or viewBox gets an empty rule; clipping to it
// would hide the drawing, so it stays unclipped.
func TestSVGWithoutSizeUnclipped(t *testing.T) {
	d := NewDocument(&bytes.Buffer{})
	src := `<svg xmlns="http://www.w3.org/2000/svg"><rect width="10" height="10"/></svg>`
	rule := d.CreateSVGNodeFromDocument(parseSVG(t, src), 0, 0)
	if rule.Width != 0 || rule.Height != 0 {
		t.Fatalf("rule is %s × %s, want empty", rule.Width, rule.Height)
	}
	if strings.Contains(rule.Pre, " W n") {
		t.Errorf("rule.Pre is clipped:\n%s", rule.Pre)
	}
}

const svgGroupWithGradient = `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">
<defs><linearGradient id="g" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#f00"/><stop offset="1" stop-color="#00f"/></linearGradient></defs>
<g opacity="0.5"><rect width="50" height="50" fill="url(#g)"/><rect x="25" width="50" height="50" fill="#0f0" fill-opacity="0.5"/></g>
</svg>`

// finishSVG ships out rule and returns the whole PDF.
func finishSVG(t *testing.T, d *PDFDocument, buf *bytes.Buffer, rule *node.Rule) string {
	t.Helper()
	d.CompressLevel = 0
	p := d.NewPage()
	p.OutputAt(bag.MustSP("2cm"), bag.MustSP("20cm"), node.Vpack(rule))
	p.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// A group with an opacity is a form with a transparency group. Its gradient and
// its alpha value are resources of the form: a form does not see the ones of
// its page, and the pattern matrix must not hold the position on the page.
func TestSVGGroupIsATransparencyGroup(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	rule := d.CreateSVGNodeFromDocument(parseSVG(t, svgGroupWithGradient), 0, 0)
	out := finishSVG(t, d, &buf, rule)

	for _, want := range []string{"/Subtype /Form", "/S /Transparency", "/I true", "/FmSVG1 Do", "/GSca0_5 gs"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q", want)
		}
	}
	at := strings.Index(out, "/Subtype /Form")
	form := out[strings.LastIndex(out[:at], " obj"):at]
	for _, want := range []string{"/Pattern <<", "/ExtGState <<"} {
		if !strings.Contains(form, want) {
			t.Errorf("the resources of the form lack %q:\n%s", want, form)
		}
	}
	// The group starts at the CTM of the root, so nothing is left of it,
	// and no translation to the place on the page has been added.
	if !strings.Contains(out, "/Matrix [1 0 0 1 0 0]") {
		t.Errorf("the pattern matrix is not relative to the form:\n%s", out)
	}
}

// A node that is built and dropped, as in measuring a table, leaves nothing in
// the PDF.
func TestSVGGroupOfUnusedNode(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.CreateSVGNodeFromDocument(parseSVG(t, svgGroupWithGradient), 0, 0)
	rule := d.CreateSVGNodeFromDocument(parseSVG(t, overflowingSVG), 0, 0)
	out := finishSVG(t, d, &buf, rule)

	if strings.Contains(out, "/Subtype /Form") {
		t.Error("a form of a dropped node is in the PDF")
	}
}

// PDF/X-3 forbids transparency, so the drawing is painted opaque in place.
func TestSVGOpaqueWithoutTransparency(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	var err error
	if d.Format, err = ParseFormat("PDF/X-3"); err != nil {
		t.Fatal(err)
	}
	rule := d.CreateSVGNodeFromDocument(parseSVG(t, svgGroupWithGradient), 0, 0)
	if strings.Contains(rule.Pre, " Do") || strings.Contains(rule.Pre, " gs") {
		t.Errorf("transparency in\n%s", rule.Pre)
	}
	if !strings.Contains(rule.Pre, "0 1 0 rg") {
		t.Errorf("the content is not in place:\n%s", rule.Pre)
	}
}

// A shading is written with the page, long before Finish writes DefaultRGB.
func TestSVGShadingUsesDefaultRGB(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	var err error
	if d.Format, err = ParseFormat("PDF/A-3b"); err != nil {
		t.Fatal(err)
	}
	const gradient = `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">
<defs><linearGradient id="g"><stop offset="0" stop-color="#f00"/><stop offset="1" stop-color="#00f"/></linearGradient></defs>
<rect width="50" height="50" fill="url(#g)"/></svg>`
	rule := d.CreateSVGNodeFromDocument(parseSVG(t, gradient), 0, 0)
	out := finishSVG(t, d, &buf, rule)

	if strings.Contains(out, "/ColorSpace /DeviceRGB") {
		t.Error("the shading uses DeviceRGB")
	}
	ref := d.PDFWriter.DefaultRGB.Ref()
	if !strings.Contains(out, "/ColorSpace "+ref) {
		t.Errorf("the shading does not use DefaultRGB %s", ref)
	}
	if !strings.Contains(out, d.PDFWriter.DefaultRGB.String()+" 0 obj") {
		t.Error("DefaultRGB is not in the file")
	}
}

const svgFadingGradient = `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100">
<defs><linearGradient id="g" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="100" y2="0">
<stop offset="0" stop-color="#f00"/><stop offset="1" stop-color="#00f" stop-opacity="0"/></linearGradient></defs>
<rect width="100" height="50" fill="url(#g)"/></svg>`

// The opacity of the stops is a gray gradient in a soft mask.
func TestSVGStopOpacityIsASoftMask(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	rule := d.CreateSVGNodeFromDocument(parseSVG(t, svgFadingGradient), 0, 0)
	out := finishSVG(t, d, &buf, rule)

	for _, want := range []string{
		"/GSMask1 gs", "/GSMask1 ", "/Type /Mask", "/S /Luminosity", "/CS /DeviceGray",
		"/ColorSpace /DeviceGray", "/C0 [1]", "/C1 [0]", "/Pattern cs /Sh scn",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q", want)
		}
	}
}

// The mask is a resource of the form that uses it.
func TestSVGStopOpacityInAGroup(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	src := strings.Replace(svgFadingGradient, `<rect`, `<g opacity="0.5"><rect`, 1)
	src = strings.Replace(src, `</svg>`, `</g></svg>`, 1)
	rule := d.CreateSVGNodeFromDocument(parseSVG(t, src), 0, 0)
	if _, ok := rule.Attributes["svgmasks"]; ok {
		t.Error("the mask of a group is not a resource of the page")
	}
	out := finishSVG(t, d, &buf, rule)
	if !strings.Contains(out, "/GSMask1 ") || !strings.Contains(out, "/Type /Mask") {
		t.Error("the mask is missing")
	}
}

// PDF/X-3 forbids a soft mask as well.
func TestSVGStopOpacityWithoutTransparency(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	var err error
	if d.Format, err = ParseFormat("PDF/X-3"); err != nil {
		t.Fatal(err)
	}
	rule := d.CreateSVGNodeFromDocument(parseSVG(t, svgFadingGradient), 0, 0)
	if strings.Contains(rule.Pre, " gs") {
		t.Errorf("a soft mask in\n%s", rule.Pre)
	}
}
