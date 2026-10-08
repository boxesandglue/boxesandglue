package document

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"regexp"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// pngImage returns a node for a wd × ht pixel PNG, sized boxW × boxH points.
func pngImage(t *testing.T, d *PDFDocument, wd, ht int, boxW, boxH float64) *node.Image {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, wd, ht))); err != nil {
		t.Fatal(err)
	}
	imgf, err := d.LoadImageFromReader(bytes.NewReader(buf.Bytes()), "/MediaBox", 1)
	if err != nil {
		t.Fatal(err)
	}
	img := d.CreateImageNodeFromImagefile(imgf, 1, "/MediaBox")
	img.Width = bag.ScaledPointFromFloat(boxW)
	img.Height = bag.ScaledPointFromFloat(boxH)
	return img
}

// pdfImage returns a node for the page of a wd × ht point PDF.
func pdfImage(t *testing.T, d *PDFDocument, wd, ht float64) *node.Image {
	t.Helper()
	var src bytes.Buffer
	sd := NewDocument(&src)
	sd.DefaultPageWidth = bag.ScaledPointFromFloat(wd)
	sd.DefaultPageHeight = bag.ScaledPointFromFloat(ht)
	sd.NewPage().Shipout()
	if err := sd.Finish(); err != nil {
		t.Fatal(err)
	}
	imgf, err := d.LoadImageFromReader(bytes.NewReader(src.Bytes()), "/MediaBox", 1)
	if err != nil {
		t.Fatal(err)
	}
	return d.CreateImageNodeFromImagefile(imgf, 1, "/MediaBox")
}

// imageStream ships img out at 2cm, 20cm on a page of d, inside a line when
// inLine is set, and returns the page's content stream.
func imageStream(t *testing.T, d *PDFDocument, buf *bytes.Buffer, img *node.Image, inLine bool) string {
	t.Helper()
	if inLine {
		return pageStream(t, d, buf, node.Vpack(node.Hpack(img)))
	}
	return pageStream(t, d, buf, node.Vpack(img))
}

// drawing returns the q … Q group that draws the image in stream.
func drawing(t *testing.T, stream string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)q [^Q]*Do\s*Q\n`).FindString(stream)
	if m == "" {
		t.Fatalf("no image drawn in\n%s", stream)
	}
	return m
}

var layouts = []struct {
	name   string
	inLine bool
}{{"hlist", true}, {"vlist", false}}

// Without a crop, the image is drawn as it always was.
func TestImageWithoutCrop(t *testing.T) {
	for _, l := range layouts {
		t.Run(l.name, func(t *testing.T) {
			var buf bytes.Buffer
			d := NewDocument(&buf)
			img := pngImage(t, d, 40, 30, 30, 22.5)
			got := drawing(t, imageStream(t, d, &buf, img, l.inLine))
			want := fmt.Sprintf("q %f 0 0 %f 56.69 544.43 cm %s Do Q\n", img.Width.ToPT(), img.Height.ToPT(), img.ImageFile.InternalName())
			if got != want {
				t.Errorf("got  %q\nwant %q", got, want)
			}
		})
	}
}

// cropped returns how img is drawn when the whole image is kx × ky times the
// size of its box and shifted left by fx and down by fy box sizes.
func cropped(img *node.Image, kx, ky, fx, fy float64) string {
	wd, ht := img.Width.ToPT(), (img.Height + img.Depth).ToPT()
	x, y := bag.MustSP("2cm"), bag.MustSP("20cm")-img.Height-img.Depth
	return fmt.Sprintf("q %s %s %s %s re W n\n%f 0 0 %f %s %s cm %s Do\nQ\n",
		x, y, img.Width, img.Height+img.Depth,
		wd*kx/img.ImageFile.ScaleX, ht*ky/img.ImageFile.ScaleY,
		x-bag.ScaledPointFromFloat(wd*fx), y-bag.ScaledPointFromFloat(ht*fy),
		img.ImageFile.InternalName())
}

// A crop region fills the image's box: the whole image is scaled and shifted
// so that the region lands on the box, and drawn inside a clip to the box.
func TestImageCrop(t *testing.T) {
	for _, l := range layouts {
		for _, tc := range []struct {
			name           string
			crop           node.ImageCrop
			kx, ky, fx, fy float64
		}{
			// A quarter of the 40 × 30 px image fills the box.
			{"top left", node.ImageCrop{X: 0, Y: 0, Width: 20, Height: 15}, 2, 2, 0, 1},
			{"bottom right", node.ImageCrop{X: 20, Y: 15, Width: 20, Height: 15}, 2, 2, 1, 0},
			// Half the width and a fifth of the height, 6 px from the top.
			{"middle strip", node.ImageCrop{X: 10, Y: 6, Width: 20, Height: 6}, 2, 5, 0.5, 3},
		} {
			t.Run(l.name+"/"+tc.name, func(t *testing.T) {
				var buf bytes.Buffer
				d := NewDocument(&buf)
				img := pngImage(t, d, 40, 30, 30, 22.5)
				img.Crop = &tc.crop
				got := drawing(t, imageStream(t, d, &buf, img, l.inLine))
				if want := cropped(img, tc.kx, tc.ky, tc.fx, tc.fy); got != want {
					t.Errorf("got  %q\nwant %q", got, want)
				}
			})
		}
	}
}

// An image with depth is clipped to its whole extent, above and below the
// baseline.
func TestImageCropWithDepth(t *testing.T) {
	for _, l := range layouts {
		t.Run(l.name, func(t *testing.T) {
			var buf bytes.Buffer
			d := NewDocument(&buf)
			img := pngImage(t, d, 40, 30, 30, 15)
			img.Depth = bag.ScaledPointFromFloat(7.5)
			img.Crop = &node.ImageCrop{X: 0, Y: 0, Width: 20, Height: 15}
			got := drawing(t, imageStream(t, d, &buf, img, l.inLine))
			if want := cropped(img, 2, 2, 0, 1); got != want {
				t.Errorf("got  %q\nwant %q", got, want)
			}
		})
	}
}

// A PDF page is cropped in points of its page box.
func TestImageCropPDF(t *testing.T) {
	for _, l := range layouts {
		t.Run(l.name, func(t *testing.T) {
			var buf bytes.Buffer
			d := NewDocument(&buf)
			img := pdfImage(t, d, 200, 100)
			img.Width, img.Height = bag.ScaledPointFromFloat(100), bag.ScaledPointFromFloat(50)
			// The middle half of the 200 × 100 page fills the 100 × 50 box.
			img.Crop = &node.ImageCrop{X: 50, Y: 25, Width: 100, Height: 50}
			got := drawing(t, imageStream(t, d, &buf, img, l.inLine))
			if want := cropped(img, 2, 2, 0.5, 0.5); got != want {
				t.Errorf("got  %q\nwant %q", got, want)
			}
		})
	}
}

// An empty region is no crop.
func TestImageCropEmpty(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	img := pngImage(t, d, 40, 30, 30, 22.5)
	img.Crop = &node.ImageCrop{X: 10, Y: 10, Width: 0, Height: 15}
	if got := drawing(t, imageStream(t, d, &buf, img, false)); strings.Contains(got, " W n") {
		t.Errorf("empty crop clips: %q", got)
	}
}

// For an SVG, the region becomes the viewBox and the natural size, and the
// drawing is clipped to it.
func TestSVGCrop(t *testing.T) {
	d := NewDocument(&bytes.Buffer{})
	// 200 × 100 user units at a natural size of 100 × 50.
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50" viewBox="0 0 200 100"><rect width="200" height="100"/></svg>`
	crop := node.ImageCrop{X: 25, Y: 10, Width: 50, Height: 25}

	rule := d.CreateSVGNodeFromDocumentCrop(parseSVG(t, src), crop, 0, 0)
	if rule.Width != bag.ScaledPointFromFloat(50) || rule.Height != bag.ScaledPointFromFloat(25) {
		t.Errorf("natural size %s × %s, want 50 × 25", rule.Width, rule.Height)
	}
	// viewBox 50 20 100 50 at 50 × 25 pt: scale 0.5, shifted by the region.
	want := "q 0 -25 50 25 re W n\nq\n0.5 0 0 -0.5 -25 10 cm\n"
	if !strings.HasPrefix(rule.Pre, want) {
		t.Errorf("rule.Pre starts\n%q, want\n%q", rule.Pre[:min(len(rule.Pre), len(want))], want)
	}

	rule = d.CreateSVGNodeFromDocumentCrop(parseSVG(t, src), crop, bag.ScaledPointFromFloat(100), 0)
	if rule.Height != bag.ScaledPointFromFloat(50) {
		t.Errorf("height %s, want 50 from the region's aspect ratio", rule.Height)
	}
	if want := "q 0 -50 100 50 re W n\nq\n1 0 0 -1 -50 20 cm\n"; !strings.HasPrefix(rule.Pre, want) {
		t.Errorf("rule.Pre starts\n%q, want\n%q", rule.Pre[:min(len(rule.Pre), len(want))], want)
	}
}

// Without a viewBox, the region is in user units.
func TestSVGCropWithoutViewBox(t *testing.T) {
	d := NewDocument(&bytes.Buffer{})
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50"><rect width="100" height="50"/></svg>`
	rule := d.CreateSVGNodeFromDocumentCrop(parseSVG(t, src), node.ImageCrop{X: 10, Y: 5, Width: 40, Height: 20}, 0, 0)
	if want := "q 0 -20 40 20 re W n\nq\n1 0 0 -1 -10 5 cm\n"; !strings.HasPrefix(rule.Pre, want) {
		t.Errorf("rule.Pre starts\n%q, want\n%q", rule.Pre[:min(len(rule.Pre), len(want))], want)
	}
}

// The SVG document passed in is left as it was.
func TestSVGCropKeepsDocument(t *testing.T) {
	d := NewDocument(&bytes.Buffer{})
	svgDoc := parseSVG(t, overflowingSVG)
	before := *svgDoc
	d.CreateSVGNodeFromDocumentCrop(svgDoc, node.ImageCrop{X: 10, Y: 10, Width: 20, Height: 20}, 0, 0)
	if svgDoc.ViewBox != before.ViewBox || svgDoc.Width != before.Width || svgDoc.Height != before.Height {
		t.Errorf("document changed: %+v, was %+v", *svgDoc, before)
	}
}
