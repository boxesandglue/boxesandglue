package frontend

import (
	"fmt"
	"strings"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/svgreader"
	"github.com/boxesandglue/textshape/ot"
)

// SVGTextRenderer implements svgreader.TextRenderer using boxesandglue's font
// subsystem. It reuses the same pdf.Face instances as the rest of the document,
// so fonts are not duplicated in the PDF output.
type SVGTextRenderer struct {
	doc           *Document
	DefaultFamily *FontFamily
	usedFaces     map[*pdf.Face]bool
}

// NewSVGTextRenderer creates a text renderer for SVG content. It resolves
// font-family names against the document's registered FontFamilies.
func NewSVGTextRenderer(doc *Document) *SVGTextRenderer {
	return &SVGTextRenderer{
		doc:       doc,
		usedFaces: make(map[*pdf.Face]bool),
	}
}

// UsedFaces returns all pdf.Face instances that were used during rendering.
// These must be registered as page resources so the PDF is valid.
func (tr *SVGTextRenderer) UsedFaces() []*pdf.Face {
	faces := make([]*pdf.Face, 0, len(tr.usedFaces))
	for f := range tr.usedFaces {
		faces = append(faces, f)
	}
	return faces
}

// RenderText implements svgreader.TextRenderer. It shapes the text using
// HarfBuzz (via textshape) and returns PDF text operators.
func (tr *SVGTextRenderer) RenderText(text string, x, y, fontSize float64, fontFamily, fontWeight, fontStyle string, fill svgreader.Color) string {
	ff := tr.resolveFamily(fontFamily)
	if ff == nil {
		return ""
	}

	weight := parseSVGFontWeight(fontWeight)
	style := parseSVGFontStyle(fontStyle)

	fs, err := ff.GetFontSource(weight, style)
	if err != nil {
		return ""
	}

	face, err := tr.doc.LoadFace(fs)
	if err != nil {
		return ""
	}

	tr.usedFaces[face] = true

	// Assemble font features: defaults, then FontSource features (same order
	// as nodebuilding.go). This ensures features like -liga are applied.
	features := append([]ot.Feature{}, tr.doc.DefaultFeatures...)
	features = append(features, parseOpenTypeFeatures(fs.FontFeatures)...)

	fnt := font.NewFont(face, bag.ScaledPointFromFloat(fontSize))
	atoms := fnt.Shape(text, features, nil)

	var buf strings.Builder

	// Set fill color
	if !fill.IsNone {
		fmt.Fprintf(&buf, "%s %s %s rg\n", pdfFloat(fill.R), pdfFloat(fill.G), pdfFloat(fill.B))
	}

	buf.WriteString("BT\n")
	fmt.Fprintf(&buf, "%s %s Tf\n", face.InternalName(), pdfFloat(fontSize*face.Scale))

	xCur := x
	for _, atom := range atoms {
		if atom.IsSpace {
			xCur += atom.Advance.ToPT()
			continue
		}
		face.RegisterCodepoint(atom.Codepoint)
		gx := xCur + atom.XOffset.ToPT()
		gy := y - atom.YOffset.ToPT()
		// Tm with Y-flip to compensate the SVG coordinate transform.
		// The SVG renderer applies a Y-down→Y-up flip via CTM; this Tm
		// flips text back so glyphs appear upright.
		fmt.Fprintf(&buf, "1 0 0 -1 %s %s Tm <%04x> Tj\n", pdfFloat(gx), pdfFloat(gy), atom.Codepoint)
		xCur += atom.Advance.ToPT() + atom.Kernafter.ToPT()
	}

	buf.WriteString("ET")
	return buf.String()
}

// resolveFamily maps an SVG font-family string to a boxesandglue FontFamily.
// It tries direct name match first, then checks comma-separated fallbacks,
// and finally falls back to DefaultFamily.
func (tr *SVGTextRenderer) resolveFamily(name string) *FontFamily {
	name = strings.Trim(name, "'\"")
	if ff, ok := tr.doc.FontFamilies[name]; ok {
		return ff
	}
	// SVG font-family can be a comma-separated list of fallbacks
	for _, candidate := range strings.Split(name, ",") {
		candidate = strings.TrimSpace(strings.Trim(candidate, "'\""))
		if ff, ok := tr.doc.FontFamilies[candidate]; ok {
			return ff
		}
	}
	return tr.DefaultFamily
}

func parseSVGFontWeight(s string) FontWeight {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "bold", "700":
		return FontWeight700
	case "100":
		return FontWeight100
	case "200":
		return FontWeight200
	case "300":
		return FontWeight300
	case "500":
		return FontWeight500
	case "600":
		return FontWeight600
	case "800":
		return FontWeight800
	case "900":
		return FontWeight900
	default:
		return FontWeight400
	}
}

func parseSVGFontStyle(s string) FontStyle {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "italic":
		return FontStyleItalic
	case "oblique":
		return FontStyleOblique
	default:
		return FontStyleNormal
	}
}

func pdfFloat(f float64) string {
	s := fmt.Sprintf("%.4f", f)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}
