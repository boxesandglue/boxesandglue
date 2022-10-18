package frontend

import (
	"math"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/color"
	"github.com/speedata/boxesandglue/frontend/pdfdraw"
)

// HTMLProperties contains css values
type HTMLProperties map[string]string

// HTMLValues contains margin, padding and border values for a rectangular area.
type HTMLValues struct {
	BorderTopWidth          bag.ScaledPoint
	BorderRightWidth        bag.ScaledPoint
	BorderBottomWidth       bag.ScaledPoint
	BorderLeftWidth         bag.ScaledPoint
	BorderTopLeftRadius     bag.ScaledPoint
	BorderTopRightRadius    bag.ScaledPoint
	BorderBottomLeftRadius  bag.ScaledPoint
	BorderBottomRightRadius bag.ScaledPoint
	MarginTop               bag.ScaledPoint
	MarginRight             bag.ScaledPoint
	MarginBottom            bag.ScaledPoint
	MarginLeft              bag.ScaledPoint
	BorderTopColor          *color.Color
	BorderRightColor        *color.Color
	BorderBottomColor       *color.Color
	BorderLeftColor         *color.Color
}

// HTMLPropertiesToValues converts CSS values to the HTMLValues struct.
func (d *Document) HTMLPropertiesToValues(p HTMLProperties) HTMLValues {
	hv := HTMLValues{}
	for k, v := range p {
		switch k {
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
		}
	}
	return hv
}

// HTMLBorder returns two string with a HTML border. The first string is part of
// a prefix for a possible background string and the second string renders the
// border.
func (d *Document) HTMLBorder(width bag.ScaledPoint, height bag.ScaledPoint, depth bag.ScaledPoint, properties HTMLProperties) (string, string) {
	hv := d.HTMLPropertiesToValues(properties)
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
	circleBezier := 0.551915024494

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
	//      x0      x1                       x2     x3
	x0, y0 := hv.MarginLeft, bag.ScaledPoint(0)
	x1, y1 := x0+hv.BorderLeftWidth, y0-hv.BorderTopWidth
	x2, y2 := width-hv.BorderRightWidth, -height+hv.BorderBottomWidth
	x3, y3 := width, -height

	// Now two clip paths are created, an outer one and an inner path

	// o = outer
	ox1 := x0 + hv.BorderBottomLeftRadius
	ox2 := width - hv.BorderBottomRightRadius
	ox3 := bag.ScaledPointFromFloat(ox2.ToPT() + circleBezier*hv.BorderBottomRightRadius.ToPT())
	ox4, ox5, ox6 := width, width, width
	ox8 := bag.ScaledPointFromFloat(width.ToPT() - circleBezier*hv.BorderTopRightRadius.ToPT())
	ox9 := width - hv.BorderTopRightRadius
	ox10 := hv.BorderTopLeftRadius
	ox11 := bag.ScaledPointFromFloat(ox10.ToPT() - circleBezier*hv.BorderTopLeftRadius.ToPT())
	ox12, ox13, ox14, ox15 := x0, x0, x0, x0
	ox16 := bag.ScaledPointFromFloat(ox1.ToPT() - circleBezier*hv.BorderBottomLeftRadius.ToPT())

	oy0 := -height
	oy1, oy2, oy3 := oy0, oy0, oy0
	oy5 := oy1 + hv.BorderBottomRightRadius
	oy4 := bag.ScaledPointFromFloat(oy5.ToPT() - circleBezier*hv.BorderBottomRightRadius.ToPT())
	oy6 := 0 - hv.BorderTopRightRadius
	oy7 := bag.ScaledPointFromFloat(oy6.ToPT() + circleBezier*hv.BorderTopRightRadius.ToPT())
	oy8 := bag.ScaledPoint(0)
	oy9, oy10, oy11 := oy8, oy8, oy8
	oy13 := 0 - hv.BorderTopLeftRadius
	oy12 := bag.ScaledPointFromFloat(oy13.ToPT() + circleBezier*hv.BorderTopLeftRadius.ToPT())
	oy14 := oy1 + hv.BorderBottomLeftRadius
	oy15 := bag.ScaledPointFromFloat(oy14.ToPT() - circleBezier*hv.BorderBottomLeftRadius.ToPT())
	oy16 := oy1

	innerBorderBottomRightRadiusX := math.Max(0, (hv.BorderBottomRightRadius - hv.BorderRightWidth).ToPT())
	innerBorderBottomRightRadiusY := math.Max(0, (hv.BorderBottomRightRadius - hv.BorderRightWidth).ToPT())
	innerBorderTopRightRadiusX := math.Max(0, (hv.BorderTopRightRadius - hv.BorderRightWidth).ToPT())
	innerBorderTopRightRadiusY := math.Max(0, (hv.BorderTopRightRadius - hv.BorderRightWidth).ToPT())
	innerBorderTopLeftRadiusX := math.Max(0, (hv.BorderTopLeftRadius - hv.BorderLeftWidth).ToPT())
	innerBorderTopLeftRadiusY := math.Max(0, (hv.BorderTopLeftRadius - hv.BorderLeftWidth).ToPT())
	innerBorderBottomLeftRadiusX := math.Max(0, (hv.BorderBottomLeftRadius - hv.BorderLeftWidth).ToPT())
	innerBorderBottomLeftRadiusY := math.Max(0, (hv.BorderBottomLeftRadius - hv.BorderLeftWidth).ToPT())

	// i = inner
	ix1 := bag.ScaledPointFromFloat(math.Max(ox1.ToPT(), (x0 + hv.BorderLeftWidth).ToPT()))
	ix2 := bag.ScaledPointFromFloat(math.Min(ox2.ToPT(), (width - hv.BorderRightWidth).ToPT()))
	ix3 := bag.ScaledPointFromFloat(ix2.ToPT() + circleBezier*innerBorderBottomRightRadiusX)
	ix4 := width - hv.BorderRightWidth
	ix5, ix6, ix7 := ix4, ix4, ix4
	ix9 := bag.ScaledPointFromFloat(math.Min(ox9.ToPT(), (width - hv.BorderRightWidth).ToPT()))
	ix8 := bag.ScaledPointFromFloat(ix9.ToPT() + circleBezier*innerBorderTopRightRadiusX)
	ix10 := bag.ScaledPointFromFloat(math.Max((x0 + hv.BorderLeftWidth).ToPT(), ox10.ToPT()))
	ix11 := bag.ScaledPointFromFloat(ix10.ToPT() - (circleBezier * innerBorderTopLeftRadiusX))
	ix13 := bag.ScaledPointFromFloat(math.Max(ox1.ToPT()+hv.BorderLeftWidth.ToPT(), ox13.ToPT()))
	ix12 := x0 + hv.BorderLeftWidth
	ix13, ix14, ix15 := ix12, ix12, ix12
	ix16 := bag.ScaledPointFromFloat(ox1.ToPT() - circleBezier*innerBorderBottomLeftRadiusX)

	iy1 := oy1 + hv.BorderBottomWidth
	iy2, iy3 := iy1, iy1
	iy5 := bag.ScaledPointFromFloat(math.Max(oy5.ToPT(), (-height + hv.BorderBottomWidth).ToPT()))
	iy4 := bag.ScaledPointFromFloat(iy5.ToPT() - circleBezier*innerBorderBottomRightRadiusY)
	iy6 := bag.ScaledPointFromFloat(math.Min(oy6.ToPT(), (-hv.BorderTopWidth).ToPT()))
	iy7 := bag.ScaledPointFromFloat(iy6.ToPT() + circleBezier*innerBorderTopRightRadiusY)
	iy9 := bag.ScaledPointFromFloat(math.Min(oy9.ToPT(), (y0 - hv.BorderTopWidth).ToPT()))
	iy8 := iy9
	iy10, iy11 := iy8, iy8
	iy13 := bag.ScaledPointFromFloat(math.Min((y0 - hv.BorderTopWidth).ToPT(), oy13.ToPT()))
	iy12 := bag.ScaledPointFromFloat(iy13.ToPT() + circleBezier*innerBorderTopLeftRadiusY)
	iy14 := bag.ScaledPointFromFloat(math.Max((-height + hv.BorderBottomWidth).ToPT(), oy14.ToPT()))
	iy15 := bag.ScaledPointFromFloat(iy14.ToPT() - circleBezier*innerBorderBottomLeftRadiusY)
	iy16 := iy1

	// When radii are added, we need to add control points. These points are
	// numbered from 1 to 16 counterclockwise and start with the bottom left point.
	// See https://blog.speedata.de/2020/06/22/borderdots.png for a visualization.

	rule := pdfdraw.New()
	rule.Save()
	rule.
		Moveto(ix1, iy1).
		Curveto(ix16, iy16, ix15, iy15, ix14, iy14).
		Lineto(ix13, iy13).
		Curveto(ix12, iy12, ix11, iy11, ix10, iy10).
		Lineto(ix9, iy9).
		Curveto(ix8, iy8, ix7, iy7, ix6, iy6).
		Lineto(ix5, iy5).
		Curveto(ix4, iy4, ix3, iy3, ix2, iy2).Close().Clip().Endpath()

	rule2 := pdfdraw.New()
	rule2.
		Restore().
		Moveto(ox1, oy1).
		Lineto(ox2, oy2).
		Curveto(ox3, oy3, ox4, oy4, ox5, oy5).
		Lineto(ox6, oy6).
		Curveto(ox6, oy7, ox8, oy8, ox9, oy9).
		Lineto(ox10, oy10).
		Curveto(ox11, oy11, ox12, oy12, ox13, oy13).
		Lineto(ox14, oy14).
		Curveto(ox15, oy15, ox16, oy16, ox1, oy1)
	rule2.
		Moveto(ix1, iy1).
		Curveto(ix16, iy16, ix15, iy15, ix14, iy14).
		Lineto(ix13, iy13).
		Curveto(ix12, iy12, ix11, iy11, ix10, iy10).
		Lineto(ix9, iy9).
		Curveto(ix8, iy8, ix7, iy7, ix6, iy6).
		Lineto(ix5, iy5).
		Curveto(ix4, iy4, ix3, iy3, ix2, iy2).Close().Clip().Endpath()

	scale := bag.ScaledPoint(2)
	x1 *= scale
	x2 -= x3 - x2
	y1 *= scale
	y2 -= y3 - y2
	rule2.ColorNonstroking(*hv.BorderTopColor).Moveto(x0, y0).Lineto(x1, y1).Lineto(x2, y1).Lineto(x3, y0).Close().Fill()
	rule2.ColorNonstroking(*hv.BorderLeftColor).Moveto(x0, y3).Lineto(x1, y2).Lineto(x1, y1).Lineto(x0, y0).Close().Fill()
	rule2.ColorNonstroking(*hv.BorderBottomColor).Moveto(x0, y3).Lineto(x3, y3).Lineto(x2, y2).Lineto(x1, y2).Close().Fill()
	rule2.ColorNonstroking(*hv.BorderRightColor).Moveto(x2, y2).Lineto(x3, y3).Lineto(x3, y0).Lineto(x2, y1).Close().Fill()

	return rule.String() + " ", rule2.String()
}
