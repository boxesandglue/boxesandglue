package document

import (
	"fmt"
	"strings"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
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

// svgResources are the PDF resources of a content stream that an SVG produced.
// Like the fonts of a rule they wait for the page output: a node can be built
// several times while a table is measured, and only one of them gets shipped.
type svgResources struct {
	shadings []pendingShading
	gstates  []pdf.ExtGState
	forms    []*svgForm
	masks    []*svgMask
}

// svgMask is the soft mask of a gradient whose stops differ in opacity. Its
// name is the one of the graphics state: the name that the document derives
// from the contents needs the object number of the form, which does not exist
// while the SVG is rendered.
type svgMask struct {
	name     pdf.Name
	gradient *svgreader.Gradient
	matrix   svgreader.Matrix
	opacity  float64
	bbox     [4]float64
}

// svgForm is a group of an SVG with an opacity, which ends up as a Form XObject
// with a transparency group.
type svgForm struct {
	svgResources
	name    pdf.Name
	content string
	bbox    [4]float64
	faces   []*pdf.Face
}

// svgShadingCollector implements svgreader.ShadingRegistrar and
// svgreader.OpacityRegistrar. It accepts gradient fills, alpha values and
// groups encountered during rendering, allocates per-page-unique names from
// the host document, and stashes the resulting definitions for later
// registration on the actual pdf.Page or on the form they are used in.
type svgShadingCollector struct {
	doc  *PDFDocument
	root svgResources
	open []*svgForm // the groups being rendered
}

// current returns the resources of the content stream being rendered.
func (c *svgShadingCollector) current() *svgResources {
	if n := len(c.open); n > 0 {
		return &c.open[n-1].svgResources
	}
	return &c.root
}

// resolveColor hands a color defined in the document to the renderer of the
// SVG. The renderer takes care of the alpha value of the color like of any other
// one. Spot colors are in every resource dictionary, see materializeSVGForms for
// overprinting.
func (c *svgShadingCollector) resolveColor(value string) (svgreader.HostColor, bool) {
	if c.doc.DefinedColor == nil {
		return svgreader.HostColor{}, false
	}
	col := c.doc.DefinedColor(value)
	if col == nil {
		return svgreader.HostColor{}, false
	}
	alpha := col.Alpha()
	opaque := *col
	opaque.Transparency = 0
	return svgreader.HostColor{
		Fill:   strings.TrimSpace(opaque.PDFStringNonStroking()),
		Stroke: strings.TrimSpace(opaque.PDFStringStroking()),
		Alpha:  alpha,
	}, true
}

// allowsTransparency reports whether the format allows transparency and warns
// once if not. The drawing is painted opaque then.
func (c *svgShadingCollector) allowsTransparency() bool {
	if c.doc.Format.AllowsTransparency() {
		return true
	}
	if !c.doc.warnedSVGTransparency {
		c.doc.warnedSVGTransparency = true
		bag.Logger.Warn("The PDF format does not allow transparency, painting SVG opaque")
	}
	return false
}

// RegisterAlpha implements svgreader.OpacityRegistrar.
func (c *svgShadingCollector) RegisterAlpha(fill, stroke float64) string {
	if !c.allowsTransparency() {
		return ""
	}
	var gs pdf.ExtGState
	if fill != 1 {
		gs.FillAlpha = &fill
	}
	if stroke != 1 {
		gs.StrokeAlpha = &stroke
	}
	cur := c.current()
	cur.gstates = append(cur.gstates, gs)
	return string(gs.ResourceName())
}

// BeginGroup implements svgreader.OpacityRegistrar.
func (c *svgShadingCollector) BeginGroup() bool {
	if !c.allowsTransparency() {
		return false
	}
	c.open = append(c.open, &svgForm{})
	return true
}

// EndGroup implements svgreader.OpacityRegistrar.
func (c *svgShadingCollector) EndGroup(content string, bbox [4]float64) string {
	form := c.open[len(c.open)-1]
	c.open = c.open[:len(c.open)-1]
	c.doc.svgFormCounter++
	form.name = pdf.Name(fmt.Sprintf("FmSVG%d", c.doc.svgFormCounter))
	form.content = content
	form.bbox = bbox
	cur := c.current()
	cur.forms = append(cur.forms, form)
	return string(form.name)
}

// RegisterGradientMask implements svgreader.OpacityRegistrar.
func (c *svgShadingCollector) RegisterGradientMask(mask svgreader.GradientMask) string {
	if !c.allowsTransparency() {
		return ""
	}
	c.doc.svgMaskCounter++
	item := &svgMask{
		name:     pdf.Name(fmt.Sprintf("GSMask%d", c.doc.svgMaskCounter)),
		gradient: mask.Gradient,
		matrix:   mask.Matrix,
		opacity:  mask.Opacity,
		bbox:     mask.BBox,
	}
	cur := c.current()
	cur.masks = append(cur.masks, item)
	return string(item.name)
}

// setFaces gives each form the fonts of the drawing. A form has no access to
// the resources of its page, and the text renderer does not know about groups.
func setFaces(forms []*svgForm, faces []*pdf.Face) {
	for _, form := range forms {
		form.faces = faces
		setFaces(form.forms, faces)
	}
}

// materializeSVGForms writes the forms and hooks them into res. A form is
// written once, however often its rule is shipped out.
func materializeSVGForms(res *pdf.Resources, doc *PDFDocument, items []*svgForm) error {
	if len(items) == 0 {
		return nil
	}
	if res.Forms == nil {
		res.Forms = make(map[pdf.Name]*pdf.Form)
	}
	for _, item := range items {
		form, ok := doc.svgForms[item]
		if !ok {
			content := doc.PDFWriter.NewObject()
			content.SetCompression(doc.CompressLevel)
			content.Data.WriteString(item.content)
			form = doc.PDFWriter.AddForm(content, item.bbox)
			form.Group = &pdf.TransparencyGroup{Isolated: true}
			form.Faces = item.faces
			if err := materializeShadings(&form.Resources, doc, item.shadings); err != nil {
				return err
			}
			if err := materializeExtGStates(&form.Resources, doc, item.gstates); err != nil {
				return err
			}
			// A page has the graphics state of overprinting by itself, a
			// form has no access to it.
			if doc.Overprint {
				if err := materializeExtGStates(&form.Resources, doc, []pdf.ExtGState{pdf.OverprintOn}); err != nil {
					return err
				}
			}
			if err := materializeSVGForms(&form.Resources, doc, item.forms); err != nil {
				return err
			}
			if err := materializeSVGMasks(&form.Resources, doc, item.masks); err != nil {
				return err
			}
			if doc.svgForms == nil {
				doc.svgForms = make(map[*svgForm]*pdf.Form)
			}
			doc.svgForms[item] = form
		}
		res.Forms[item.name] = form
	}
	return nil
}

func (c *svgShadingCollector) RegisterShading(req svgreader.ShadingRequest) string {
	if req.Gradient == nil || len(req.Gradient.Stops) == 0 {
		return ""
	}
	c.doc.shadingCounter++
	name := pdf.Name(fmt.Sprintf("Sh%d", c.doc.shadingCounter))

	pattern := buildShadingPattern(req)
	cur := c.current()
	cur.shadings = append(cur.shadings, pendingShading{name: name, pattern: pattern})
	return string(name)
}

// materializeSVGMasks writes the soft masks and hooks their graphics states
// into res. A mask is a form with a gray gradient of the stop opacities, whose
// pattern matrix holds only the transformation of the gradient: the
// coordinates of a soft mask are those of the place that sets it.
func materializeSVGMasks(res *pdf.Resources, doc *PDFDocument, items []*svgMask) error {
	if len(items) == 0 {
		return nil
	}
	if res.ExtGStates == nil {
		res.ExtGStates = make(map[pdf.Name]*pdf.Object)
	}
	for _, item := range items {
		obj, ok := doc.svgMasks[item]
		if !ok {
			g := item.gradient
			m := item.matrix.Multiply(g.GradientTransform)
			shadingType, coords := g.Shading()
			gray := func(s svgreader.GradientStop) []float64 {
				return []float64{s.StopOpacity * item.opacity}
			}
			pattern, err := doc.PDFWriter.WriteShadingPattern(pdf.ShadingPattern{
				Shading: pdf.Shading{
					ShadingType: shadingType,
					ColorSpace:  "/DeviceGray",
					Coords:      coords,
					Function:    buildShadingFunction(g.Stops, gray),
					Extend:      [2]bool{true, true},
				},
				Matrix: [6]float64{m[0], m[1], m[2], m[3], m[4], m[5]},
			})
			if err != nil {
				return err
			}
			content := doc.PDFWriter.NewObject()
			content.SetCompression(doc.CompressLevel)
			fmt.Fprintf(content.Data, "/Pattern cs /Sh scn %f %f %f %f re f",
				item.bbox[0], item.bbox[1], item.bbox[2]-item.bbox[0], item.bbox[3]-item.bbox[1])
			form := doc.PDFWriter.AddForm(content, item.bbox)
			form.Group = &pdf.TransparencyGroup{ColorSpace: "/DeviceGray", Isolated: true}
			form.Patterns = map[pdf.Name]*pdf.Object{"Sh": pattern}
			if obj, err = doc.PDFWriter.WriteExtGState(pdf.ExtGState{SoftMask: &pdf.SoftMask{Group: form}}); err != nil {
				return err
			}
			if doc.svgMasks == nil {
				doc.svgMasks = make(map[*svgMask]*pdf.Object)
			}
			doc.svgMasks[item] = obj
		}
		res.ExtGStates[item.name] = obj
	}
	return nil
}

// buildShadingPattern converts an SVG gradient plus the current CTM
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
	shadingType, coords := g.Shading()

	return pdf.ShadingPattern{
		Shading: pdf.Shading{
			ShadingType: shadingType,
			ColorSpace:  "/DeviceRGB",
			Coords:      coords,
			Function:    buildShadingFunction(g.Stops, rgbArray),
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
func buildShadingFunction(stops []svgreader.GradientStop, color func(svgreader.GradientStop) []float64) pdf.Function {
	if len(stops) == 0 {
		// Defensive — caller should already have rejected this.
		c := color(svgreader.GradientStop{})
		return pdf.Function{FunctionType: 2, Domain: [2]float64{0, 1}, C0: c, C1: c, N: 1}
	}
	if len(stops) == 1 {
		c := color(stops[0])
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
			C0: color(first), C1: color(last), N: 1,
		}
	}

	subs := make([]pdf.Function, 0, len(stops)+1)
	bounds := make([]float64, 0, len(stops))
	encode := make([][2]float64, 0, len(stops)+1)

	if first.Offset > 0 {
		c := color(first)
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
			C0: color(stops[i]),
			C1: color(stops[i+1]),
			N:  1,
		})
		encode = append(encode, [2]float64{0, 1})
		if i < len(stops)-2 {
			bounds = append(bounds, stops[i+1].Offset)
		}
	}
	if last.Offset < 1 {
		c := color(last)
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

func rgbArray(s svgreader.GradientStop) []float64 {
	return []float64{s.Color.R, s.Color.G, s.Color.B}
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

// materializeShadings writes each pending shading as PDF objects and hooks
// them into the Pattern resource dict of a page or a form. Called from the page
// output path when a rule carries a "shadings" attribute.
func materializeShadings(res *pdf.Resources, doc *PDFDocument, items []pendingShading) error {
	if len(items) == 0 {
		return nil
	}
	// A shading is written now and has to name its color space, which
	// Finish writes much later. See there.
	if doc.Format.NeedsColorProfile() && doc.PDFWriter.DefaultRGB == 0 {
		doc.PDFWriter.DefaultRGB = doc.PDFWriter.NextObject()
	}
	if res.Patterns == nil {
		res.Patterns = make(map[pdf.Name]*pdf.Object)
	}
	for _, ps := range items {
		obj, err := doc.PDFWriter.WriteShadingPattern(ps.pattern)
		if err != nil {
			return err
		}
		res.Patterns[ps.name] = obj
	}
	return nil
}
