package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// The current color of the host reaches currentColor in the SVG, the
// TextRenderer and the crop still work through the options (svgreader#7).
func TestSVGCurrentColor(t *testing.T) {
	const icon = `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor"><path d="M0 0 L10 10"/><rect width="5" height="5" fill="currentColor"/></svg>`
	var buf bytes.Buffer
	d := NewDocument(&buf)
	cyan := &color.Color{Space: color.ColorCMYK, C: 1}
	sz := bag.MustSP("10pt")

	rule := d.CreateSVGNodeWithOptions(parseSVG(t, icon), sz, sz, SVGOptions{CurrentColor: cyan})
	for _, want := range []string{"1 0 0 0 K", "1 0 0 0 k"} {
		if !strings.Contains(rule.Pre, want) {
			t.Errorf("%q is missing in\n%s", want, rule.Pre)
		}
	}

	// Without a current color, currentColor is black as before.
	plain := d.CreateSVGNodeFromDocument(parseSVG(t, icon), sz, sz)
	if strings.Contains(plain.Pre, "1 0 0 0 K") || !strings.Contains(plain.Pre, "0 0 0 RG") {
		t.Errorf("without a current color the stroke is not black:\n%s", plain.Pre)
	}

	// The crop of the options is the one of CreateSVGNodeFromDocumentCrop.
	crop := SVGOptions{Crop: node.ImageCrop{Width: 5, Height: 5}}
	a := d.CreateSVGNodeWithOptions(parseSVG(t, icon), sz, sz, crop)
	b := d.CreateSVGNodeFromDocumentCrop(parseSVG(t, icon), crop.Crop, sz, sz)
	if a.Pre != b.Pre {
		t.Errorf("crop through the options differs:\n%s\n---\n%s", a.Pre, b.Pre)
	}
}
