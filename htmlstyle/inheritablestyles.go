package htmlstyle

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/color"
	"github.com/speedata/boxesandglue/backend/document"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/frontend"
	"golang.org/x/net/html"
)

var tenpt = bag.MustSp("10pt")
var tenptflt = bag.MustSp("10pt").ToPT()

// ParseVerticalAlign parses the input ("top","middle",...) and returns the
// VerticalAlignment value.
func ParseVerticalAlign(align string, styles *FormattingStyles) frontend.VerticalAlignment {
	switch align {
	case "top":
		return frontend.VAlignTop
	case "middle":
		return frontend.VAlignMiddle
	case "bottom":
		return frontend.VAlignBottom
	case "inherit":
		return styles.Valign
	default:
		return styles.Valign
	}
}

// ParseHorizontalAlign parses the input ("left","center") and returns the
// HorizontalAlignment value.
func ParseHorizontalAlign(align string, styles *FormattingStyles) frontend.HorizontalAlignment {
	switch align {
	case "left":
		return frontend.HAlignLeft
	case "center":
		return frontend.HAlignCenter
	case "right":
		return frontend.HAlignRight
	case "inherit":
		return styles.Halign
	default:
		return styles.Halign
	}
}

// ParseRelativeSize converts the string fs to a scaled point. This can be an
// absolute size like 12pt but also a size like 1.2 or 2em. The provided dflt is
// the source size. The root is the document's default value.
func ParseRelativeSize(fs string, cur bag.ScaledPoint, root bag.ScaledPoint) bag.ScaledPoint {
	if strings.HasSuffix(fs, "%") {
		p := strings.TrimSuffix(fs, "%")
		f, err := strconv.ParseFloat(p, 64)
		if err != nil {
			panic(err)
		}
		ret := bag.MultiplyFloat(cur, f/100)
		return ret
	}
	if strings.HasSuffix(fs, "rem") {
		if root == 0 {
			// logger.Warn("Calculating an rem size without a root font size results in a size of 0.")
			return 0
		}

		prefix := strings.TrimSuffix(fs, "rem")
		factor, err := strconv.ParseFloat(prefix, 32)
		if err != nil {
			// logger.Error(fmt.Sprintf("Cannot convert relative size %s", fs))
			return bag.MustSp("10pt")
		}
		return bag.ScaledPoint(float64(root) * factor)
	}
	if strings.HasSuffix(fs, "em") {
		if cur == 0 {
			// logger.Warn("Calculating an em size without a body font size results in a size of 0.")
			return 0
		}
		prefix := strings.TrimSuffix(fs, "em")
		factor, err := strconv.ParseFloat(prefix, 32)
		if err != nil {
			// logger.Error(fmt.Sprintf("Cannot convert relative size %s", fs))
			return bag.MustSp("10pt")
		}
		return bag.ScaledPoint(float64(cur) * factor)
	}
	if unit, err := bag.Sp(fs); err == nil {
		return unit
	}
	if factor, err := strconv.ParseFloat(fs, 64); err == nil {
		return bag.ScaledPointFromFloat(cur.ToPT() * factor)
	}
	switch fs {
	case "larger":
		return bag.ScaledPointFromFloat(cur.ToPT() * 1.2)
	case "smaller":
		return bag.ScaledPointFromFloat(cur.ToPT() / 1.2)
	case "xx-small":
		return bag.ScaledPointFromFloat(tenptflt / 1.2 / 1.2 / 1.2)
	case "x-small":
		return bag.ScaledPointFromFloat(tenptflt / 1.2 / 1.2)
	case "small":
		return bag.ScaledPointFromFloat(tenptflt / 1.2)
	case "medium":
		return tenpt
	case "large":
		return bag.ScaledPointFromFloat(tenptflt * 1.2)
	case "x-large":
		return bag.ScaledPointFromFloat(tenptflt * 1.2 * 1.2)
	case "xx-large":
		return bag.ScaledPointFromFloat(tenptflt * 1.2 * 1.2 * 1.2)
	case "xxx-large":
		return bag.ScaledPointFromFloat(tenptflt * 1.2 * 1.2 * 1.2 * 1.2)
	}
	// logger.Error(fmt.Sprintf("Could not convert %s from default %s", fs, cur))
	return cur
}

// StylesToStyles updates the inheritable formattingStyles from the attributes
// (of the current HTML element).
func StylesToStyles(ih *FormattingStyles, attributes map[string]string, df *frontend.Document, curFontSize bag.ScaledPoint) error {
	// Resolve font size first, since some of the attributes depend on the
	// current font size.
	if v, ok := attributes["font-size"]; ok {
		ih.Fontsize = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
	}
	for k, v := range attributes {
		switch k {
		case "font-size":
			// already set
		case "hyphens":
			// ignore for now
		case "display":
			ih.Hide = (v == "none")
		case "background-color":
			ih.BackgroundColor = df.GetColor(v)
		case "border-right-width", "border-left-width", "border-top-width", "border-bottom-width":
			size := ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
			switch k {
			case "border-right-width":
				ih.BorderRightWidth = size
			case "border-left-width":
				ih.BorderLeftWidth = size
			case "border-top-width":
				ih.BorderTopWidth = size
			case "border-bottom-width":
				ih.BorderBottomWidth = size
			}
		case "border-top-right-radius", "border-top-left-radius", "border-bottom-right-radius", "border-bottom-left-radius":
			size := ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
			switch k {
			case "border-top-right-radius":
				ih.BorderTopRightRadius = size
			case "border-top-left-radius":
				ih.BorderTopLeftRadius = size
			case "border-bottom-left-radius":
				ih.BorderBottomLeftRadius = size
			case "border-bottom-right-radius":
				ih.BorderBottomRightRadius = size
			}
		case "border-right-style", "border-left-style", "border-top-style", "border-bottom-style":
			var sty frontend.BorderStyle
			switch v {
			case "none":
				// default
			case "solid":
				sty = frontend.BorderStyleSolid
			default:
				// logger.Error(fmt.Sprintf("not implemented: border style %q", v))
			}
			switch k {
			case "border-right-style":
				ih.BorderRightStyle = sty
			case "border-left-style":
				ih.BorderLeftStyle = sty
			case "border-top-style":
				ih.BorderTopStyle = sty
			case "border-bottom-style":
				ih.BorderBottomStyle = sty
			}

		case "border-right-color":
			ih.BorderRightColor = df.GetColor(v)
		case "border-left-color":
			ih.BorderLeftColor = df.GetColor(v)
		case "border-top-color":
			ih.BorderTopColor = df.GetColor(v)
		case "border-bottom-color":
			ih.BorderBottomColor = df.GetColor(v)
		case "border-spacing":
			// ignore
		case "color":
			ih.color = df.GetColor(v)
		case "content":
			// ignore
		case "font-style":
			switch v {
			case "italic":
				ih.fontstyle = frontend.FontStyleItalic
			case "normal":
				ih.fontstyle = frontend.FontStyleNormal
			}
		case "font-weight":
			ih.Fontweight = frontend.ResolveFontWeight(v, ih.Fontweight)
		case "font-feature-settings":
			ih.fontfeatures = append(ih.fontfeatures, v)
		case "list-style-type":
			ih.ListStyleType = v
		case "font-family":
			v = strings.Trim(v, `"`)
			ih.fontfamily = df.FindFontFamily(v)
			if ih.fontfamily == nil {
				return fmt.Errorf("font family %q not found", v)
			}
		case "hanging-punctuation":
			switch v {
			case "allow-end":
				ih.hangingPunctuation = frontend.HangingPunctuationAllowEnd
			}
		case "line-height":
			ih.lineheight = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
		case "margin-bottom":
			ih.marginBottom = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
		case "margin-left":
			ih.marginLeft = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
		case "margin-right":
			ih.marginRight = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
		case "margin-top":
			ih.marginTop = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
		case "padding-inline-start":
			ih.paddingInlineStart = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
		case "padding-bottom":
			ih.PaddingBottom = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
		case "padding-left":
			ih.PaddingLeft = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
		case "padding-right":
			ih.PaddingRight = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
		case "padding-top":
			ih.PaddingTop = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
		case "tab-size":
			if ts, err := strconv.Atoi(v); err == nil {
				ih.tabsizeSpaces = ts
			} else {
				ih.tabsize = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
			}
		case "text-align":
			ih.Halign = ParseHorizontalAlign(v, ih)
		case "text-decoration-style":
			// not yet implemented
		case "text-decoration-line":
			switch v {
			case "underline":
				ih.TextDecorationLine = frontend.TextDecorationUnderline
			}
		case "text-indent":
			ih.indent = ParseRelativeSize(v, curFontSize, ih.DefaultFontSize)
			ih.indentRows = 1
		case "user-select":
			// ignore
		case "vertical-align":
			if v == "sub" {
				ih.yoffset = -1 * ih.Fontsize * 1000 / 5000
			} else if v == "super" {
				ih.yoffset = ih.Fontsize * 1000 / 5000
			}
		case "width":
			ih.width = v
		case "white-space":
			ih.preserveWhitespace = (v == "pre")
		case "-bag-font-expansion":
			if strings.HasSuffix(v, "%") {
				p := strings.TrimSuffix(v, "%")
				f, err := strconv.ParseFloat(p, 64)
				if err != nil {
					return err
				}
				fe := f / 100
				ih.fontexpansion = &fe
			}
		default:
			fmt.Println("unresolved attribute", k, v)
		}
	}
	return nil
}

// FormattingStyles are HTML formatting styles.
type FormattingStyles struct {
	BackgroundColor         *color.Color
	BorderLeftWidth         bag.ScaledPoint
	BorderRightWidth        bag.ScaledPoint
	BorderBottomWidth       bag.ScaledPoint
	BorderTopWidth          bag.ScaledPoint
	BorderTopLeftRadius     bag.ScaledPoint
	BorderTopRightRadius    bag.ScaledPoint
	BorderBottomLeftRadius  bag.ScaledPoint
	BorderBottomRightRadius bag.ScaledPoint
	BorderLeftColor         *color.Color
	BorderRightColor        *color.Color
	BorderBottomColor       *color.Color
	BorderTopColor          *color.Color
	BorderLeftStyle         frontend.BorderStyle
	BorderRightStyle        frontend.BorderStyle
	BorderBottomStyle       frontend.BorderStyle
	BorderTopStyle          frontend.BorderStyle
	DefaultFontSize         bag.ScaledPoint
	DefaultFontFamily       *frontend.FontFamily
	color                   *color.Color
	Hide                    bool
	fontfamily              *frontend.FontFamily
	fontfeatures            []string
	Fontsize                bag.ScaledPoint
	fontstyle               frontend.FontStyle
	Fontweight              frontend.FontWeight
	fontexpansion           *float64
	Halign                  frontend.HorizontalAlignment
	hangingPunctuation      frontend.HangingPunctuation
	indent                  bag.ScaledPoint
	indentRows              int
	language                string
	lineheight              bag.ScaledPoint
	ListStyleType           string
	marginBottom            bag.ScaledPoint
	marginLeft              bag.ScaledPoint
	marginRight             bag.ScaledPoint
	marginTop               bag.ScaledPoint
	paddingInlineStart      bag.ScaledPoint
	OlCounter               int
	PaddingBottom           bag.ScaledPoint
	PaddingLeft             bag.ScaledPoint
	PaddingRight            bag.ScaledPoint
	PaddingTop              bag.ScaledPoint
	TextDecorationLine      frontend.TextDecorationLine
	preserveWhitespace      bool
	tabsize                 bag.ScaledPoint
	tabsizeSpaces           int
	Valign                  frontend.VerticalAlignment
	width                   string
	yoffset                 bag.ScaledPoint
}

// Clone mimics style inheritance.
func (is *FormattingStyles) Clone() *FormattingStyles {
	// inherit
	newFontFeatures := make([]string, len(is.fontfeatures))
	for i, f := range is.fontfeatures {
		newFontFeatures[i] = f
	}
	newis := &FormattingStyles{
		color:              is.color,
		DefaultFontSize:    is.DefaultFontSize,
		DefaultFontFamily:  is.DefaultFontFamily,
		fontexpansion:      is.fontexpansion,
		fontfamily:         is.fontfamily,
		fontfeatures:       newFontFeatures,
		Fontsize:           is.Fontsize,
		fontstyle:          is.fontstyle,
		Fontweight:         is.Fontweight,
		hangingPunctuation: is.hangingPunctuation,
		language:           is.language,
		lineheight:         is.lineheight,
		ListStyleType:      is.ListStyleType,
		OlCounter:          is.OlCounter,
		preserveWhitespace: is.preserveWhitespace,
		tabsize:            is.tabsize,
		tabsizeSpaces:      is.tabsizeSpaces,
		Valign:             is.Valign,
		Halign:             is.Halign,
	}
	return newis
}

// ApplySettings converts the inheritable settings to boxes and glue text
// settings.
func ApplySettings(settings frontend.TypesettingSettings, ih *FormattingStyles) {
	if ih.Fontweight > 0 {
		settings[frontend.SettingFontWeight] = ih.Fontweight
	}
	settings[frontend.SettingBackgroundColor] = ih.BackgroundColor
	settings[frontend.SettingBorderTopWidth] = ih.BorderTopWidth
	settings[frontend.SettingBorderLeftWidth] = ih.BorderLeftWidth
	settings[frontend.SettingBorderRightWidth] = ih.BorderRightWidth
	settings[frontend.SettingBorderBottomWidth] = ih.BorderBottomWidth
	settings[frontend.SettingBorderTopColor] = ih.BorderTopColor
	settings[frontend.SettingBorderLeftColor] = ih.BorderLeftColor
	settings[frontend.SettingBorderRightColor] = ih.BorderRightColor
	settings[frontend.SettingBorderBottomColor] = ih.BorderBottomColor
	settings[frontend.SettingBorderTopStyle] = ih.BorderTopStyle
	settings[frontend.SettingBorderLeftStyle] = ih.BorderLeftStyle
	settings[frontend.SettingBorderRightStyle] = ih.BorderRightStyle
	settings[frontend.SettingBorderBottomStyle] = ih.BorderBottomStyle
	settings[frontend.SettingBorderTopLeftRadius] = ih.BorderTopLeftRadius
	settings[frontend.SettingBorderTopRightRadius] = ih.BorderTopRightRadius
	settings[frontend.SettingBorderBottomLeftRadius] = ih.BorderBottomLeftRadius
	settings[frontend.SettingBorderBottomRightRadius] = ih.BorderBottomRightRadius
	settings[frontend.SettingColor] = ih.color
	if ih.fontexpansion != nil {
		settings[frontend.SettingFontExpansion] = *ih.fontexpansion
	} else {
		settings[frontend.SettingFontExpansion] = 0.05
	}
	settings[frontend.SettingFontFamily] = ih.fontfamily
	settings[frontend.SettingHAlign] = ih.Halign
	settings[frontend.SettingHangingPunctuation] = ih.hangingPunctuation
	settings[frontend.SettingIndentLeft] = ih.indent
	settings[frontend.SettingIndentLeftRows] = ih.indentRows
	settings[frontend.SettingLeading] = ih.lineheight
	settings[frontend.SettingMarginBottom] = ih.marginBottom
	settings[frontend.SettingMarginRight] = ih.marginRight
	settings[frontend.SettingMarginLeft] = ih.marginLeft
	settings[frontend.SettingMarginTop] = ih.marginTop
	settings[frontend.SettingOpenTypeFeature] = ih.fontfeatures
	settings[frontend.SettingPaddingRight] = ih.PaddingRight
	settings[frontend.SettingPaddingLeft] = ih.PaddingLeft
	settings[frontend.SettingPaddingTop] = ih.PaddingTop
	settings[frontend.SettingPaddingBottom] = ih.PaddingBottom
	settings[frontend.SettingPreserveWhitespace] = ih.preserveWhitespace
	settings[frontend.SettingSize] = ih.Fontsize
	settings[frontend.SettingStyle] = ih.fontstyle
	settings[frontend.SettingYOffset] = ih.yoffset
	settings[frontend.SettingTabSize] = ih.tabsize
	settings[frontend.SettingTabSizeSpaces] = ih.tabsizeSpaces
	settings[frontend.SettingTextDecorationLine] = ih.TextDecorationLine

	if ih.width != "" {
		settings[frontend.SettingWidth] = ih.width
	}

}

// StylesStack mimics CSS style inheritance.
type StylesStack []*FormattingStyles

// PushStyles creates a new style instance, pushes it onto the stack and returns
// the new style.
func (ss *StylesStack) PushStyles() *FormattingStyles {
	var is *FormattingStyles
	if len(*ss) == 0 {
		is = &FormattingStyles{}
	} else {
		is = (*ss)[len(*ss)-1].Clone()
	}
	*ss = append(*ss, is)
	return is
}

// PopStyles removes the top style from the stack.
func (ss *StylesStack) PopStyles() {
	*ss = (*ss)[:len(*ss)-1]
}

// CurrentStyle returns the current style from the stack. CurrentStyle does not
// change the stack.
func (ss StylesStack) CurrentStyle() *FormattingStyles {
	return ss[len(ss)-1]
}

// SetDefaultFontFamily sets the font family that should be used as a default
// for the document.
func (ss *StylesStack) SetDefaultFontFamily(ff *frontend.FontFamily) {
	for _, sty := range *ss {
		sty.DefaultFontFamily = ff
	}
}

// SetDefaultFontSize sets the document font size which should be used for rem
// calculation.
func (ss *StylesStack) SetDefaultFontSize(size bag.ScaledPoint) {
	for _, sty := range *ss {
		sty.DefaultFontSize = size
	}
}

// Output turns HTML structure into a nested frontend.Text element.
func Output(item *HTMLItem, ss StylesStack, df *frontend.Document) (*frontend.Text, error) {
	// item is guaranteed to be in vertical direction
	newte := frontend.NewText()
	styles := ss.PushStyles()
	if err := StylesToStyles(styles, item.Styles, df, ss.CurrentStyle().Fontsize); err != nil {
		return nil, err
	}
	ApplySettings(newte.Settings, styles)
	newte.Settings[frontend.SettingDebug] = item.Data
	switch item.Data {
	case "html":
		if fs, ok := item.Styles["font-size"]; ok {
			rfs := ParseRelativeSize(fs, 0, 0)
			ss.SetDefaultFontSize(rfs)
		}
		if ffs, ok := item.Styles["font-family"]; ok {
			ff := df.FindFontFamily(ffs)
			ss.SetDefaultFontFamily(ff)
		}
	case "body":
		if ffs, ok := item.Styles["font-family"]; ok {
			ff := df.FindFontFamily(ffs)
			ss.SetDefaultFontFamily(ff)
		}
	case "table":
		tbl, err := processTable(item, ss, df)
		ss.PopStyles()
		if err != nil {
			return nil, err
		}
		newte.Items = append(newte.Items, tbl)
		return newte, nil
	case "ol", "ul":
		styles.OlCounter = 0
	case "li":
		var item string
		if strings.HasPrefix(styles.ListStyleType, `"`) && strings.HasSuffix(styles.ListStyleType, `"`) {
			item = strings.TrimPrefix(styles.ListStyleType, `"`)
			item = strings.TrimSuffix(item, `"`)
		} else {
			switch styles.ListStyleType {
			case "disc":
				item = "•"
			case "circle":
				item = "◦"
			case "none":
				item = ""
			case "square":
				item = "□"
			case "decimal":
				item = fmt.Sprintf("%d.", styles.OlCounter)
			default:
				// logger.Error(fmt.Sprintf("unhandled list-style-type: %q", styles.ListStyleType))
				item = "•"
			}
			item += " "
		}
		n, err := df.BuildNodelistFromString(newte.Settings, item)
		if err != nil {
			return nil, err
		}
		newte.Settings[frontend.SettingPrepend] = n
	}

	var te *frontend.Text
	cur := ModeVertical

	// display = "none"
	if styles.Hide {
		ss.PopStyles()
		return newte, nil
	}

	for _, itm := range item.Children {
		if itm.Dir == ModeHorizontal {
			// Going from vertical to horizontal.
			if cur == ModeVertical && itm.Data == " " {
				// there is only a whitespace element.
				continue
			}
			// now in horizontal mode, there can be more children in horizontal
			// mode, so append all of them to a single frontend.Text element
			if itm.Typ == html.TextNode && cur == ModeVertical {
				itm.Data = strings.TrimLeft(itm.Data, " ")
			}
			if te == nil {
				te = frontend.NewText()
				styles = ss.PushStyles()
			}
			ApplySettings(te.Settings, styles)
			if err := collectHorizontalNodes(te, itm, ss, ss.CurrentStyle().Fontsize, ss.CurrentStyle().DefaultFontSize, df); err != nil {
				return nil, err
			}
			cur = ModeHorizontal
		} else {
			// still vertical
			if itm.Data == "li" {
				styles.OlCounter++
			}
			if te != nil {
				newte.Items = append(newte.Items, te)
				newte.Settings[frontend.SettingBox] = true
				te = nil
			}
			te, err := Output(itm, ss, df)
			if err != nil {
				return nil, err
			}
			if len(te.Items) > 0 {
				newte.Items = append(newte.Items, te)
			}
		}
	}
	if item.Dir == ModeVertical && cur == ModeVertical {
		newte.Settings[frontend.SettingBox] = true
	}
	switch item.Data {
	case "ul", "ol":
		ulte := frontend.NewText()
		ApplySettings(ulte.Settings, styles)
		ulte.Settings[frontend.SettingDebug] = item.Data
		ulte.Settings[frontend.SettingBox] = true
	}
	if te != nil {
		newte.Items = append(newte.Items, te)
		ss.PopStyles()
		te = nil
	}
	ss.PopStyles()
	return newte, nil
}

func collectHorizontalNodes(te *frontend.Text, item *HTMLItem, ss StylesStack, currentFontsize bag.ScaledPoint, defaultFontsize bag.ScaledPoint, df *frontend.Document) error {
	switch item.Typ {
	case html.TextNode:
		te.Items = append(te.Items, item.Data)
	case html.ElementNode:
		childSettings := make(frontend.TypesettingSettings)
		switch item.Data {
		case "a":
			var href string
			for k, v := range item.Attributes {
				switch k {
				case "href":
					href = v
				}
			}
			hl := document.Hyperlink{URI: href}
			childSettings[frontend.SettingHyperlink] = hl
		case "img":
			wd := bag.MustSp("3cm")
			ht := wd
			var filename string
			for k, v := range item.Attributes {
				switch k {
				case "width":
					wd = bag.MustSp(v)
				case "height":
					ht = bag.MustSp(v)
				case "src":
					filename = v
				}
			}
			imgfile, err := df.Doc.LoadImageFile(filename)
			if err != nil {
				panic(err)
			}

			ii := df.Doc.CreateImage(imgfile, 1, "/MediaBox")
			imgNode := node.NewImage()
			imgNode.Img = ii
			imgNode.Width = wd
			imgNode.Height = ht
			hlist := node.Hpack(imgNode)
			te.Items = append(te.Items, hlist)
		}

		for _, itm := range item.Children {
			cld := frontend.NewText()
			sty := ss.PushStyles()
			if err := StylesToStyles(sty, item.Styles, df, currentFontsize); err != nil {
				return err
			}
			ApplySettings(cld.Settings, sty)
			for k, v := range childSettings {
				cld.Settings[k] = v
			}
			if err := collectHorizontalNodes(cld, itm, ss, currentFontsize, defaultFontsize, df); err != nil {
				return err
			}
			te.Items = append(te.Items, cld)
			ss.PopStyles()
		}
	}
	return nil
}
