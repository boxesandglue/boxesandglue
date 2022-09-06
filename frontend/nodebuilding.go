package frontend

import (
	"fmt"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/color"
	"github.com/speedata/boxesandglue/backend/document"
	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/boxesandglue/backend/lang"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/pdfbackend/pdf"
	"github.com/speedata/textlayout/harfbuzz"
)

// SettingType represents a setting such as font weight or color.
type SettingType int

// FontWeight is the type which represents different font weights.
type FontWeight int

func (fw FontWeight) String() string {
	switch fw {
	case 100:
		return "Thin"
	case 200:
		return "Extra Light"
	case 300:
		return "Light"
	case 400:
		return "Normal"
	case 500:
		return "Medium"
	case 600:
		return "SemiBold"
	case 700:
		return "Bold"
	case 800:
		return "Ultra Bold"
	case 900:
		return "Black"
	default:
		return fmt.Sprintf("fontweight %d", fw)
	}
}

const (
	// FontWeight100 is commonly named “Thin”.
	FontWeight100 FontWeight = 100
	// FontWeight200 is commonly named “Extra Light”.
	FontWeight200 FontWeight = 200
	// FontWeight300 is commonly named “Light”.
	FontWeight300 FontWeight = 300
	// FontWeight400 is commonly named “Normal”.
	FontWeight400 FontWeight = 400
	// FontWeight500 is commonly named “Medium”.
	FontWeight500 FontWeight = 500
	// FontWeight600 is commonly named “Semi Bold”.
	FontWeight600 FontWeight = 600
	// FontWeight700 is commonly named Bold”.
	FontWeight700 FontWeight = 700
	// FontWeight800 is commonly named “Ultra Bold”.
	FontWeight800 FontWeight = 800
	// FontWeight900 is commonly named “Black”.
	FontWeight900 FontWeight = 900
)

// FontStyle is the type which represents different font styles such as italic or oblique.
type FontStyle int

func (fs FontStyle) String() string {
	switch fs {
	case FontStyleNormal:
		return "normal"
	case FontStyleItalic:
		return "italic"
	case FontStyleOblique:
		return "oblique"
	default:
		return "???"
	}
}

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
	// SettingHyperlink defines an external hyperlink.
	SettingHyperlink
	// SettingMarginLeft sets the left margin.
	SettingMarginLeft
	// SettingMarginRight sets the right margin.
	SettingMarginRight
	// SettingMarginBottom sets the bottom margin.
	SettingMarginBottom
	// SettingMarginTop sets the top margin.
	SettingMarginTop
	// SettingOpenTypeFeature allows the user to (de)select OpenType features such as ligatures.
	SettingOpenTypeFeature
)

func (st SettingType) String() string {
	return fmt.Sprintf("%d", st)
}

// TypesettingSettings is a set of settings for text rendering.
type TypesettingSettings map[SettingType]any

// Paragraph associates all items with the given settings. Items can be
// text (string), images, other instances of a Paragraph or nodes.
type Paragraph struct {
	Settings TypesettingSettings
	Items    []any
}

// NewParagraph returns an initialized typesetting element.
func NewParagraph() *Paragraph {
	te := Paragraph{}
	te.Settings = make(TypesettingSettings)
	return &te
}

func (ts *Paragraph) String() string {
	ret := []string{}
	ret = append(ret, "Settings:")
	for k, v := range ts.Settings {
		ret = append(ret, fmt.Sprintf("%s:%v", k, v))
	}
	ret = append(ret, fmt.Sprintf("\nitems(len %d)", len(ts.Items)))
	for _, itm := range ts.Items {
		ret = append(ret, fmt.Sprintf("%s", itm))
	}
	return strings.Join(ret, " ")
}

type paragraph struct {
	fontsize   bag.ScaledPoint
	fontfamily *FontFamily
	hsize      bag.ScaledPoint
	leading    bag.ScaledPoint
	language   *lang.Lang
}

// TypesettingOption controls the formatting of the paragraph.
type TypesettingOption func(*paragraph)

// Leading sets the distance between two baselines in a paragraph.
func Leading(leading bag.ScaledPoint) TypesettingOption {
	return func(p *paragraph) {
		p.leading = leading
	}
}

// Language sets the default language for the whole paragraph (used for hyphenation).
func Language(language *lang.Lang) TypesettingOption {
	return func(p *paragraph) {
		p.language = language
	}
}

// FontSize sets the font size for the paragraph.
func FontSize(size bag.ScaledPoint) TypesettingOption {
	return func(p *paragraph) {
		p.fontsize = size
	}
}

// Family sets the font family for the paragraph.
func Family(fam *FontFamily) TypesettingOption {
	return func(p *paragraph) {
		p.fontfamily = fam
	}
}

// FormatParagraph creates a rectangular text from the data stored in the
// Paragraph.
func (fe *Document) FormatParagraph(te *Paragraph, hsize bag.ScaledPoint, opts ...TypesettingOption) (*node.VList, []*node.Breakpoint, error) {
	p := &paragraph{
		language: fe.Doc.DefaultLanguage,
		hsize:    hsize,
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.fontsize != 0 {
		te.Settings[SettingSize] = p.fontsize
	}
	if p.fontfamily != nil {
		te.Settings[SettingFontFamily] = p.fontfamily
	}
	hlist, tail, err := fe.Mknodes(te)
	if err != nil {
		return nil, nil, err
	}

	Hyphenate(hlist, p.language)
	node.AppendLineEndAfter(tail)

	ls := node.NewLinebreakSettings()
	ls.HSize = p.hsize
	if p.leading == 0 {
		ls.LineHeight = p.fontsize * 120 / 100
	} else {
		ls.LineHeight = p.leading
	}
	vlist, info := node.Linebreak(hlist, ls)
	return vlist, info, nil
}

func (fe *Document) buildNodelistFromString(ts TypesettingSettings, str string) (node.Node, error) {
	fontweight := FontWeight400
	fontstyle := FontStyleNormal
	var fontfamily *FontFamily
	fontsize := 12 * bag.Factor
	var col *color.Color
	var hyperlink document.Hyperlink
	var hasHyperlink bool
	fontfeatures := make([]harfbuzz.Feature, 0, len(fe.DefaultFeatures))
	for _, f := range fe.DefaultFeatures {
		fontfeatures = append(fontfeatures, f)
	}

	for k, v := range ts {
		switch k {
		case SettingFontWeight:
			switch t := v.(type) {
			case int:
				fontweight = FontWeight(t)
			case FontWeight:
				fontweight = t
			}
		case SettingFontFamily:
			fontfamily = v.(*FontFamily)
		case SettingSize:
			fontsize = v.(bag.ScaledPoint)
		case SettingColor:
			switch t := v.(type) {
			case string:
				if c := fe.GetColor(t); c != nil {
					col = c
				}
			case *color.Color:
				col = t
			}
		case SettingHyperlink:
			hyperlink = v.(document.Hyperlink)
			hasHyperlink = true
		case SettingStyle:
			fontstyle = v.(FontStyle)
		case SettingOpenTypeFeature:
			for _, str := range strings.Split(v.(string), ",") {
				f, err := harfbuzz.ParseFeature(str)
				if err != nil {
					bag.Logger.Errorf("cannot parse OpenType feature tag %q.", str)
				}
				fontfeatures = append(fontfeatures, f)
			}
		case SettingMarginTop, SettingMarginRight, SettingMarginBottom, SettingMarginLeft:
			// ignore
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
		hyperlinkStart = node.NewStartStop()
		hyperlinkStart.Action = node.ActionHyperlink
		hyperlinkStart.Value = &hyperlink
		if head != nil {
			head = node.InsertAfter(head, head, hyperlinkStart)
		}
		head = hyperlinkStart
	}

	if col != nil {
		colStart := node.NewStartStop()
		colStart.Position = node.PDFOutputPage
		colStart.Callback = func(n node.Node) string {
			return col.PDFStringStroking() + " "
		}
		if head != nil {
			head = node.InsertAfter(head, head, colStart)
		}
		head = colStart
	}
	cur = head
	var lastglue node.Node
	atoms := fnt.Shape(str, fontfeatures)
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

			if r.Kernafter != 0 {
				k := node.NewKern()
				k.Kern = r.Kernafter
				head = node.InsertAfter(head, cur, k)
				cur = k
			}
		}
	}
	if col != nil {
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

// Mknodes creates a list of nodes which which can be formatted to a given
// width. The returned head and the tail are the beginning and the end of the
// node list.
func (fe *Document) Mknodes(ts *Paragraph) (head node.Node, tail node.Node, err error) {
	if len(ts.Items) == 0 {
		return nil, nil, nil
	}
	var newSettings = make(TypesettingSettings)
	var nl, end node.Node
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
			head = node.InsertAfter(head, tail, nl)
			tail = node.Tail(nl)
		case *Paragraph:
			for k, v := range newSettings {
				if _, found := t.Settings[k]; !found {
					t.Settings[k] = v
				}
			}
			nl, end, err = fe.Mknodes(t)
			if err != nil {
				return nil, nil, err
			}
			if nl != nil {
				head = node.InsertAfter(head, tail, nl)
				tail = end
			}
		case node.Node:
			head = node.InsertAfter(head, tail, t)
			tail = t
		default:
			bag.Logger.DPanicf("Mknodes: unknown item type %T", t)
		}
	}
	return head, tail, nil
}
