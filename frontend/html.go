package frontend

import (
	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/color"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/frontend/pdfdraw"
)

// BorderStyle represents the HTML border styles such as solid, dashed, ...
type BorderStyle uint

const (
	// BorderStyleNone is no border
	BorderStyleNone BorderStyle = iota
	// BorderStyleSolid is a solid line
	BorderStyleSolid
)

// HTMLProperties contains css values
type HTMLProperties map[string]string

// HTMLValues contains margin, padding and border values for a rectangular area.
type HTMLValues struct {
	BackgroundColor         *color.Color
	BorderTopWidth          bag.ScaledPoint
	BorderRightWidth        bag.ScaledPoint
	BorderBottomWidth       bag.ScaledPoint
	BorderLeftWidth         bag.ScaledPoint
	BorderTopLeftRadius     bag.ScaledPoint
	BorderTopRightRadius    bag.ScaledPoint
	BorderBottomLeftRadius  bag.ScaledPoint
	BorderBottomRightRadius bag.ScaledPoint
	BorderTopColor          *color.Color
	BorderRightColor        *color.Color
	BorderBottomColor       *color.Color
	BorderLeftColor         *color.Color
	BorderTopStyle          BorderStyle
	BorderRightStyle        BorderStyle
	BorderBottomStyle       BorderStyle
	BorderLeftStyle         BorderStyle
	MarginTop               bag.ScaledPoint
	MarginRight             bag.ScaledPoint
	MarginBottom            bag.ScaledPoint
	MarginLeft              bag.ScaledPoint
	PaddingTop              bag.ScaledPoint
	PaddingRight            bag.ScaledPoint
	PaddingBottom           bag.ScaledPoint
	PaddingLeft             bag.ScaledPoint
}

func (hv HTMLValues) hasBorder() bool {
	return hv.BorderTopWidth > 0 && hv.BorderTopStyle != BorderStyleNone ||
		hv.BorderLeftWidth > 0 && hv.BorderLeftStyle != BorderStyleNone ||
		hv.BorderBottomWidth > 0 && hv.BorderBottomStyle != BorderStyleNone ||
		hv.BorderRightWidth > 0 && hv.BorderRightStyle != BorderStyleNone
}

func (d *Document) SettingsToValues(s TypesettingSettings) HTMLValues {
	hv := HTMLValues{}
	if c, ok := s[SettingBackgroundColor]; ok {
		hv.BackgroundColor = c.(*color.Color)
	}
	if bw, ok := s[SettingBorderTopWidth]; ok {
		hv.BorderTopWidth = bw.(bag.ScaledPoint)
	}
	if bw, ok := s[SettingBorderBottomWidth]; ok {
		hv.BorderBottomWidth = bw.(bag.ScaledPoint)
	}
	if bw, ok := s[SettingBorderLeftWidth]; ok {
		hv.BorderLeftWidth = bw.(bag.ScaledPoint)
	}
	if bw, ok := s[SettingBorderRightWidth]; ok {
		hv.BorderRightWidth = bw.(bag.ScaledPoint)
	}
	if bw, ok := s[SettingBorderTopLeftRadius]; ok {
		hv.BorderTopLeftRadius = bw.(bag.ScaledPoint)
	}
	if wd, ok := s[SettingMarginTop]; ok {
		hv.MarginTop = wd.(bag.ScaledPoint)
	}
	if wd, ok := s[SettingMarginBottom]; ok {
		hv.MarginBottom = wd.(bag.ScaledPoint)
	}
	if wd, ok := s[SettingMarginLeft]; ok {
		hv.MarginLeft = wd.(bag.ScaledPoint)
	}
	if wd, ok := s[SettingMarginRight]; ok {
		hv.MarginRight = wd.(bag.ScaledPoint)
	}
	if wd, ok := s[SettingPaddingTop]; ok {
		hv.PaddingTop = wd.(bag.ScaledPoint)
		delete(s, SettingPaddingTop)
	}
	if wd, ok := s[SettingPaddingBottom]; ok {
		hv.PaddingBottom = wd.(bag.ScaledPoint)
		delete(s, SettingPaddingBottom)
	}
	if wd, ok := s[SettingPaddingLeft]; ok {
		hv.PaddingLeft = wd.(bag.ScaledPoint)
		delete(s, SettingPaddingLeft)
	}
	if wd, ok := s[SettingPaddingRight]; ok {
		hv.PaddingRight = wd.(bag.ScaledPoint)
		delete(s, SettingPaddingRight)
	}
	if bw, ok := s[SettingBorderTopLeftRadius]; ok {
		hv.BorderTopLeftRadius = bw.(bag.ScaledPoint)
	}
	if bw, ok := s[SettingBorderTopRightRadius]; ok {
		hv.BorderTopRightRadius = bw.(bag.ScaledPoint)
	}
	if bw, ok := s[SettingBorderBottomLeftRadius]; ok {
		hv.BorderBottomLeftRadius = bw.(bag.ScaledPoint)
	}
	if bw, ok := s[SettingBorderBottomRightRadius]; ok {
		hv.BorderBottomRightRadius = bw.(bag.ScaledPoint)
	}
	if col, ok := s[SettingBorderRightColor]; ok {
		hv.BorderRightColor = col.(*color.Color)
	}
	if col, ok := s[SettingBorderLeftColor]; ok {
		hv.BorderLeftColor = col.(*color.Color)
	}
	if col, ok := s[SettingBorderTopColor]; ok {
		hv.BorderTopColor = col.(*color.Color)
	}
	if col, ok := s[SettingBorderBottomColor]; ok {
		hv.BorderBottomColor = col.(*color.Color)
	}
	if sty, ok := s[SettingBorderRightStyle]; ok {
		hv.BorderRightStyle = sty.(BorderStyle)
	}
	if sty, ok := s[SettingBorderLeftStyle]; ok {
		hv.BorderLeftStyle = sty.(BorderStyle)
	}
	if sty, ok := s[SettingBorderTopStyle]; ok {
		hv.BorderTopStyle = sty.(BorderStyle)
	}
	if sty, ok := s[SettingBorderBottomStyle]; ok {
		hv.BorderBottomStyle = sty.(BorderStyle)
	}
	return hv
}

// CSSPropertiesToValues converts CSS values to the HTMLValues struct.
func (d *Document) CSSPropertiesToValues(p HTMLProperties) HTMLValues {
	hv := HTMLValues{}
	for k, v := range p {
		switch k {
		case "background-color":
			hv.BackgroundColor = d.GetColor(v)
		case "border-top-width":
			hv.BorderTopWidth = bag.MustSp(v)
		case "border-right-width":
			hv.BorderRightWidth = bag.MustSp(v)
		case "border-bottom-width":
			hv.BorderBottomWidth = bag.MustSp(v)
		case "border-left-width":
			hv.BorderLeftWidth = bag.MustSp(v)
		case "border-top-left-radius":
			hv.BorderTopLeftRadius = bag.MustSp(v)
		case "border-top-right-radius":
			hv.BorderTopRightRadius = bag.MustSp(v)
		case "border-bottom-left-radius":
			hv.BorderBottomLeftRadius = bag.MustSp(v)
		case "border-bottom-right-radius":
			hv.BorderBottomRightRadius = bag.MustSp(v)
		case "border-top-color":
			hv.BorderTopColor = d.GetColor(v)
		case "border-right-color":
			hv.BorderRightColor = d.GetColor(v)
		case "border-bottom-color":
			hv.BorderBottomColor = d.GetColor(v)
		case "border-left-color":
			hv.BorderLeftColor = d.GetColor(v)
		case "border-top-style", "border-right-style", "border-bottom-style", "border-left-style":
			var sty BorderStyle
			switch v {
			case "none":
				sty = BorderStyleNone
			default:
				sty = BorderStyleSolid
			}
			switch k {
			case "border-top-style":
				hv.BorderTopStyle = sty
			case "border-right-style":
				hv.BorderRightStyle = sty
			case "border-bottom-style":
				hv.BorderBottomStyle = sty
			case "border-left-style":
				hv.BorderLeftStyle = sty
			}
		default:
			// fmt.Println("unresolved attribute", k, v)
		}
	}
	return hv
}

const circleBezier = 0.551915024494

func getBorderPaths(x0, y0, x1, y1, x2, y2, x3, y3 bag.ScaledPoint, hv HTMLValues) (inner, outer *pdfdraw.Object) {
	// When radii are added, we need to add control points. These points are
	// numbered from 1 to 16 counterclockwise and start with the bottom left point.
	// See https://blog.speedata.de/2020/06/22/borderdots.png for a visualization.

	// Now two clip paths are created, an outer one and an inner path
	// o = outer
	ox1 := x0 + hv.BorderBottomLeftRadius
	ox2 := x3 - hv.BorderBottomRightRadius
	ox3 := ox2 + bag.MultiplyFloat(hv.BorderBottomRightRadius, circleBezier)
	ox4, ox5, ox6, ox7 := x3, x3, x3, x3
	ox9 := x3 - hv.BorderTopRightRadius
	ox8 := ox9 + bag.MultiplyFloat(hv.BorderTopRightRadius, circleBezier)
	ox10 := x0 + hv.BorderTopLeftRadius
	ox11 := ox10 - bag.MultiplyFloat(hv.BorderTopLeftRadius, circleBezier)
	ox12, ox13, ox14, ox15 := x0, x0, x0, x0
	ox16 := ox1 - bag.MultiplyFloat(hv.BorderBottomLeftRadius, circleBezier)

	oy1, oy2, oy3 := y3, y3, y3
	oy5 := oy1 + hv.BorderBottomRightRadius
	oy4 := oy5 - bag.MultiplyFloat(hv.BorderBottomRightRadius, circleBezier)
	oy6 := y0 - hv.BorderTopRightRadius
	oy7 := oy6 + bag.MultiplyFloat(hv.BorderTopRightRadius, circleBezier)
	oy8, oy9, oy10, oy11 := y0, y0, y0, y0
	oy13 := y0 - hv.BorderTopLeftRadius
	oy12 := oy13 + bag.MultiplyFloat(hv.BorderTopLeftRadius, circleBezier)
	oy14 := oy1 + hv.BorderBottomLeftRadius
	oy15 := oy14 - bag.MultiplyFloat(hv.BorderBottomLeftRadius, circleBezier)
	oy16 := oy1

	innerBorderBottomLeftRadiusX := bag.Max(0, hv.BorderBottomLeftRadius-hv.BorderLeftWidth)
	innerBorderBottomLeftRadiusY := bag.Max(0, hv.BorderBottomLeftRadius-hv.BorderLeftWidth)
	innerBorderTopLeftRadiusX := bag.Max(0, hv.BorderTopLeftRadius-hv.BorderLeftWidth)
	innerBorderTopLeftRadiusY := bag.Max(0, hv.BorderTopLeftRadius-hv.BorderLeftWidth)
	innerBorderTopRightRadiusX := bag.Max(0, hv.BorderTopRightRadius-hv.BorderRightWidth)
	innerBorderTopRightRadiusY := bag.Max(0, hv.BorderTopRightRadius-hv.BorderRightWidth)
	innerBorderBottomRightRadiusX := bag.Max(0, hv.BorderBottomRightRadius-hv.BorderRightWidth)
	innerBorderBottomRightRadiusY := bag.Max(0, hv.BorderBottomRightRadius-hv.BorderRightWidth)

	//  i = inner
	ix1 := bag.Max(ox1, x0+hv.BorderLeftWidth)
	ix2 := bag.Min(ox2, x3-hv.BorderRightWidth)
	ix3 := ix2 + bag.MultiplyFloat(innerBorderBottomRightRadiusX, circleBezier)
	ix4 := x3 - hv.BorderRightWidth
	ix5, ix6, ix7 := ix4, ix4, ix4
	ix9 := bag.Min(ox9, x3-hv.BorderRightWidth)
	ix8 := ix9 + bag.MultiplyFloat(innerBorderTopRightRadiusX, circleBezier)
	ix10 := bag.Max(x0+hv.BorderLeftWidth, ox10)
	ix11 := ix10 - bag.MultiplyFloat(innerBorderTopLeftRadiusX, circleBezier)
	ix13 := bag.Max(ox13+hv.BorderLeftWidth, ox13)
	ix12 := x0 + hv.BorderLeftWidth
	ix13, ix14, ix15 := ix12, ix12, ix12
	ix16 := ix1 - bag.MultiplyFloat(innerBorderBottomLeftRadiusX, circleBezier)

	iy1 := oy1 + hv.BorderBottomWidth
	iy2, iy3 := iy1, iy1
	iy5 := bag.Max(oy5, y3+hv.BorderBottomWidth)
	iy4 := iy5 - bag.MultiplyFloat(innerBorderBottomRightRadiusY, circleBezier)
	iy6 := bag.Min(oy6, y0-hv.BorderTopWidth)
	iy7 := iy6 + bag.MultiplyFloat(innerBorderTopRightRadiusY, circleBezier)
	iy9 := bag.Min(oy9, y0-hv.BorderTopWidth)
	iy8 := iy9
	iy10, iy11 := iy8, iy8
	iy13 := bag.Min(y0-hv.BorderTopWidth, oy13)
	iy12 := iy13 + bag.MultiplyFloat(innerBorderTopLeftRadiusY, circleBezier)
	iy14 := bag.Max(y3+hv.BorderBottomWidth, oy14)
	iy15 := iy14 - bag.MultiplyFloat(innerBorderBottomLeftRadiusY, circleBezier)
	iy16 := iy1

	outer = pdfdraw.New()
	inner = pdfdraw.New()
	outer.Moveto(ox1, oy1).
		Lineto(ox2, oy2).
		Curveto(ox3, oy3, ox4, oy4, ox5, oy5).
		Lineto(ox6, oy6).
		Curveto(ox7, oy7, ox8, oy8, ox9, oy9).
		Lineto(ox10, oy10).
		Curveto(ox11, oy11, ox12, oy12, ox13, oy13).
		Lineto(ox14, oy14).
		Curveto(ox15, oy15, ox16, oy16, ox1, oy1)

	inner.Moveto(ix1, iy1).
		Curveto(ix16, iy16, ix15, iy15, ix14, iy14).
		Lineto(ix13, iy13).
		Curveto(ix12, iy12, ix11, iy11, ix10, iy10).
		Lineto(ix9, iy9).
		Curveto(ix8, iy8, ix7, iy7, ix6, iy6).
		Lineto(ix5, iy5).
		Curveto(ix4, iy4, ix3, iy3, ix2, iy2)

	return
}

// HTMLBorder returns two string with a HTML border. The first string is part of
// a prefix for a possible background string and the second string renders the
// border.
func (d *Document) HTMLBorder(vl *node.VList, hv HTMLValues) *node.VList {
	width := vl.Width
	height := vl.Height
	black := d.GetColor("black")
	if hv.BorderTopColor == nil {
		hv.BorderTopColor = black
	}
	if hv.BorderRightColor == nil {
		hv.BorderRightColor = black
	}
	if hv.BorderBottomColor == nil {
		hv.BorderBottomColor = black
	}
	if hv.BorderLeftColor == nil {
		hv.BorderLeftColor = black
	}

	// We start with 4 trapezoids (1 for each border).
	//
	//      4    4------------------------------3   3  y0
	//      |\    \                            /   /|
	//      | \    \                          /   / |
	//      |  \    \                        /   /  |
	//      |   \    \                      /   /   |
	//      |    \    \                    /   /    |
	//      |     3    1------------------2   4     |  y1
	//      |     |                           |     |
	//      |     |                           |     |
	//      |     |                           |     |
	//      |     |                           |     |
	//      |     |                           |     |
	//      |    2    4--------------------3   1    |  y2
	//      |   /    /                      \   \   |
	//      |  /    /                        \   \  |
	//      | /    /                          \   \ |
	//      |/    /                            \   \|
	//      1    /                              \   2  y3
	//          1--------------------------------2
	//     x0    x1                         x2    x3

	// does not look correct, but for now it is fine.
	maxTrapezoidThickness := bag.Min(width, height+vl.Depth) / 2
	x0 := 0 - width - hv.PaddingLeft - hv.PaddingRight - hv.BorderLeftWidth - hv.BorderRightWidth
	x1 := x0 + hv.BorderLeftWidth + maxTrapezoidThickness
	x2 := 0 - hv.BorderRightWidth - maxTrapezoidThickness
	x3 := bag.ScaledPoint(0)

	y0 := height + hv.PaddingTop + hv.BorderTopWidth
	y1 := y0 - maxTrapezoidThickness - hv.BorderTopWidth
	y3 := bag.ScaledPoint(0) - hv.PaddingBottom - hv.BorderBottomWidth - vl.Depth
	y2 := y3 + maxTrapezoidThickness + hv.BorderBottomWidth
	// for the background, we need x-coordinates looking from the left border
	// and y-coordinates from top to bottom
	xbg0 := 0 - hv.BorderLeftWidth - hv.PaddingLeft
	xbg1 := xbg0 + hv.BorderLeftWidth + hv.PaddingLeft
	xbg2 := xbg1 + width
	xbg3 := xbg2 + hv.BorderRightWidth + hv.PaddingRight

	ybg0 := bag.ScaledPoint(0) + hv.PaddingTop + hv.BorderTopWidth
	ybg1 := ybg0 - maxTrapezoidThickness
	ybg3 := ybg0 - height - hv.PaddingTop - hv.BorderTopWidth - hv.PaddingBottom - hv.BorderBottomWidth
	ybg2 := ybg3 + maxTrapezoidThickness

	if hv.BackgroundColor != nil && hv.BackgroundColor.Space != color.ColorNone {
		// this is the rule node for the background
		rbg := node.NewRule()
		rbg.Hide = true
		var innerBG *pdfdraw.Object
		innerBG, _ = getBorderPaths(xbg0, ybg0, xbg1, ybg1, xbg2, ybg2, xbg3, ybg3, hv)
		innerBG.Clip().Endpath()
		innerBG.ColorNonstroking(*hv.BackgroundColor).Rect(xbg0, ybg3, xbg3-xbg0, ybg0-ybg3).Fill()
		rbg.Pre = "q " + innerBG.String() + " Q"
		rbg.Attributes = node.H{"origin": "html background color"}
		vl.List = node.InsertBefore(vl.List, vl.List, rbg)
	}

	lgWd := hv.PaddingLeft + hv.BorderLeftWidth
	rgWd := hv.PaddingRight + hv.BorderRightWidth
	tgWd := hv.PaddingTop + hv.BorderTopWidth
	bgWd := hv.PaddingBottom + hv.BorderBottomWidth

	if lgWd == 0 && rgWd == 0 && tgWd == 0 && bgWd == 0 {
		return vl
	}

	var head, tail node.Node
	head = vl
	tail = vl
	if lgWd != 0 {
		paddingLeftGlue := node.NewGlue()
		paddingLeftGlue.Width = lgWd
		paddingLeftGlue.Attributes = node.H{"origin": "paddingLeft + borderLeft"}
		head = node.InsertBefore(head, vl, paddingLeftGlue)
		tail = vl
	}
	if rgWd != 0 {
		paddingRightGlue := node.NewGlue()
		paddingRightGlue.Attributes = node.H{"origin": "paddingRight + borderRight"}
		paddingRightGlue.Width = rgWd
		head = node.InsertAfter(head, tail, paddingRightGlue)
		tail = paddingRightGlue
	}

	if hv.hasBorder() {
		// this is the border rule node
		r := node.NewRule()
		r.Attributes = node.H{"origin": "html border + clipping"}
		r.Hide = true

		inner, outer := getBorderPaths(x0, y0, x1, y1, x2, y2, x3, y3, hv)
		inner.Clip().Endpath()
		// for debugging:
		// inner.Stroke().Endpath()

		// Draw the four trapezoids
		inner.ColorNonstroking(*hv.BorderTopColor).Moveto(x0, y0).Lineto(x1, y1).Lineto(x2, y1).Lineto(x3, y0).Close().Fill()
		inner.ColorNonstroking(*hv.BorderLeftColor).Moveto(x0, y3).Lineto(x1, y2).Lineto(x1, y1).Lineto(x0, y0).Close().Fill()
		inner.ColorNonstroking(*hv.BorderBottomColor).Moveto(x0, y3).Lineto(x3, y3).Lineto(x2, y2).Lineto(x1, y2).Close().Fill()
		inner.ColorNonstroking(*hv.BorderRightColor).Moveto(x2, y2).Lineto(x3, y3).Lineto(x3, y0).Lineto(x2, y1).Close().Fill()

		r.Pre = "q " + outer.String() + " " + inner.String() + " Q"
		head = node.InsertAfter(head, tail, r)
	}

	hl := node.Hpack(head)
	hl.Attributes = node.H{"origin": "hpack padding"}
	head = hl
	tail = hl
	if tgWd != 0 {
		paddingTopGlue := node.NewGlue()
		paddingTopGlue.Width = tgWd
		paddingTopGlue.Attributes = node.H{"origin": "html border top glue"}
		head = node.InsertBefore(head, hl, paddingTopGlue)
	}
	if bgWd != 0 {
		paddingBottomGlue := node.NewGlue()
		paddingBottomGlue.Width = bgWd
		paddingBottomGlue.Attributes = node.H{"origin": "html border bottom glue"}
		head = node.InsertAfter(head, tail, paddingBottomGlue)
	}

	vl = node.Vpack(head)
	vl.Attributes = node.H{"origin": "vpack padding"}

	return vl
}
