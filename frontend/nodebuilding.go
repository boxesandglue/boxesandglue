package frontend

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"sort"
	"strings"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/document"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/image"
	"github.com/boxesandglue/boxesandglue/backend/lang"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textlayout/harfbuzz"
)

// FormatToVList is a function that gets collects typesetting material and gets
// executed when the hsize is known.
type FormatToVList func(bag.ScaledPoint) (*node.VList, error)

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

// TextDecorationLine sets the underline type
type TextDecorationLine int

const (
	// TextDecorationLineNone means no underline
	TextDecorationLineNone TextDecorationLine = iota
	// TextDecorationUnderline is a simple underlining
	TextDecorationUnderline
	// TextDecorationOverline has a line above
	TextDecorationOverline
	// TextDecorationLineThrough is a strike out
	TextDecorationLineThrough
)

// BorderStyle represents the HTML border styles such as solid, dashed, ...
type BorderStyle uint

const (
	// BorderStyleNone is no border
	BorderStyleNone BorderStyle = iota
	// BorderStyleSolid is a solid line
	BorderStyleSolid
)

const (
	// SettingDummy is a no op.
	SettingDummy SettingType = iota
	// SettingBox signals that this text element contains items that should be arranged vertically.
	SettingBox
	// SettingBackgroundColor sets the background color.
	SettingBackgroundColor
	// SettingBorderBottomWidth sets the bottom border width.
	SettingBorderBottomWidth
	// SettingBorderLeftWidth sets the left border width.
	SettingBorderLeftWidth
	// SettingBorderRightWidth sets the right border width.
	SettingBorderRightWidth
	// SettingBorderTopWidth sets the top border width.
	SettingBorderTopWidth
	// SettingBorderBottomColor sets the bottom border color.
	SettingBorderBottomColor
	// SettingBorderLeftColor sets the left border color.
	SettingBorderLeftColor
	// SettingBorderRightColor sets the right border color.
	SettingBorderRightColor
	// SettingBorderTopColor sets the top border color.
	SettingBorderTopColor
	// SettingBorderBottomStyle sets the bottom border style.
	SettingBorderBottomStyle
	// SettingBorderLeftStyle sets the left border style.
	SettingBorderLeftStyle
	// SettingBorderRightStyle sets the right border style.
	SettingBorderRightStyle
	// SettingBorderTopStyle sets the top border style.
	SettingBorderTopStyle
	// SettingBorderTopLeftRadius sets the top left radius (x and y are the same).
	SettingBorderTopLeftRadius
	// SettingBorderTopRightRadius sets the top right radius (x and y are the same).
	SettingBorderTopRightRadius
	// SettingBorderBottomLeftRadius sets the bottom left radius (x and y are the same).
	SettingBorderBottomLeftRadius
	// SettingBorderBottomRightRadius sets the bottom right radius (x and y are the same).
	SettingBorderBottomRightRadius
	// SettingColor sets a predefined color.
	SettingColor
	// SettingDebug can contain debugging information
	SettingDebug
	// SettingFontExpansion is the amount of expansion / shrinkage allowed. Value is a float between 0 (no expansion) and 1 (100% of the glyph width).
	SettingFontExpansion
	// SettingFontFamily selects a font family.
	SettingFontFamily
	// SettingFontWeight represents a font weight setting.
	SettingFontWeight
	// SettingHAlign sets the horizontal alignment of the paragraph.
	SettingHAlign
	// SettingHangingPunctuation sets the margin protrusion.
	SettingHangingPunctuation
	// SettingHeight sets the height of a box if it should be vertically aligned.
	SettingHeight
	// SettingHyperlink defines an external hyperlink.
	SettingHyperlink
	// SettingIndentLeft inserts a left margin
	SettingIndentLeft
	// SettingIndentLeftRows determines the number of rows to be indented (positive value), or the number of rows not indented (negative values). 0 means all rows.
	SettingIndentLeftRows
	// SettingLeading determines the distance between two base lines (line height).
	SettingLeading
	// SettingMarginBottom sets the bottom margin.
	SettingMarginBottom
	// SettingMarginLeft sets the left margin.
	SettingMarginLeft
	// SettingMarginRight sets the right margin.
	SettingMarginRight
	// SettingMarginTop sets the top margin.
	SettingMarginTop
	// SettingOpenTypeFeature allows the user to (de)select OpenType features such as ligatures.
	SettingOpenTypeFeature
	// SettingPaddingBottom is the bottom padding.
	SettingPaddingBottom
	// SettingPaddingLeft is the left hand padding.
	SettingPaddingLeft
	// SettingPaddingRight is the right hand padding.
	SettingPaddingRight
	// SettingPaddingTop is the top padding.
	SettingPaddingTop
	// SettingPrepend contains a node list which should be prepended to the list.
	SettingPrepend
	// SettingPreserveWhitespace makes a monospace paragraph with newlines.
	SettingPreserveWhitespace
	// SettingSize sets the font size.
	SettingSize
	// SettingStyle represents a font style such as italic or normal.
	SettingStyle
	// SettingTabSizeSpaces is the amount of spaces for a tab.
	SettingTabSizeSpaces
	// SettingTabSize is the tab width.
	SettingTabSize
	// SettingTextDecorationLine sets underline
	SettingTextDecorationLine
	// SettingWidth sets alternative widths for the text.
	SettingWidth
	// SettingVAlign sets the vertical alignment. A height should be set.
	SettingVAlign
	// SettingYOffset shifts the glyph.
	SettingYOffset
)

func (st SettingType) String() string {
	var settingName string
	switch st {
	case SettingBackgroundColor:
		settingName = "SettingBackgroundColor"
	case SettingBorderBottomColor:
		settingName = "SettingBorderBottomColor"
	case SettingBorderBottomLeftRadius:
		settingName = "SettingBorderBottomLeftRadius"
	case SettingBorderBottomRightRadius:
		settingName = "SettingBorderBottomRightRadius"
	case SettingBorderBottomStyle:
		settingName = "SettingBorderBottomStyle"
	case SettingBorderBottomWidth:
		settingName = "SettingBorderBottomWidth"
	case SettingBorderLeftColor:
		settingName = "SettingBorderLeftColor"
	case SettingBorderLeftStyle:
		settingName = "SettingBorderLeftStyle"
	case SettingBorderLeftWidth:
		settingName = "SettingBorderLeftWidth"
	case SettingBorderRightColor:
		settingName = "SettingBorderRightColor"
	case SettingBorderRightStyle:
		settingName = "SettingBorderRightStyle"
	case SettingBorderRightWidth:
		settingName = "SettingBorderRightWidth"
	case SettingBorderTopColor:
		settingName = "SettingBorderTopColor"
	case SettingBorderTopLeftRadius:
		settingName = "SettingBorderTopLeftRadius"
	case SettingBorderTopRightRadius:
		settingName = "SettingBorderTopRightRadius"
	case SettingBorderTopStyle:
		settingName = "SettingBorderTopStyle"
	case SettingBorderTopWidth:
		settingName = "SettingBorderTopWidth"
	case SettingBox:
		settingName = "SettingBox"
	case SettingColor:
		settingName = "SettingColor"
	case SettingDebug:
		settingName = "SettingDebug"
	case SettingFontExpansion:
		settingName = "SettingFontExpansion"
	case SettingFontFamily:
		settingName = "SettingFontFamily"
	case SettingFontWeight:
		settingName = "SettingFontWeight"
	case SettingHAlign:
		settingName = "SettingHAlign"
	case SettingHangingPunctuation:
		settingName = "SettingHangingPunctuation"
	case SettingHeight:
		settingName = "SettingHeight"
	case SettingHyperlink:
		settingName = "SettingHyperlink"
	case SettingIndentLeft:
		settingName = "SettingIndentLeft"
	case SettingIndentLeftRows:
		settingName = "SettingIndentLeftRows"
	case SettingLeading:
		settingName = "SettingLeading"
	case SettingMarginBottom:
		settingName = "SettingMarginBottom"
	case SettingMarginLeft:
		settingName = "SettingMarginLeft"
	case SettingMarginRight:
		settingName = "SettingMarginRight"
	case SettingMarginTop:
		settingName = "SettingMarginTop"
	case SettingOpenTypeFeature:
		settingName = "SettingOpenTypeFeature"
	case SettingPaddingBottom:
		settingName = "SettingPaddingBottom"
	case SettingPaddingLeft:
		settingName = "SettingPaddingLeft"
	case SettingPaddingRight:
		settingName = "SettingPaddingRight"
	case SettingPaddingTop:
		settingName = "SettingPaddingTop"
	case SettingPrepend:
		settingName = "SettingPrepend"
	case SettingPreserveWhitespace:
		settingName = "SettingPreserveWhitespace"
	case SettingSize:
		settingName = "SettingSize"
	case SettingStyle:
		settingName = "SettingStyle"
	case SettingTabSize:
		settingName = "SettingTabSize"
	case SettingTabSizeSpaces:
		settingName = "SettingTabSizeSpaces"
	case SettingTextDecorationLine:
		settingName = "SettingTextDecorationLine"
	case SettingVAlign:
		settingName = "SettingVAlign"
	case SettingWidth:
		settingName = "SettingWidth"
	case SettingYOffset:
		settingName = "SettingYOffset"
	default:
		settingName = fmt.Sprintf("%d", st)
	}
	return fmt.Sprintf("%s", settingName)
}

// TypesettingSettings is a set of settings for text rendering.
type TypesettingSettings map[SettingType]any

// Text associates all items with the given settings. Items can be text
// (string), images, other instances of Text or nodes. Text behaves like a span
// in HTML or it just contains a collection of Go strings.
type Text struct {
	Settings TypesettingSettings
	Items    []any
}

// NewText returns an initialized text element.
func NewText() *Text {
	te := Text{}
	te.Settings = make(TypesettingSettings)
	return &te
}

// DebugTextToFile writes an XML representation of the Text to the filename. It
// overwrites the file if it already exists.
func DebugTextToFile(filename string, ts *Text) error {
	w, err := os.Create(filename)
	if err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	debugText(ts, enc)
	enc.Flush()
	return w.Close()
}

// DebugText returns an XML representation of the Text structure.
func DebugText(ts *Text) string {
	w := new(bytes.Buffer)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	debugText(ts, enc)
	enc.Flush()
	w.WriteString("\n")
	return w.String()
}

func debugText(ts *Text, enc *xml.Encoder) {
	var err error
	start := xml.StartElement{}
	start.Name = xml.Name{Local: "text"}
	if dbg, ok := ts.Settings[SettingDebug]; ok {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "debug"}, Value: fmt.Sprint(dbg)})
	}
	fields := []int{}
	for field := range ts.Settings {
		fields = append(fields, int(field))
	}
	sort.Ints(fields)

	for _, field := range fields {
		k := SettingType(field)
		v := ts.Settings[k]
		showSetting := true
		switch t := v.(type) {
		case int:
			if t == 0 {
				showSetting = false
			}
		case []string:
			if len(t) == 0 {
				showSetting = false
			}
		case uint:
			if t == 0 {
				showSetting = false
			}
		case bag.ScaledPoint:
			if t == 0 {
				showSetting = false
			}
		case *color.Color:
			if t == nil {
				showSetting = false
			}
		case BorderStyle:
			if t == 0 {
				showSetting = false
			}
		case TextDecorationLine:
			if t == 0 {
				showSetting = false
			}
		case HangingPunctuation:
			if t == 0 {
				showSetting = false
			}
		case FontWeight:
			if t == 0 {
				showSetting = false
			}
		case *FontFamily:
			if t == nil {
				showSetting = false
			} else {
				v = t.Name
			}
		}
		if k == SettingPrepend {
			v = node.StringValue(v.(node.Node))
		} else if k == SettingPreserveWhitespace {
			if v == false {
				showSetting = false
			}
		} else if k == SettingTabSizeSpaces {
			if v == 4 {
				showSetting = false
			}
		} else if k == SettingDebug {
			showSetting = false
		}
		if showSetting {
			start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: strings.TrimPrefix(fmt.Sprint(k), "Setting")}, Value: fmt.Sprint(v)})
		}
	}
	if err = enc.EncodeToken(start); err != nil {
		panic(err)
	}
	for _, itm := range ts.Items {
		switch t := itm.(type) {
		case *Text:
			debugText(t, enc)
		case *Table:
			startTable := xml.StartElement{Name: xml.Name{Local: "Table"}}
			enc.EncodeToken(startTable)
			for _, row := range t.Rows {
				startRow := xml.StartElement{Name: xml.Name{Local: "Row"}}
				enc.EncodeToken(startRow)

				for _, col := range row.Cells {
					startCell := xml.StartElement{Name: xml.Name{Local: "Cell"}}
					enc.EncodeToken(startCell)
					for _, v := range col.Contents {
						switch t := v.(type) {
						case *Text:
							debugText(t, enc)
						default:
							startUnknown := xml.StartElement{Name: xml.Name{Local: "Unknown"}}
							enc.EncodeToken(startUnknown)
							enc.EncodeToken(startUnknown.End())

						}
					}
					enc.EncodeToken(startCell.End())
				}
				enc.EncodeToken(startRow.End())
			}
			enc.EncodeToken(startTable.End())
		case string:
			enc.EncodeToken(xml.CharData(t))
		case *node.VList:
			enc.EncodeToken(xml.CharData(node.DebugToString(t)))
		case *image.Image:
			startImg := xml.StartElement{}
			startImg.Name = xml.Name{Local: "Image"}
			enc.EncodeToken(startImg)
			enc.EncodeToken(startImg.End())
		default:
			panic(fmt.Sprintf("unknown type %T", t))
		}
	}
	if err = enc.EncodeToken(start.End()); err != nil {
		panic(err)
	}
}

func (ts *Text) String() string {
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

// Options collects the TypesettingOption for FormatParagraph.
type Options struct {
	Alignment      HorizontalAlignment
	Fontfamily     *FontFamily
	Fontsize       bag.ScaledPoint
	hsize          bag.ScaledPoint
	IndentLeft     bag.ScaledPoint
	IndentLeftRows int
	Language       *lang.Lang
	Leading        bag.ScaledPoint
}

// TypesettingOption controls the formatting of the paragraph.
type TypesettingOption func(*Options)

// Leading sets the distance between two baselines in a paragraph.
func Leading(leading bag.ScaledPoint) TypesettingOption {
	return func(p *Options) {
		p.Leading = leading
	}
}

// Language sets the default language for the whole paragraph (used for
// hyphenation).
func Language(language *lang.Lang) TypesettingOption {
	return func(p *Options) {
		p.Language = language
	}
}

// FontSize sets the font size for the paragraph.
func FontSize(size bag.ScaledPoint) TypesettingOption {
	return func(p *Options) {
		p.Fontsize = size
	}
}

// IndentLeft sets the left indent.
func IndentLeft(size bag.ScaledPoint, rows int) TypesettingOption {
	return func(p *Options) {
		p.IndentLeft = size
		p.IndentLeftRows = rows
	}
}

// Family sets the font family for the paragraph.
func Family(fam *FontFamily) TypesettingOption {
	return func(p *Options) {
		p.Fontfamily = fam
	}
}

// HorizontalAlign sets the horizontal alignment for a paragraph.
func HorizontalAlign(a HorizontalAlignment) TypesettingOption {
	return func(p *Options) {
		p.Alignment = a
	}
}

// ParagraphInfo contains information about the whole paragraph and each line.
type ParagraphInfo struct {
	Height bag.ScaledPoint
	Depth  bag.ScaledPoint
	Widths []bag.ScaledPoint
}

// FormatParagraph creates a rectangular text from the data stored in the
// Paragraph.
func (fe *Document) FormatParagraph(te *Text, hsize bag.ScaledPoint, opts ...TypesettingOption) (*node.VList, *ParagraphInfo, error) {
	bag.Logger.Log(nil, -8, "FormatParagraph")
	if len(te.Items) == 0 {
		g := node.NewGlue()
		g.Attributes = node.H{"origin": "empty list in FormatParagraph"}
		return node.Vpack(g), nil, nil
	}
	p := &Options{
		Language: fe.Doc.DefaultLanguage,
		hsize:    hsize,
	}
	if ha, ok := te.Settings[SettingHAlign]; ok {
		if ha != nil {
			p.Alignment = ha.(HorizontalAlignment)
		} else {
			p.Alignment = HAlignDefault
		}
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.Fontsize != 0 {
		te.Settings[SettingSize] = p.Fontsize
	}
	if p.Fontfamily != nil {
		te.Settings[SettingFontFamily] = p.Fontfamily
	}
	pi := ParagraphInfo{}
	if len(te.Items) > 0 {
		if tbl, ok := te.Items[0].(*Table); ok {
			if sWd, ok := te.Settings[SettingWidth]; ok {
				if wd, ok := sWd.(string); ok {
					if wd == "100%" {
						tbl.Stretch = true
					}
				}
			}
			tbl.MaxWidth = hsize
			vls, err := fe.BuildTable(tbl)
			for _, vl := range vls {
				pi.Widths = append(pi.Widths, vl.Width)
				pi.Height += vl.Height
			}
			if err != nil {
				return nil, &pi, err
			}
			vl := vls[0]
			return vl, &pi, nil
		}
	}
	var hlist, tail node.Node
	var err error

	hlist, tail, err = fe.Mknodes(te)
	if err != nil {
		return nil, nil, err
	}
	if hlist == nil {
		return node.NewVList(), nil, nil
	}

	// A single start stop node (like a PDF dest)
	if _, ok := hlist.(*node.StartStop); ok && hlist.Next() == nil {
		return node.Vpack(hlist), nil, nil
	}

	Hyphenate(hlist, p.Language)
	node.AppendLineEndAfter(hlist, tail)

	ls := node.NewLinebreakSettings()
	ls.HSize = p.hsize
	ls.Indent = p.IndentLeft
	ls.IndentRows = p.IndentLeftRows
	ls.Tolerance = 4
	if hp, ok := te.Settings[SettingHangingPunctuation]; ok {
		if hps, ok := hp.(HangingPunctuation); ok {
			ls.HangingPunctuationEnd = hps&HangingPunctuationAllowEnd == 1
		}
	}

	if fe, ok := te.Settings[SettingFontExpansion]; ok {
		if fef, ok := fe.(float64); ok {
			ls.FontExpansion = fef
		}
	}

	if p.Leading == 0 {
		if l, ok := te.Settings[SettingLeading]; ok {
			ls.LineHeight = l.(bag.ScaledPoint)
		} else {
			ls.LineHeight = p.Fontsize * 120 / 100
		}
	} else {
		ls.LineHeight = p.Leading
	}
	if p.Alignment == HAlignLeft || p.Alignment == HAlignCenter {
		lg := node.NewGlue()
		lg.Stretch = bag.Factor
		lg.StretchOrder = 3
		lg.Subtype = node.GlueLineEnd
		ls.LineEndGlue = lg
	}
	if p.Alignment == HAlignRight || p.Alignment == HAlignCenter {
		lg := node.NewGlue()
		lg.Stretch = bag.Factor
		lg.StretchOrder = 3
		lg.Subtype = node.GlueLineStart
		ls.LineStartGlue = lg
	}
	vlist, info := node.Linebreak(hlist, ls)
	for _, inf := range info {
		pi.Widths = append(pi.Widths, inf.Width)
	}

	for _, cb := range fe.postLinebreakCallback {
		vlist = cb(vlist)
	}
	if htt, ok := te.Settings[SettingHeight]; ok {
		if ht, ok := htt.(bag.ScaledPoint); ok {
			moreHeight := ht - vlist.Height - vlist.Depth
			topGlue := node.NewGlue()
			bottomGlue := node.NewGlue()
			var valign = VAlignMiddle
			if vat, ok := te.Settings[SettingVAlign]; ok {
				if va, ok := vat.(VerticalAlignment); ok {
					valign = va
				}
			}
			switch valign {
			case VAlignTop:
				bottomGlue.Width = moreHeight
			case VAlignBottom:
				topGlue.Width = moreHeight
			default:
				bottomGlue.Width = moreHeight / 2
				topGlue.Width = moreHeight / 2
			}
			var head node.Node
			if topGlue.Width != 0 {
				head = topGlue
			}
			head = node.InsertAfter(head, head, vlist)
			if bottomGlue.Width != 0 {
				head = node.InsertAfter(head, vlist, bottomGlue)
			}
			vlist = node.Vpack(head)
			vlist.Attributes = node.H{"origin": "FormatParagraph, setHeight"}
		}
	}
	return vlist, &pi, nil
}

func parseHarfbuzzFontFeatures(featurelist any) []harfbuzz.Feature {
	fontfeatures := []harfbuzz.Feature{}
	switch t := featurelist.(type) {
	case string:
		for _, str := range strings.Split(t, ",") {
			f, err := harfbuzz.ParseFeature(str)
			if err != nil {
				// FIXME: proper error handling
				// logger.Error(fmt.Sprintf("Cannot parse OpenType feature tag %q.", str))
			}
			fontfeatures = append(fontfeatures, f)
		}
	case []string:
		for _, single := range t {
			for _, str := range strings.Split(single, ",") {
				f, err := harfbuzz.ParseFeature(str)
				if err != nil {
					// FIXME: proper error handling
					// logger.Error(fmt.Sprintf("Cannot parse OpenType feature tag %q.", str))
				}
				fontfeatures = append(fontfeatures, f)
			}

		}
	}
	return fontfeatures
}

// BuildNodelistFromString returns a node list containing glyphs from the string
// with the settings in ts.
func (fe *Document) BuildNodelistFromString(ts TypesettingSettings, str string) (node.Node, error) {
	bag.Logger.Log(nil, -8, "Document#BuildNodelistFromString")
	fontweight := FontWeight400
	fontstyle := FontStyleNormal
	var fontfamily *FontFamily
	fontsize := 12 * bag.Factor
	var col *color.Color
	var hyperlink document.Hyperlink
	var hasHyperlink bool
	var hasUnderline bool
	fontfeatures := make([]harfbuzz.Feature, 0, len(fe.DefaultFeatures))
	for _, f := range fe.DefaultFeatures {
		fontfeatures = append(fontfeatures, f)
	}
	preserveWhitespace := false
	yoffset := bag.ScaledPoint(0)
	var settingFontFeatures []harfbuzz.Feature
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
		case SettingTextDecorationLine:
			if underlineType, ok := v.(TextDecorationLine); ok && underlineType == TextDecorationUnderline {
				hasUnderline = true
			}
		case SettingFontExpansion:
			// ignore
		case SettingStyle:
			fontstyle = v.(FontStyle)
		case SettingOpenTypeFeature:
			settingFontFeatures = parseHarfbuzzFontFeatures(v)
		case SettingMarginTop, SettingMarginRight, SettingMarginBottom, SettingMarginLeft, SettingPaddingRight, SettingPaddingBottom, SettingPaddingTop, SettingPaddingLeft:
			// ignore
		case SettingHAlign, SettingLeading, SettingIndentLeft, SettingIndentLeftRows, SettingTabSize, SettingTabSizeSpaces:
			// ignore
		case SettingBorderBottomWidth, SettingBorderLeftWidth, SettingBorderRightWidth, SettingBorderTopWidth:
			// ignore
		case SettingBorderBottomColor, SettingBorderLeftColor, SettingBorderRightColor, SettingBorderTopColor:
			// ignore
		case SettingBorderBottomStyle, SettingBorderLeftStyle, SettingBorderRightStyle, SettingBorderTopStyle:
			// ignore
		case SettingBorderBottomLeftRadius, SettingBorderBottomRightRadius, SettingBorderTopLeftRadius, SettingBorderTopRightRadius:
			// ignore
		case SettingBackgroundColor, SettingPrepend, SettingDebug, SettingHeight, SettingVAlign, SettingHangingPunctuation:
			// ignore
		case SettingWidth, SettingBox:
			// ignore
		case SettingPreserveWhitespace:
			preserveWhitespace = v.(bool)
		case SettingYOffset:
			yoffset = v.(bag.ScaledPoint)
		default:
			return nil, fmt.Errorf("Unknown setting %v", k)
		}
	}

	var fnt *font.Font
	var face *pdf.Face
	var fs *FontSource
	var err error
	if fs, err = fontfamily.GetFontSource(fontweight, fontstyle); err != nil {
		return nil, err
	}
	bag.Logger.Log(nil, -8, "GetFontSource", "fs", fs.Name)
	// fs.SizeAdjust is CSS size-adjust normalized so that 0 = 100% and negative = shrinking.
	if fs.SizeAdjust != 0 {
		fontsize = bag.ScaledPointFromFloat(fontsize.ToPT() * (1 - fs.SizeAdjust))
	}
	// First the font source default features should get applied, then the
	// features from the current settings.
	fontfeatures = append(fontfeatures, parseHarfbuzzFontFeatures(fs.FontFeatures)...)
	fontfeatures = append(fontfeatures, settingFontFeatures...)
	if face, err = fe.LoadFace(fs); err != nil {
		if fs.Name == "" {
			bag.Logger.Error("Cannot load face", "location", fs.Location)
		} else {
			bag.Logger.Error("Cannot load face", "name", fs.Name)
		}
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
		head = hyperlinkStart
	}
	var underlineStart *node.StartStop
	if hasUnderline {
		underlineStart = node.NewStartStop()
		node.SetAttribute(underlineStart, "underline", true)
		node.SetAttribute(underlineStart, "underlinepos", -fontsize/6)
		node.SetAttribute(underlineStart, "underlinelw", fontsize/20)
		node.SetAttribute(underlineStart, "SettingTextDecorationLine", TextDecorationUnderline)
		if head != nil {
			head = node.InsertAfter(head, head, underlineStart)
		} else {
			head = underlineStart
		}
		underlineStart.Action = node.ActionUserSetting
	}
	if col != nil {
		colStart := node.NewStartStop()
		colStart.Position = node.PDFOutputPage
		colStart.ShipoutCallback = func(n node.Node) string {
			return col.PDFStringNonStroking() + " "
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
			if preserveWhitespace {
				switch r.Components {
				case " ":
					g := node.NewRule()
					g.Width = fnt.Space
					head = node.InsertAfter(head, cur, g)
					cur = g
					lastglue = g
				case "\t":
					// tab size...
					g := node.NewGlue()
					hasTabsize := false
					if wd, ok := ts[SettingTabSize]; ok {
						if tabsize, ok := wd.(bag.ScaledPoint); ok && tabsize > 0 {
							hasTabsize = true
							g.Width = bag.ScaledPoint(tabsize)
						}
					}
					if tw, ok := ts[SettingTabSizeSpaces]; ok && !hasTabsize {
						if nspaces, ok := tw.(int); ok {
							g.Width = bag.ScaledPoint(nspaces) * fnt.Space
							hasTabsize = true
						}
					}
					if !hasTabsize {
						g.Width = 4 * fnt.Space
					}
					head = node.InsertAfter(head, cur, g)
					cur = g
					lastglue = g
				case "\n":
					head, cur = node.AppendLineEndAfter(head, cur)
					lastglue = cur
				default:
					panic("unhandled whitespace type")
				}
			} else {
				if r.Components == "\n" {
					p1 := node.NewPenalty()
					p1.Penalty = 10000
					g := node.NewGlue()
					g.Stretch = bag.Factor
					g.StretchOrder = node.StretchFill
					p2 := node.NewPenalty()
					p2.Penalty = -10000
					head = node.InsertAfter(head, cur, p1)
					head = node.InsertAfter(head, p1, g)
					head = node.InsertAfter(head, g, p2)
					cur = p2
					lastglue = g
				}

				if lastglue == nil {
					g := node.NewGlue()
					g.Width = fnt.Space
					g.Stretch = fnt.SpaceStretch
					g.Shrink = fnt.SpaceShrink
					head = node.InsertAfter(head, cur, g)
					cur = g
					lastglue = g
				}
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
			n.YOffset = yoffset
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
		stop.ShipoutCallback = func(n node.Node) string {
			return "0 0 0 RG 0 0 0 rg "
		}
		node.InsertAfter(head, cur, stop)
		cur = stop
	}
	if hasUnderline {
		underlineStop := node.NewStartStop()
		underlineStop.StartNode = underlineStart
		node.SetAttribute(underlineStop, "underline", false)
		head = node.InsertAfter(head, cur, underlineStop)
		cur = underlineStop
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
func (fe *Document) Mknodes(ts *Text) (head node.Node, tail node.Node, err error) {
	bag.Logger.Log(nil, -8, "Document#Mknodes")
	if len(ts.Items) == 0 {
		return nil, nil, nil
	}
	var newSettings = make(TypesettingSettings)
	var nl, end node.Node
	for k, v := range ts.Settings {
		newSettings[k] = v
	}
	var hyperlinkStartNode *node.StartStop
	var hyperlinkDest string
	for _, itm := range ts.Items {
		switch t := itm.(type) {
		case string:
			if hyperlinkStartNode != nil {
				endHL := node.NewStartStop()
				endHL.Action = node.ActionNone
				endHL.StartNode = hyperlinkStartNode
				hyperlinkStartNode = nil
				node.InsertAfter(head, tail, endHL)
				tail = endHL
			}

			nl, err = fe.BuildNodelistFromString(newSettings, t)
			if err != nil {
				return nil, nil, err
			}

			if nl != nil {
				if pr, ok := ts.Settings[SettingPaddingRight]; ok {
					paddingRight := pr.(bag.ScaledPoint)
					if paddingRight > 0 {
						g := node.NewGlue()
						g.Width = paddingRight
						g.Attributes = node.H{"origin": "padding right"}
						node.InsertAfter(nl, node.Tail(nl), g)
					}
				}
				head = node.InsertAfter(head, tail, nl)
				tail = node.Tail(nl)
			}
		case *Text:
			if hyperlinkStartNode == nil {
				// we are within a hyperlink, so lets remove all startstop
				if hlSetting, ok := t.Settings[SettingHyperlink]; ok {
					hl := hlSetting.(document.Hyperlink)
					// insert a startstop with the hyperlink
					hyperlinkDest = hl.URI
					startHL := node.NewStartStop()
					startHL.Action = node.ActionHyperlink
					startHL.Value = &hl
					hyperlinkStartNode = startHL
					head = node.InsertAfter(head, tail, startHL)
					tail = startHL
				}
			} else {
				if hlSetting, ok := t.Settings[SettingHyperlink]; ok && hlSetting.(document.Hyperlink).URI == hyperlinkDest {
					// same destination
				} else {
					// probably no hyperlink, TODO: insert end startstop here?
				}
			}
			// copy current settings to the child if not already set.
			for k, v := range newSettings {
				if _, found := t.Settings[k]; !found {
					t.Settings[k] = v
				}
			}
			// we don't want to inherit hyperlinks
			delete(t.Settings, SettingHyperlink)
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
		case *Table:
			s := node.NewStartStop()
			s.Attributes = node.H{"table": t}
			head = node.InsertAfter(head, tail, s)
			tail = s
		default:
			return nil, nil, fmt.Errorf("Mknodes: unknown item type %T", t)
		}
	}
	if hyperlinkStartNode != nil {
		endHL := node.NewStartStop()
		endHL.Action = node.ActionNone
		endHL.StartNode = hyperlinkStartNode
		node.InsertAfter(head, tail, endHL)
		tail = endHL
	}
	return head, tail, nil
}
