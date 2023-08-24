package cssbuilder

import (
	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/frontend"
)

// CreateVlist converts the te into a big vlist.
func (cb *CSSBuilder) CreateVlist(te *frontend.Text, wd bag.ScaledPoint) (*node.VList, error) {
	info, err := cb.buildVlistInternal(te, wd, 0, 0)
	if err != nil {
		return nil, err
	}

	var list node.Node

	for _, n := range info.pagebox {
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

func (cb *CSSBuilder) buildVlistInternal(te *frontend.Text, width bag.ScaledPoint, x bag.ScaledPoint, shiftDown bag.ScaledPoint) (*info, error) {
	hv := frontend.SettingsToValues(te.Settings)
	hsize := width - hv.MarginLeft - hv.MarginRight - hv.BorderLeftWidth - hv.BorderRightWidth - hv.PaddingLeft - hv.PaddingRight
	x += hv.MarginLeft

	ret := &info{
		marginTop:    hv.MarginTop,
		marginBottom: hv.MarginBottom,
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
			vl := node.Vpack(p)
			vl.Height = 0
			vl.Attributes = node.H{"height": bag.ScaledPoint(0), "x": x, "origin": "v prepend in HTML mode"}
			ret.pagebox = append(ret.pagebox, vl)
		}
	}

	var prevMB, height bag.ScaledPoint
	if bx, ok := te.Settings[frontend.SettingBox]; ok && bx.(bool) {
		// a box, containing one or more item (a div for example)
		for _, itm := range te.Items {
			if txt, ok := itm.(*frontend.Text); ok {
				info, err := cb.buildVlistInternal(txt, hsize, x+hv.BorderLeftWidth+hv.PaddingLeft, shiftDown)
				if err != nil {
					return nil, err
				}

				// margin collapse
				if prevMB >= info.marginTop {
					info.marginTop = 0
				} else {
					info.marginTop -= prevMB
				}
				height += info.height
				height += info.marginTop + info.marginBottom
				height += info.hv.PaddingTop + info.hv.PaddingBottom + info.hv.BorderTopWidth + info.hv.BorderBottomWidth

				start := node.NewStartStop()
				start.Attributes = node.H{
					"shiftDown": info.marginTop,
					"hv":        info.hv,
					"height":    info.height,
					"hsize":     info.hsize,
					"x":         info.x,
				}
				ret.pagebox = append(ret.pagebox, start)

				if info.vl == nil {
					ret.pagebox = append(ret.pagebox, info.pagebox...)
				} else {
					ret.pagebox = append(ret.pagebox, info.vl)
				}

				stop := node.NewStartStop()
				stop.Attributes = node.H{
					"shiftDown": info.marginBottom,
					"height":    height,
					"hv":        info.hv,
				}
				stop.StartNode = start
				ret.pagebox = append(ret.pagebox, stop)
				prevMB = info.marginBottom
			}
		}
		ret.x = x
		ret.hsize = hsize
		ret.height = height
		ret.hv = hv
		return ret, nil
	}

	// not a box
	//
	// something like a p tag that contains some stuff to be typeset.
	vl, err := cb.createVList(te, hsize, hv)
	if err != nil {
		return nil, err
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

	vl.Attributes = node.H{
		"height": vl.Height + vl.Depth,
		"x":      x + hv.PaddingLeft + hv.BorderLeftWidth,
		"hsize":  hsize,
	}
	ret.height = vl.Height + vl.Depth
	ret.vl = vl
	ret.hv = hv
	ret.hsize = hsize
	ret.x = x
	return ret, nil
}

func (cb *CSSBuilder) createVList(te *frontend.Text, wd bag.ScaledPoint, hv frontend.HTMLValues) (*node.VList, error) {
	vl, _, err := cb.frontend.FormatParagraph(te, wd)
	// FIXME: vl can be nil if empty (empty li for example)
	if err != nil {
		return nil, err
	}
	vl.Attributes = node.H{
		"hv":    hv,
		"hsize": wd,
	}
	return vl, nil
}
