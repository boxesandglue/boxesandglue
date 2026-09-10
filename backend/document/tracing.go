package document

import (
	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/boxesandglue/frontend/pdfdraw"
)

// VTrace determines the type of visual tracing
type VTrace int

const (
	// VTraceImages shows bounding box of images
	VTraceImages VTrace = iota
	// VTraceVBoxes shows the bounding box of vlists
	VTraceVBoxes
	// VTraceHBoxes shows height and depth of line boxes: the area above
	// the baseline and the one below get distinct translucent tints, the
	// baseline and the outline are stroked. Only hlists built as text
	// lines (origin "line") are marked; wrapper hboxes would blanket
	// whole blocks and hide the line metrics the trace is about.
	VTraceHBoxes
	// VTraceHyperlinks shows hyperlinks
	VTraceHyperlinks
	// VTraceDest shows destinations
	VTraceDest
)

// SetVTrace sets the visual tracing
func (d *PDFDocument) SetVTrace(t VTrace) {
	d.tracing |= 1 << t
}

// ClearVTrace removes the visual tracing.
func (d *PDFDocument) ClearVTrace(t VTrace) {
	d.tracing &^= (1 << t)
}

// IsTrace returns true if tracing t is set
func (d *PDFDocument) IsTrace(t VTrace) bool {
	return (d.tracing>>t)&1 == 1
}

// Colors of the hbox trace (VTraceHBoxes): the area above the baseline
// (height) and below it (depth) get distinct translucent tints so both
// metrics are readable at a glance. Deliberately different hues from the
// box model trace palette so the overlays stay distinguishable when both
// are active.
var (
	hboxTraceHeightColor = color.Color{Space: color.ColorRGB, R: 0.635, G: 0.769, B: 0.788} // #a2c4c9
	hboxTraceDepthColor  = color.Color{Space: color.ColorRGB, R: 0.918, G: 0.6, B: 0.6}     // #ea9999
)

// hboxTraceAlpha is the constant fill alpha of the hbox trace areas.
const hboxTraceAlpha = 0.4

// isLineBox reports whether the HList is a text line produced by the
// linebreaker (origin attribute "line") that carries its own content.
// Origin alone is not enough: a finished block VList routed through
// paragraph formatting gets wrapped in a "line" hbox as tall as the whole
// block, and tracing that wrapper would blanket the block and hide the
// real line metrics. Such wrappers hold only a VList (plus glue/kern),
// so a line qualifies when it directly contains a glyph, or when it
// contains no VList at all (e.g. an image line).
func isLineBox(v *node.HList) bool {
	if v.Attributes == nil {
		return false
	}
	if origin, ok := v.Attributes["origin"].(string); !ok || origin != "line" {
		return false
	}
	hasVList := false
	for e := v.List; e != nil; e = e.Next() {
		switch e.(type) {
		case *node.Glyph:
			return true
		case *node.VList:
			hasVList = true
		}
	}
	return !hasVList
}

// hboxTraceRule builds the hidden rule that visualises an HList's vertical
// metrics: height tinted above the baseline, depth below, the baseline
// itself as a line and the box outline stroked. The rule is inserted as the
// line's first child, so its origin sits at the line start on the baseline.
// When transparent is false (the claimed conformance forbids transparency,
// see Format.AllowsTransparency) only outline and baseline are drawn.
func hboxTraceRule(v *node.HList, transparent bool) *node.Rule {
	r := node.NewRule()
	r.Hide = true
	p := pdfdraw.NewStandalone()
	if transparent {
		alpha := float64(hboxTraceAlpha)
		gs := pdf.ExtGState{FillAlpha: &alpha}
		p.GState(string(gs.ResourceName()))
		if v.Height > 0 {
			p.ColorNonstroking(hboxTraceHeightColor).Rect(0, 0, v.Width, v.Height).Fill()
		}
		if v.Depth > 0 {
			p.ColorNonstroking(hboxTraceDepthColor).Rect(0, -v.Depth, v.Width, v.Depth).Fill()
		}
		r.Attributes = node.H{
			"origin":     "hbox trace",
			"extgstates": []pdf.ExtGState{gs},
		}
	} else {
		r.Attributes = node.H{"origin": "hbox trace"}
	}
	p.LineWidth(bag.MustSP("0.4pt")).Rect(0, -v.Depth, v.Width, v.Height+v.Depth).Stroke()
	p.Moveto(0, 0).Lineto(v.Width, 0).Stroke()
	r.Pre = "q " + p.String() + " Q"
	return r
}
