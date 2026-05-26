package document

import (
	"fmt"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/svgreader"
)

// pendingShading holds everything needed to materialise a PDF Pattern object
// for a single gradient fill. The renderer has already chosen the resource
// name (so the SVG content stream can reference it); the indirect Pattern
// object is written later, when the enclosing page is finalised.
type pendingShading struct {
	name    pdf.Name
	pattern pdf.ShadingPattern
}

// svgShadingCollector implements svgreader.ShadingRegistrar. It accepts
// gradient fills encountered during rendering, allocates a per-page-unique
// name from the host document, and stashes the resulting Pattern definition
// for later registration on the actual pdf.Page.
type svgShadingCollector struct {
	doc       *PDFDocument
	collected []pendingShading
}

func (c *svgShadingCollector) RegisterShading(req svgreader.ShadingRequest) string {
	if req.Gradient == nil || len(req.Gradient.Stops) == 0 {
		return ""
	}
	c.doc.shadingCounter++
	name := pdf.Name(fmt.Sprintf("Sh%d", c.doc.shadingCounter))

	pattern := buildShadingPattern(req)
	c.collected = append(c.collected, pendingShading{name: name, pattern: pattern})
	return string(name)
}

// buildShadingPattern converts an SVG LinearGradient plus the current CTM
// into the pdf.ShadingPattern data type.
//
// PDF Shading Patterns map pattern space to *default* page coordinates;
// the CTM at use time is NOT additionally applied (ISO 32000-1 §8.7.3.3).
// The matrix we write here therefore has to encode the full chain from
// gradient-pattern space all the way to the page's default frame:
//
//	pattern_matrix = outerTranslation × svgInternalCTM × gradientTransform
//
// At build time we only know the svg-internal CTM (req.CTM, captured by
// the renderer at fill time). The outer wrapper translation — applied by
// boxesandglue around the SVG content stream — is layered on later by
// composePatternOuter when the page output path knows the rule's position.
func buildShadingPattern(req svgreader.ShadingRequest) pdf.ShadingPattern {
	g := req.Gradient
	patternMatrix := req.CTM.Multiply(g.GradientTransform)

	return pdf.ShadingPattern{
		Shading: pdf.Shading{
			ShadingType: 2, // axial (linear)
			ColorSpace:  "/DeviceRGB",
			Coords:      [4]float64{g.X1, g.Y1, g.X2, g.Y2},
			Function:    buildShadingFunction(g.Stops),
			Extend:      [2]bool{true, true},
		},
		Matrix: [6]float64{
			patternMatrix[0], patternMatrix[1],
			patternMatrix[2], patternMatrix[3],
			patternMatrix[4], patternMatrix[5],
		},
	}
}

// buildShadingFunction synthesises a PDF Function describing the gradient's
// stop sequence. The output handles three regimes:
//
//   - single stop → constant color (Type 2 with C0 == C1).
//   - two stops covering exactly [0,1] → single Type 2 linear interpolation.
//   - everything else → Type 3 stitching function. Pad segments (constant
//     color) are prepended when the first stop's offset > 0 or appended when
//     the last stop's offset < 1, so the gradient parameter t covers [0,1]
//     without relying on the Shading dict's Extend behaviour to produce the
//     pad regions inside the gradient line itself.
func buildShadingFunction(stops []svgreader.GradientStop) pdf.Function {
	if len(stops) == 0 {
		// Defensive — caller should already have rejected this.
		return pdf.Function{FunctionType: 2, Domain: [2]float64{0, 1}, N: 1}
	}
	if len(stops) == 1 {
		c := rgbArray(stops[0])
		return pdf.Function{
			FunctionType: 2, Domain: [2]float64{0, 1},
			C0: c, C1: c, N: 1,
		}
	}
	first := stops[0]
	last := stops[len(stops)-1]
	if len(stops) == 2 && first.Offset == 0 && last.Offset == 1 {
		return pdf.Function{
			FunctionType: 2, Domain: [2]float64{0, 1},
			C0: rgbArray(first), C1: rgbArray(last), N: 1,
		}
	}

	subs := make([]pdf.Function, 0, len(stops)+1)
	bounds := make([]float64, 0, len(stops))
	encode := make([][2]float64, 0, len(stops)+1)

	if first.Offset > 0 {
		c := rgbArray(first)
		subs = append(subs, pdf.Function{
			FunctionType: 2, Domain: [2]float64{0, 1},
			C0: c, C1: c, N: 1,
		})
		bounds = append(bounds, first.Offset)
		encode = append(encode, [2]float64{0, 1})
	}
	for i := 0; i < len(stops)-1; i++ {
		subs = append(subs, pdf.Function{
			FunctionType: 2, Domain: [2]float64{0, 1},
			C0: rgbArray(stops[i]),
			C1: rgbArray(stops[i+1]),
			N:  1,
		})
		encode = append(encode, [2]float64{0, 1})
		if i < len(stops)-2 {
			bounds = append(bounds, stops[i+1].Offset)
		}
	}
	if last.Offset < 1 {
		c := rgbArray(last)
		bounds = append(bounds, last.Offset)
		subs = append(subs, pdf.Function{
			FunctionType: 2, Domain: [2]float64{0, 1},
			C0: c, C1: c, N: 1,
		})
		encode = append(encode, [2]float64{0, 1})
	}

	if len(subs) == 1 {
		return subs[0]
	}
	return pdf.Function{
		FunctionType: 3, Domain: [2]float64{0, 1},
		SubFunctions: subs,
		Bounds:       bounds,
		Encode:       encode,
	}
}

func rgbArray(s svgreader.GradientStop) [3]float64 {
	return [3]float64{s.Color.R, s.Color.G, s.Color.B}
}

// composePatternOuter left-multiplies an additional translation onto each
// pendingShading's pattern matrix. The translation represents the cm
// operator that boxesandglue emits around rule.Pre at output time — the
// outer wrapper the SVG renderer cannot see while it is still building
// the content stream. PDF Shading Patterns ignore the CTM at use time so
// we have to encode this final hop into the pattern matrix ourselves.
//
// posX / posY are in PDF default user space (origin bottom-left, y-up),
// which matches what boxesandglue's outputContent emits with
// "1 0 0 1 posX posY cm".
func composePatternOuter(items []pendingShading, posX, posY float64) []pendingShading {
	if len(items) == 0 {
		return items
	}
	outer := svgreader.Matrix{1, 0, 0, 1, posX, posY}
	out := make([]pendingShading, len(items))
	for i, ps := range items {
		pm := svgreader.Matrix{
			ps.pattern.Matrix[0], ps.pattern.Matrix[1],
			ps.pattern.Matrix[2], ps.pattern.Matrix[3],
			ps.pattern.Matrix[4], ps.pattern.Matrix[5],
		}
		composed := outer.Multiply(pm)
		ps.pattern.Matrix = [6]float64{
			composed[0], composed[1], composed[2], composed[3], composed[4], composed[5],
		}
		out[i] = ps
	}
	return out
}

// materializeShadingsOnPage writes each pending shading as PDF objects and
// hooks them into the page's Pattern resource dict. Called from the page
// output path when a rule carries a "shadings" attribute.
func materializeShadingsOnPage(pdfPage *pdf.Page, doc *PDFDocument, items []pendingShading) error {
	if len(items) == 0 {
		return nil
	}
	if pdfPage.Patterns == nil {
		pdfPage.Patterns = make(map[pdf.Name]*pdf.Object)
	}
	for _, ps := range items {
		obj, err := doc.PDFWriter.WriteShadingPattern(ps.pattern)
		if err != nil {
			return err
		}
		pdfPage.Patterns[ps.name] = obj
	}
	return nil
}
