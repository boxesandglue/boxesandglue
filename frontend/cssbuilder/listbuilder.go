package cssbuilder

import (
	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/frontend"
)

// CreateVlist converts the te into a big vlist.
func (cb *CSSBuilder) CreateVlist(te *frontend.Text, wd bag.ScaledPoint) (*node.VList, error) {
	_, err := cb.buildVlistInternal(te, wd, 0, 0, 0)
	if err != nil {
		return nil, err
	}

	var list node.Node

	for _, n := range cb.pagebox {
		switch t := n.(type) {
		case *node.StartStop:
			// ignore for now - should be used for frames
		case *node.VList:
			if xattr, ok := t.Attributes["x"].(bag.ScaledPoint); ok {
				t.ShiftX = xattr
			}
			list = node.InsertAfter(list, node.Tail(list), t)
		}
	}

	return node.Vpack(list), nil
}

func (cb *CSSBuilder) buildVlistInternal(te *frontend.Text, width bag.ScaledPoint, x, y bag.ScaledPoint, prevMB bag.ScaledPoint) (*info, error) {
	var sumHT bag.ScaledPoint
	dim, err := cb.PageSize()
	if err != nil {
		return nil, err
	}
	hv := frontend.SettingsToValues(te.Settings)
	// margin collapse
	if hv.MarginTop > prevMB {
		hv.MarginTop = hv.MarginTop - prevMB
	} else {
		hv.MarginTop = 0
	}
	hsize := width - hv.MarginLeft - hv.MarginRight - hv.BorderLeftWidth - hv.BorderRightWidth - hv.PaddingLeft - hv.PaddingRight
	x += hv.MarginLeft
	y -= hv.MarginTop
	start := node.NewStartStop()
	start.Attributes = node.H{
		"hv":    hv,
		"hsize": hsize,
		"y":     y,
		"x":     x,
	}
	y -= hv.BorderTopWidth
	x += hv.PaddingLeft
	cb.pagebox = append(cb.pagebox, start)

	if bx, ok := te.Settings[frontend.SettingBox]; ok && bx.(bool) {
		// a box, containing one or more item (a div for example)
		marginBottom := bag.ScaledPoint(0)

		if prepend, ok := te.Settings[frontend.SettingPrepend]; ok {
			if p, ok := prepend.(node.Node); ok {
				g := node.NewGlue()
				g.Stretch = bag.Factor
				g.Shrink = bag.Factor
				g.StretchOrder = node.StretchFil
				g.ShrinkOrder = node.StretchFil
				p = node.InsertBefore(p, p, g)
				p = node.HpackTo(p, 0)
				p.(*node.HList).Depth = 0
				vl := node.Vpack(p)
				vl.Height = 0
				vl.Attributes = node.H{"y": y, "x": x, "origin": "v prepend in HTML mode"}
				cb.pagebox = append(cb.pagebox, vl)
			}
		}

		for _, itm := range te.Items {
			if txt, ok := itm.(*frontend.Text); ok {
				info, err := cb.buildVlistInternal(txt, hsize, x+hv.MarginLeft+hv.BorderLeftWidth, y, marginBottom)
				if err != nil {
					return nil, err
				}
				y = info.newY
				y -= info.height
				sumHT += info.height
				marginBottom = info.hv.MarginBottom
			}
		}
	} else {
		// something like a p tag that contains some stuff
		// to be typeset.
		vl := cb.createVList(te, hsize, hv)
		sumHT = vl.Height
		if y-vl.Height-vl.Depth < dim.MarginBottom {
			start.Attributes["pagebreak"] = true
			sumHT = vl.Height
			y = dim.Height - dim.MarginTop
			start.Attributes["y"] = y
		}

		if prepend, ok := te.Settings[frontend.SettingPrepend]; ok {
			if p, ok := prepend.(node.Node); ok {
				g := node.NewGlue()
				g.Stretch = bag.Factor
				g.Shrink = bag.Factor
				g.StretchOrder = node.StretchFil
				g.ShrinkOrder = node.StretchFil
				p = node.InsertBefore(p, p, g)
				p = node.HpackTo(p, 0)
				p.(*node.HList).Depth = 0
				n := node.InsertAfter(p, node.Tail(p), vl)
				hl := node.Hpack(n)
				hl.VAlign = node.VAlignTop
				vl = node.Vpack(hl)
			}
		}
		x += hv.BorderLeftWidth
		vl.Attributes = node.H{"y": y, "x": x, "origin": "prepend in HTML mode"}
		cb.pagebox = append(cb.pagebox, vl)

	}

	y -= hv.MarginBottom
	start.Attributes["height"] = sumHT
	thisht := sumHT + hv.MarginTop + hv.MarginBottom + hv.BorderTopWidth + hv.BorderBottomWidth
	return &info{sumht: sumHT, height: thisht, hv: hv, newY: y}, nil
}

func (cb *CSSBuilder) createVList(te *frontend.Text, wd bag.ScaledPoint, hv frontend.HTMLValues) *node.VList {
	vl, _, _ := cb.frontend.FormatParagraph(te, wd)
	// FIXME: vl can be nil if empty (empty li for example)
	vl.Attributes = node.H{
		"hv":    hv,
		"hsize": wd,
	}
	return vl
}
