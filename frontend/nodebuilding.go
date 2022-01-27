package frontend

import (
	"fmt"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/document"
	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/pdfbackend/pdf"
)

// SettingType represents a setting such as font weight or color.
type SettingType int

// FontWeight is the type which represents different font weights.
type FontWeight int

// FontStyle is the type which represents different font styles such as italic or oblique.
type FontStyle int

const (
	// FontWeight100 is commonly named “Thin”.
	FontWeight100 FontWeight = 100
	// FontWeight200 is commonly named “Extra Light”.
	FontWeight200 = 200
	// FontWeight300 is commonly named “Light”.
	FontWeight300 = 300
	// FontWeight400 is commonly named “Normal”.
	FontWeight400 = 400
	// FontWeight500 is commonly named “Medium”.
	FontWeight500 = 500
	// FontWeight600 is commonly named “Semi Bold”.
	FontWeight600 = 600
	// FontWeight700 is commonly named “Semi Bold”.
	FontWeight700 = 700
	// FontWeight800 is commonly named “Ultra Bold”.
	FontWeight800 = 800
	// FontWeight900 is commonly named “Black”.
	FontWeight900 = 900
)

const (
	// FontStyleNormal is an upright font.
	FontStyleNormal FontStyle = iota
	// FontStyleItalic is an italicized font.
	FontStyleItalic
	// FontStyleOblique is an upright font tilted by an angle.
	FontStyleOblique
)

const (
	// SettingFontWeight represents a font weight setting.
	SettingFontWeight SettingType = iota
	// SettingColor sets a predefined color.
	SettingColor
	// SettingStyle represents a font style such as italic or normal.
	SettingStyle
	// SettingFontFamily selects a font family.
	SettingFontFamily
	// SettingSize sets the font size.
	SettingSize
	// SettingHyperlink defines an external hypherlink.
	SettingHyperlink
)

// TypesettingSettings is a hash of glyph attributes to values.
type TypesettingSettings map[SettingType]interface{}

// TypesettingElement associates all items with the given settings. Items can be
// text (string), images, other instances of a TypesettingElement or nodes.
type TypesettingElement struct {
	Settings TypesettingSettings
	Items    []interface{}
}

func (fe *Frontend) buildNodelistFromString(ts TypesettingSettings, str string) (node.Node, error) {
	fontweight := FontWeight400
	fontstyle := FontStyleNormal
	var fontfamily *FontFamily
	fontsize := 12 * bag.Factor
	var color string
	var hyperlink document.Hyperlink
	var hasColor, hasHyperlink bool

	for k, v := range ts {
		switch k {
		case SettingFontWeight:
			fontweight = v.(int)
		case SettingFontFamily:
			fontfamily = v.(*FontFamily)
		case SettingSize:
			fontsize = v.(bag.ScaledPoint)
		case SettingColor:
			color = v.(string)
			hasColor = true
		case SettingHyperlink:
			hyperlink = v.(document.Hyperlink)
			hasHyperlink = true
		default:
			bag.Logger.DPanicf("Unknown setting %v", k)
		}
	}

	var fnt *font.Font
	var face *pdf.Face
	var fs *FontSource
	var err error

	if fs, err = fontfamily.GetFontSource(fontweight, fontstyle); err != nil {
		return nil, err
	}
	if face, err = fe.LoadFace(fs); err != nil {
		return nil, err
	}
	if fe.usedFonts[face] == nil {
		fe.usedFonts = make(map[*pdf.Face]map[bag.ScaledPoint]*font.Font)
	}
	if fe.usedFonts[face][fontsize] == nil {
		fe.usedFonts[face] = make(map[bag.ScaledPoint]*font.Font)
	}

	var found bool
	if fnt, found = fe.usedFonts[face][fontsize]; !found {
		fnt = fe.Doc.CreateFont(face, fontsize)
		fe.usedFonts[face][fontsize] = fnt
	}

	var head, cur node.Node
	var hyperlinkStart, hyperlinkStop *node.StartStop
	if hasHyperlink {
		if false {
			fmt.Println(hyperlink)
		}
		hyperlinkStart = node.NewStartStop()
		hyperlinkStart.Action = node.ActionHyperlink
		hyperlinkStart.Value = &hyperlink
		if head != nil {
			head = node.InsertAfter(head, head, hyperlinkStart)
		}
		head = hyperlinkStart
	}

	if hasColor {
		if col := fe.GetColor(color); col != nil {
			colStart := node.NewStartStop()
			colStart.Position = node.PDFOutputPage
			colStart.Callback = func(n node.Node) string {
				return col.PDFStringFG() + " "
			}
			if head != nil {
				head = node.InsertAfter(head, head, colStart)
			}
			head = colStart
		} else {
			bag.Logger.Errorf("color %q not found", color)
		}
	}
	cur = head
	var lastglue node.Node
	atoms := fnt.Shape(str)
	for _, r := range atoms {
		if r.IsSpace {
			if lastglue == nil {
				g := node.NewGlue()
				g.Width = fnt.Space
				g.Stretch = fnt.SpaceStretch
				g.Shrink = fnt.SpaceShrink
				head = node.InsertAfter(head, cur, g)
				cur = g
				lastglue = g
			}
		} else {
			n := node.NewGlyph()
			n.Hyphenate = r.Hyphenate
			n.Codepoint = r.Codepoint
			n.Components = r.Components
			n.Font = fnt
			n.Width = r.Advance
			n.Height = r.Height
			n.Depth = r.Depth
			head = node.InsertAfter(head, cur, n)
			cur = n
			lastglue = nil
		}
	}
	if hasColor {
		stop := node.NewStartStop()
		stop.Position = node.PDFOutputPage
		stop.Callback = func(n node.Node) string {
			return "0 0 0 RG 0 0 0 rg "
		}
		node.InsertAfter(head, cur, stop)
		cur = stop
	}
	if hasHyperlink {
		hyperlinkStop = node.NewStartStop()
		hyperlinkStop.StartNode = hyperlinkStart
		head = node.InsertAfter(head, cur, hyperlinkStop)
		cur = hyperlinkStop
	}

	return head, nil
}

// Mknodes creates a list of nodes which which can be formatted to a given width.
func (fe *Frontend) Mknodes(ts *TypesettingElement) (node.Node, node.Node, error) {
	var newSettings = make(TypesettingSettings)
	var head, cur, nl, end node.Node
	var err error
	for k, v := range ts.Settings {
		newSettings[k] = v
	}
	for _, itm := range ts.Items {
		switch t := itm.(type) {
		case string:
			nl, err = fe.buildNodelistFromString(newSettings, t)
			if err != nil {
				return nil, nil, err
			}
			head = node.InsertAfter(head, cur, nl)
			cur = node.Tail(nl)
		case *TypesettingElement:
			for k, v := range newSettings {
				if _, found := t.Settings[k]; !found {
					t.Settings[k] = v
				}
			}
			nl, end, err = fe.Mknodes(t)
			if err != nil {
				return nil, nil, err
			}
			head = node.InsertAfter(head, cur, nl)
			cur = end
		default:
			fmt.Printf("Mknodes: unknown item type %T\n", t)
		}

	}
	return head, cur, nil
}
