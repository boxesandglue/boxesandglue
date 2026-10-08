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
	rule = d.CreateSVGNodeFromDocument(parseSVG(t, overflowingSVG), 0, 0)
	if want := "q 0 -100 100 100 re W n\n"; !strings.HasPrefix(rule.Pre, want) {
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
