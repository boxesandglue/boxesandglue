package frontend

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/node"
)

type VlistInfo struct {
	vl           *node.VList
	hsize        bag.ScaledPoint
	x            bag.ScaledPoint
	marginTop    bag.ScaledPoint
	marginBottom bag.ScaledPoint
	Pagebox      []node.Node
	height       bag.ScaledPoint
	hv           HTMLValues
	debug        string
}

// CreateVlist converts the te into a big vlist.
func (fe *Document) CreateVlist(te *Text, wd bag.ScaledPoint) (*node.VList, error) {
	info, err := fe.BuildVlistInternal(te, wd, 0, 0)
	if err != nil {
		return nil, err
	}

	var list node.Node

	for _, n := range info.Pagebox {
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

// BuildVlistInternal does this and that
func (fe *Document) BuildVlistInternal(te *Text, width bag.ScaledPoint, x bag.ScaledPoint, shiftDown bag.ScaledPoint) (*VlistInfo, error) {
	hv := SettingsToValues(te.Settings)
	hsize := width - hv.MarginLeft - hv.MarginRight - hv.BorderLeftWidth - hv.BorderRightWidth - hv.PaddingLeft - hv.PaddingRight
	x += hv.MarginLeft

	ret := &VlistInfo{
		marginTop:    hv.MarginTop,
		marginBottom: hv.MarginBottom,
	}

	if prepend, ok := te.Settings[SettingPrepend]; ok {
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
			ret.Pagebox = append(ret.Pagebox, vl)
		}
	}

	var prevMB, height bag.ScaledPoint
	if bx, ok := te.Settings[SettingBox]; ok && bx.(bool) {
		// a box, containing one or more item (a div for example)
		for _, itm := range te.Items {
			switch textItem := itm.(type) {
			case *Text:
				info, err := fe.BuildVlistInternal(textItem, hsize, x+hv.BorderLeftWidth+hv.PaddingLeft, shiftDown)
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
				ret.Pagebox = append(ret.Pagebox, start)

				if info.vl == nil {
					ret.Pagebox = append(ret.Pagebox, info.Pagebox...)
				} else {
					ret.Pagebox = append(ret.Pagebox, info.vl)
				}

				stop := node.NewStartStop()
				stop.Attributes = node.H{
					"shiftDown": info.marginBottom,
					"height":    height,
					"hv":        info.hv,
				}
				stop.StartNode = start
				ret.Pagebox = append(ret.Pagebox, stop)
				prevMB = info.marginBottom
			case string:
				vl, err := fe.createVList(te, hsize, hv)
				if err != nil {
					return nil, err
				}

				if prepend, ok := te.Settings[SettingPrepend]; ok {
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

			default:
				fmt.Println("~~> unknown type", textItem)
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
	vl, err := fe.createVList(te, hsize, hv)
	if err != nil {
		return nil, err
	}

	if prepend, ok := te.Settings[SettingPrepend]; ok {
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

func fixupWidth(te *Text, hsize bag.ScaledPoint, hv HTMLValues) error {
	var err error
	for _, itm := range te.Items {
		switch t := itm.(type) {
		case *Text:
			if err = fixupWidth(t, hsize, hv); err != nil {
				return err
			}
		case string:
			// ignore
		case *node.Image:
			if wd, ok := t.Attributes["wd"]; ok {
				t.Width = wd.(bag.ScaledPoint)
			}
			if ht, ok := t.Attributes["ht"]; ok {
				t.Height = ht.(bag.ScaledPoint)
			} else {
				attr := t.Attributes["attr"].(map[string]string)
				if k, ok := attr["!width"]; ok {
					if str, ok := strings.CutSuffix(k, "%"); ok {
						f, err := strconv.ParseFloat(str, 64)
						if err != nil {
							return err
						}
						t.Width = bag.MultiplyFloat(hsize, f/100)
					}
				}
			}
		case *Table:
			// ignore
		default:
			return fmt.Errorf("fixupWidth: unknown item %T", t)
		}
	}
	return nil
}

func (fe *Document) createVList(te *Text, wd bag.ScaledPoint, hv HTMLValues) (*node.VList, error) {
	if err := fixupWidth(te, wd, hv); err != nil {
		return nil, err
	}
	vl, _, err := fe.FormatParagraph(te, wd)
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
