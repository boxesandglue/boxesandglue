package document

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// encodePNG returns m as a PNG file.
func encodePNG(t *testing.T, m image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pdfxImages are 2 × 2 pixel PNGs of each kind PDF/X-3 cares about.
func pdfxImages(t *testing.T) map[string][]byte {
	gray8 := image.NewGray(image.Rect(0, 0, 2, 2))
	gray16 := image.NewGray16(image.Rect(0, 0, 2, 2))
	alpha := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	alpha.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	key := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{
		color.NRGBA{}, color.NRGBA{R: 255, A: 255},
	})
	key.SetColorIndex(1, 1, 1)
	return map[string][]byte{
		"gray8":    encodePNG(t, gray8),
		"gray16":   encodePNG(t, gray16),
		"alpha":    encodePNG(t, alpha),
		"colorkey": encodePNG(t, key),
	}
}

// finishWithImage places the image data on a page of a document in format
// and returns the error of Finish.
func finishWithImage(t *testing.T, format Format, data []byte) error {
	t.Helper()
	d := NewDocument(&bytes.Buffer{})
	d.Format = format
	d.Title = "Test"
	imgf, err := d.LoadImageFromReader(bytes.NewReader(data), "/MediaBox", 1)
	if err != nil {
		t.Fatal(err)
	}
	img := d.CreateImageNodeFromImagefile(imgf, 1, "/MediaBox")
	img.Width = bag.MustSP("1cm")
	img.Height = bag.MustSP("1cm")
	p := d.NewPage()
	p.OutputAt(bag.MustSP("2cm"), bag.MustSP("20cm"), node.Vpack(img))
	p.Shipout()
	return d.Finish()
}

// PDF/X-3 refuses 16 bits per component and a soft mask, but accepts a
// color key. PDF/X-4 and plain PDF accept every image.
func TestPDFX3RefusesImages(t *testing.T) {
	images := pdfxImages(t)
	for _, tc := range []struct {
		image string
		want  string // part of the PDF/X-3 error, empty for none
	}{
		{"gray8", ""},
		{"colorkey", ""},
		{"gray16", "has 16 bits per channel, which PDF/X-3 does not allow"},
		{"alpha", "has an alpha channel, which PDF/X-3 does not allow"},
	} {
		t.Run(tc.image, func(t *testing.T) {
			err := finishWithImage(t, FormatPDFX3, images[tc.image])
			switch {
			case tc.want == "" && err != nil:
				t.Errorf("PDF/X-3: unexpected error %v", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Errorf("PDF/X-3: got error %v, want %q", err, tc.want)
			}
			for _, f := range []Format{FormatPDFX4, FormatPDF} {
				if err := finishWithImage(t, f, images[tc.image]); err != nil {
					t.Errorf("%s: unexpected error %v", f, err)
				}
			}
		})
	}
}

// The error names the image file, and loading the same file twice reports
// it once.
func TestPDFX3ImageErrorNamesFile(t *testing.T) {
	fn := filepath.Join(t.TempDir(), "deep.png")
	if err := os.WriteFile(fn, pdfxImages(t)["gray16"], 0o644); err != nil {
		t.Fatal(err)
	}
	d := NewDocument(&bytes.Buffer{})
	d.Format = FormatPDFX3
	d.Title = "Test"
	p := d.NewPage()
	for i := range 2 {
		imgf, err := d.LoadImageFile(fn)
		if err != nil {
			t.Fatal(err)
		}
		img := d.CreateImageNodeFromImagefile(imgf, 1, "/MediaBox")
		img.Width = bag.MustSP("1cm")
		img.Height = bag.MustSP("1cm")
		p.OutputAt(bag.MustSP("2cm"), bag.MustSP("20cm")-bag.ScaledPoint(i)*bag.MustSP("2cm"), node.Vpack(img))
	}
	p.Shipout()
	err := d.Finish()
	want := "image " + fn + " has 16 bits per channel, which PDF/X-3 does not allow; convert it to 8 bits"
	if err == nil || err.Error() != want {
		t.Errorf("got error %v, want %q", err, want)
	}
}

// PDF/X requires a title. Without one, Finish falls back to "Untitled" and
// writes dc:title as a language alternative, as for PDF/UA.
func TestPDFXTitleFallback(t *testing.T) {
	for _, f := range []Format{FormatPDFX3, FormatPDFX4} {
		var buf bytes.Buffer
		d := NewDocument(&buf)
		d.Format = f
		d.CompressLevel = 0
		d.NewPage().Shipout()
		if err := d.Finish(); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.Contains(out, "/Title (Untitled)") {
			t.Errorf("%s: no /Title (Untitled) in the Info dictionary", f)
		}
		if !strings.Contains(out, `<rdf:li xml:lang="x-default">Untitled</rdf:li>`) {
			t.Errorf("%s: dc:title is not a language alternative", f)
		}
	}
}

// Plain PDF keeps an empty title.
func TestPDFNoTitleFallback(t *testing.T) {
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.CompressLevel = 0
	d.NewPage().Shipout()
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Untitled") {
		t.Error("plain PDF got a placeholder title")
	}
}

// finishPDF writes a page with a link in format f and returns the PDF.
func finishPDF(t *testing.T, f Format) []byte {
	t.Helper()
	var buf bytes.Buffer
	d := NewDocument(&buf)
	d.Format = f
	d.Title = "Title"
	p := d.NewPage()
	link := node.NewStartStop()
	link.Action = node.ActionHyperlink
	link.Value = &Hyperlink{URI: "https://example.com"}
	end := node.NewStartStop()
	end.StartNode = link
	node.InsertAfter(link, link, end)
	p.OutputAt(bag.MustSP("2cm"), bag.MustSP("20cm"), node.Vpack(node.Hpack(link)))
	p.Shipout()
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// PDF/X has no use for a link: an annotation in the printed area and an action
// are both refused. A color reset must not be RGB, and whatever RGB the
// content brings gets a device independent meaning through DefaultRGB.
func TestPDFXLinksAndRGB(t *testing.T) {
	for _, tc := range []struct {
		name      string
		format    Format
		annot     bool
		defaultCS bool
	}{
		{"PDF", FormatPDF, true, false},
		{"PDF/A-3b", FormatPDFA3b, true, true},
		{"PDF/X-3", FormatPDFX3, false, true},
		{"PDF/X-4", FormatPDFX4, false, true},
	} {
		out := finishPDF(t, tc.format)
		if got := bytes.Contains(out, []byte("/Annot")); got != tc.annot {
			t.Errorf("%s: annotation %t, want %t", tc.name, got, tc.annot)
		}
		if got := bytes.Contains(out, []byte("/DefaultRGB")); got != tc.defaultCS {
			t.Errorf("%s: DefaultRGB %t, want %t", tc.name, got, tc.defaultCS)
		}
	}
}
