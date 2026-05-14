package frontend

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"maps"
	"os"
	"sort"
	"strings"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/document"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/lang"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/boxesandglue/backend/text/bidi"
	"github.com/boxesandglue/textshape/ot"
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
	// SettingColumnWidth sets the width of a table column (for col elements).
	SettingColumnWidth
	// SettingColspan sets the number of columns a table cell spans.
	SettingColspan
	// SettingDebug can contain debugging information
	SettingDebug
	// SettingDest defines a named PDF destination (anchor) for internal links.
	SettingDest
	// SettingFontExpansion is the amount of expansion / shrinkage allowed. Value is a float between 0 (no expansion) and 1 (100% of the glyph width).
	SettingFontExpansion
	// SettingFontFamily selects a font family.
	SettingFontFamily
	// SettingFontWeight represents a font weight setting.
	SettingFontWeight
	// SettingFontVariationSettings contains variable font axis values (e.g., "wght" -> 700).
	SettingFontVariationSettings
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
	// SettingLeader contains the leader pattern string (e.g. ".") for TeX-style leaders.
	SettingLeader
	// SettingLeading determines the distance between two base lines (line height).
	SettingLeading
	// SettingLetterSpacing adds extra space between glyphs (CSS letter-spacing).
	SettingLetterSpacing
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
	// SettingPageBreakAfter controls page breaking after this element ("auto", "always", "avoid").
	SettingPageBreakAfter
	// SettingPageBreakBefore controls page breaking before this element ("auto", "always", "avoid").
	SettingPageBreakBefore
	// SettingPaddingBottom is the bottom padding.
	SettingPaddingBottom
	// SettingPaddingLeft is the left hand padding.
	SettingPaddingLeft
	// SettingPaddingRight is the right hand padding.
	SettingPaddingRight
	// SettingPaddingTop is the top padding.
	SettingPaddingTop
	// SettingPrerenderedVListID references a pre-rendered VList stored in CSSBuilder.PendingVLists.
	SettingPrerenderedVListID
	// SettingPrepend contains a node list which should be prepended to the list.
	SettingPrepend
	// SettingPreserveWhitespace makes a monospace paragraph with newlines.
	SettingPreserveWhitespace
	// SettingRowspan sets the number of rows a table cell spans.
	SettingRowspan
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
	// SettingDirection sets the writing direction of a paragraph (DirectionLTR
	// or DirectionRTL). If unset, the direction is auto-detected from the
	// paragraph content using the Unicode Bidi algorithm (UAX#9).
	SettingDirection
	// SettingLanguage selects the hyphenation language for a Text element. The
	// value is a *lang.Lang. When set on a sub-Text whose language differs
	// from the parent, Mknodes emits node.Lang markers around the sub-Text so
	// the Hyphenate walker switches patterns for that run. CSS-level
	// `hyphens: none` is encoded by setting SettingLanguage to a no-op
	// language (see frontend.GetLanguage for unknown tags).
	SettingLanguage
	// SettingHyphens carries the CSS Text 3 `hyphens` keyword (string:
	// "auto", "manual", or "none"). It controls only the soft-hyphen
	// (U+00AD) behaviour; pattern-based hyphenation is governed by
	// SettingLanguage. "auto" / unset / "manual" insert a Disc at every
	// U+00AD; "none" drops U+00AD silently.
	SettingHyphens
	// SettingHyphenPenalty carries the Knuth-Plass hyphen penalty as int.
	// Lower values encourage hyphenation, higher discourage. TeX default
	// is 50; lower (5–25) helps long compound words in German typesetting,
	// higher (200+) discourages hyphenation in narrow English columns.
	SettingHyphenPenalty
	// SettingLinebreakTolerance carries the Knuth-Plass tolerance (badness
	// ceiling) as float64. Higher values accept looser lines. TeX defaults
	// to 200 for \fussy and 10000 for \sloppy. The boxesandglue default is
	// conservative (4); set higher per paragraph or per element to allow
	// the line breaker more flexibility (e.g. for narrow columns or text
	// with few hyphenation opportunities).
	SettingLinebreakTolerance
	// SettingLinebreakEmergencyStretch carries TeX's \emergencystretch as
	// bag.ScaledPoint. It adds a per-line stretch budget that the breaker
	// may draw on as a last resort, so a feasible underfull break can be
	// found when no natural stretch is available on the line (e.g. very
	// long words in narrow columns). Default 0 disables it. Typical values
	// are 1–3em of the body font size.
	SettingLinebreakEmergencyStretch
)

// Direction describes the writing direction of a paragraph.
type Direction int

const (
	// DirectionLTR is left-to-right writing direction.
	DirectionLTR Direction = iota
	// DirectionRTL is right-to-left writing direction (Hebrew, Arabic, ...).
	DirectionRTL
)

func (d Direction) String() string {
	switch d {
	case DirectionLTR:
		return "ltr"
	case DirectionRTL:
		return "rtl"
	default:
		return fmt.Sprintf("Direction(%d)", int(d))
	}
}

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
	case SettingColumnWidth:
		settingName = "SettingColumnWidth"
	case SettingColspan:
		settingName = "SettingColspan"
	case SettingDebug:
		settingName = "SettingDebug"
	case SettingDest:
		settingName = "SettingDest"
	case SettingFontExpansion:
		settingName = "SettingFontExpansion"
	case SettingFontFamily:
		settingName = "SettingFontFamily"
	case SettingFontWeight:
		settingName = "SettingFontWeight"
	case SettingFontVariationSettings:
		settingName = "SettingFontVariationSettings"
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
	case SettingLeader:
		settingName = "SettingLeader"
	case SettingLeading:
		settingName = "SettingLeading"
	case SettingLetterSpacing:
		settingName = "SettingLetterSpacing"
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
	case SettingPageBreakAfter:
		settingName = "SettingPageBreakAfter"
	case SettingPageBreakBefore:
		settingName = "SettingPageBreakBefore"
	case SettingPaddingBottom:
		settingName = "SettingPaddingBottom"
	case SettingPaddingLeft:
		settingName = "SettingPaddingLeft"
	case SettingPaddingRight:
		settingName = "SettingPaddingRight"
	case SettingPaddingTop:
		settingName = "SettingPaddingTop"
	case SettingPrerenderedVListID:
		settingName = "SettingPrerenderedVListID"
	case SettingPrepend:
		settingName = "SettingPrepend"
	case SettingPreserveWhitespace:
		settingName = "SettingPreserveWhitespace"
	case SettingRowspan:
		settingName = "SettingRowspan"
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
	case SettingDirection:
		settingName = "SettingDirection"
	case SettingLanguage:
		settingName = "SettingLanguage"
	case SettingHyphens:
		settingName = "SettingHyphens"
	case SettingHyphenPenalty:
		settingName = "SettingHyphenPenalty"
	case SettingLinebreakTolerance:
		settingName = "SettingLinebreakTolerance"
	case SettingLinebreakEmergencyStretch:
		settingName = "SettingLinebreakEmergencyStretch"
	default:
		settingName = fmt.Sprintf("%d", st)
	}
	return settingName
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
	te.Settings = make(TypesettingSettings, 32)
	te.Items = make([]any, 0, 4)
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
		switch k {
		case SettingPrepend:
			v = node.StringValue(v.(node.Node))
		case SettingPreserveWhitespace:
			if v == false {
				showSetting = false
			}
		case SettingTabSizeSpaces:
			if v == 4 {
				showSetting = false
			}
		case SettingDebug:
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
		case *node.Image:
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
	Fontfamily       *FontFamily
	Language         *lang.Lang
	Alignment        HorizontalAlignment
	Fontsize         bag.ScaledPoint
	hsize            bag.ScaledPoint
	HyphenPenalty    int
	IndentLeft       bag.ScaledPoint
	IndentLeftRows   int
	Leading          bag.ScaledPoint
	Tolerance        float64
	EmergencyStretch bag.ScaledPoint
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

// HyphenPenalty sets the penalty for hyphenating words. Higher values
// discourage hyphenation, preferring to stretch/shrink spaces instead.
// Default is 50. Values around 200-1000 reduce hyphenation noticeably.
// A value of 10000 effectively disables hyphenation.
func HyphenPenalty(penalty int) TypesettingOption {
	return func(p *Options) {
		p.HyphenPenalty = penalty
	}
}

// Tolerance sets how much the line can deviate from the ideal spacing
// before the algorithm considers it unacceptable. Default is 4.0.
// Higher values allow looser/tighter lines, which may be needed for
// narrow columns or text with few hyphenation opportunities.
// TeX uses 200 for \sloppy and 10000 for \emergencystretch.
func Tolerance(tolerance float64) TypesettingOption {
	return func(p *Options) {
		p.Tolerance = tolerance
	}
}

// EmergencyStretch sets a per-line stretch budget that the line breaker
// may use as a last resort when no natural stretch is available on a
// candidate line. This is TeX's \emergencystretch and helps narrow
// columns containing long words: without it the breaker may go overfull
// rather than produce an underfull line. Default is 0 (disabled).
// Typical values are 1–3em of the body font size.
func EmergencyStretch(es bag.ScaledPoint) TypesettingOption {
	return func(p *Options) {
		p.EmergencyStretch = es
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

// resolveLogicalAlignment maps the alignment value carried on a paragraph
// to a physical HorizontalAlignment, given the paragraph's resolved
// direction. Two distinct defaults serve different aufrufer-worlds:
//
//   - HAlignDefault is the Go zero value and means "no SettingHAlign was
//     ever set" — i.e. a TeX-style API caller. These get full justification
//     regardless of direction.
//   - HAlignStart / HAlignEnd are CSS Text 3 §7 logical keywords. They
//     resolve direction-aware so that "text-align: start" produces the
//     line-start edge of the inline axis (left for LTR, right for RTL).
//     htmlbag funnels here for unstyled HTML so Arabic blocks are right-
//     aligned and Latin blocks are left-aligned without explicit opt-in.
//
// Physical keywords (HAlignLeft / HAlignRight / HAlignCenter / HAlignJustified)
// pass through unchanged — they are author-set and must not flip on direction.
func resolveLogicalAlignment(a HorizontalAlignment, dir Direction) HorizontalAlignment {
	switch a {
	case HAlignStart:
		if dir == DirectionRTL {
			return HAlignRight
		}
		return HAlignLeft
	case HAlignEnd:
		if dir == DirectionRTL {
			return HAlignLeft
		}
		return HAlignRight
	case HAlignDefault:
		return HAlignJustified
	}
	return a
}

// ParagraphInfo contains information about the whole paragraph and each line.
type ParagraphInfo struct {
	Widths []bag.ScaledPoint
	Height bag.ScaledPoint
	Depth  bag.ScaledPoint
}

// stripLeadingTrailingGlue removes collapsible whitespace (Glue and Kern
// nodes) from the start and end of a node list. This implements CSS
// white-space collapsing. Non-breaking spaces (Penalty 10000 + Glue) are
// preserved. StartStop nodes (colors, hyperlinks) are preserved.
func stripLeadingTrailingGlue(head, tail node.Node) (node.Node, node.Node) {
	// Strip leading Glue/Kern, but stop at a Penalty (protects NBSP).
	for head != nil {
		switch head.(type) {
		case *node.Glue, *node.Kern:
			next := head.Next()
			head = node.DeleteFromList(head, head)
			if next != nil {
				next.SetPrev(nil)
			}
			head = next
			continue
		}
		break
	}
	// Strip trailing Glue/Kern. If a Glue is preceded by a Penalty(10000),
	// it is a non-breaking space — stop and keep both.
	for tail != nil && tail != head {
		switch tail.(type) {
		case *node.Glue:
			if p, ok := tail.Prev().(*node.Penalty); ok && p.Penalty >= 10000 {
				// NBSP: Penalty + Glue pair, keep it
				return head, tail
			}
			prev := tail.Prev()
			head = node.DeleteFromList(head, tail)
			tail = prev
			continue
		case *node.Kern:
			prev := tail.Prev()
			head = node.DeleteFromList(head, tail)
			tail = prev
			continue
		}
		break
	}
	// Edge case: head == tail and it's collapsible
	if head != nil {
		switch head.(type) {
		case *node.Glue, *node.Kern:
			return nil, nil
		}
	}
	return head, tail
}

// preventBreakBeforeClosingPunctuation walks the node list and, for every
// glyph that must not start a line (closing brackets, closing quotation
// marks), neutralises every break-opportunity that immediately precedes it
// by inserting a Penalty(10000) before each preceding Glue/Disc until the
// previous glyph (or list start). Without this pass, an inline-padding glue
// from a <code>/<span> right before a ")" is a valid breakpoint and the
// linebreaker happily wraps the bracket onto the next line — visible e.g.
// as "…CSS @bottom-center\n), and a Lua…". Stretch/shrink contributions of
// the glue itself are preserved; only the breakability is removed.
func preventBreakBeforeClosingPunctuation(head node.Node) node.Node {
	for n := head; n != nil; n = n.Next() {
		g, ok := n.(*node.Glyph)
		if !ok || !isLineStartForbidden(g.Components) {
			continue
		}
		p := n.Prev()
	walkBack:
		for p != nil {
			prev := p.Prev()
			switch p.(type) {
			case *node.Glyph:
				break walkBack
			case *node.Glue, *node.Disc:
				if pn, ok := prev.(*node.Penalty); !(ok && pn.Penalty >= 10000) {
					pen := node.NewPenalty()
					pen.Penalty = 10000
					head = node.InsertBefore(head, p, pen)
				}
				p = prev
			default:
				// Kern, StartStop, Lang and similar non-breaking markers —
				// keep walking back through them.
				p = prev
			}
		}
	}
	return head
}

// isLineStartForbidden returns true for characters that should never appear
// at the start of a line: closing brackets and closing quotation marks. The
// list mirrors Unicode "Pe" (Punctuation, Close) plus the closing members of
// the "Pf" (Final Punctuation) category in widespread typographic use.
func isLineStartForbidden(s string) bool {
	r := []rune(s)
	if len(r) != 1 {
		return false
	}
	switch r[0] {
	case ')', ']', '}',
		'›', // ›  single right-pointing angle quotation mark
		'»', // »  right-pointing double angle quotation mark
		'’', // ’  right single quotation mark
		'”', // ”  right double quotation mark
		'」', // 」 right corner bracket
		'』', // 』 right white corner bracket
		'〉', // 〉 right angle bracket
		'》', // 》 right double angle bracket
		'】', // 】 right black lenticular bracket
		'〕': // 〕 right tortoise shell bracket
		return true
	}
	return false
}

// FormatParagraph creates a rectangular text from the data stored in the
// Paragraph. It applies hyphenation to the node list.
func (fe *Document) FormatParagraph(te *Text, hsize bag.ScaledPoint, opts ...TypesettingOption) (*node.VList, *ParagraphInfo, error) {
	bag.Logger.Log(context.Background(), -8, "FormatParagraph")
	if len(te.Items) == 0 {
		g := node.NewGlue()
		g.Attributes = node.H{"origin": "empty list in FormatParagraph"}
		return node.Vpack(g), nil, nil
	}
	p := &Options{
		Language: fe.Doc.DefaultLanguage,
		hsize:    hsize,
	}
	// SettingLanguage on the paragraph wins over the document default. Sub-Text
	// language switches are handled by Mknodes via node.Lang markers.
	if v, ok := te.Settings[SettingLanguage]; ok {
		if ll, ok := v.(*lang.Lang); ok && ll != nil {
			p.Language = ll
		}
	}
	if ha, ok := te.Settings[SettingHAlign]; ok {
		if ha != nil {
			p.Alignment = ha.(HorizontalAlignment)
		} else {
			p.Alignment = HAlignDefault
		}
	}
	// Resolve paragraph direction. An explicit SettingDirection wins; otherwise
	// auto-detect from the paragraph text using UAX#9. The resolved direction
	// is propagated to all child Text nodes via Mknodes.
	if _, ok := te.Settings[SettingDirection]; !ok {
		if d := detectParagraphDirection(te); d == DirectionRTL {
			te.Settings[SettingDirection] = DirectionRTL
		}
	}
	// Resolve logical text-align (start/end) to physical (left/right) once
	// the paragraph direction is known. CSS Text 3 §7: "start" maps to the
	// line-start edge and "end" to the line-end edge of the inline-axis,
	// which are direction-dependent. CSS-default "text-align: start" thus
	// produces left-aligned LTR paragraphs and right-aligned RTL paragraphs
	// without any extra opt-in from the caller.
	paraDir := DirectionLTR
	if dir, ok := te.Settings[SettingDirection]; ok {
		if d, ok := dir.(Direction); ok {
			paraDir = d
		}
	}
	p.Alignment = resolveLogicalAlignment(p.Alignment, paraDir)
	// Apply indent/margin settings from Text settings
	if il, ok := te.Settings[SettingIndentLeft]; ok {
		p.IndentLeft = il.(bag.ScaledPoint)
	}
	if ilr, ok := te.Settings[SettingIndentLeftRows]; ok {
		p.IndentLeftRows = ilr.(int)
	}
	// Use padding-left as indent for all rows (HTML list behavior).
	// Consume and delete from te.Settings so Mknodes does not also apply
	// it as an inline padding-left glue at the start of the paragraph —
	// padding-left at the block level becomes IndentLeft, the inline-glue
	// path is reserved for child Texts (e.g. <code>, <span>).
	if pl, ok := te.Settings[SettingPaddingLeft]; ok {
		if p.IndentLeft == 0 {
			p.IndentLeft = pl.(bag.ScaledPoint)
			p.IndentLeftRows = 0 // 0 means all rows
		}
		delete(te.Settings, SettingPaddingLeft)
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
	// CSS list-style-position: outside. A marker hbox carrying
	// Attributes["outside-marker"]=true (htmlbag's <li> renderer)
	// must be painted at the line's content-origin minus
	// ListPaddingLeft regardless of the line's text-align stretch.
	// The X offset to use as the marker's anchor inside the line is
	// p.IndentLeft (the LeftSkip portion that's not the centering
	// fil-stretch). Stamp it here so the backend can recover it
	// post-HpackTo, when the natural-vs-actual Glue split is gone.
	if prep, ok := te.Settings[SettingPrepend]; ok {
		if hbox, ok := prep.(*node.HList); ok && hbox.Attributes != nil {
			if outside, _ := hbox.Attributes["outside-marker"].(bool); outside {
				hbox.Attributes["outside-marker-anchor"] = p.IndentLeft
			}
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

	// Strip leading and trailing whitespace (Glue/Kern) for CSS-conformant
	// behavior: spaces at the start/end of a paragraph should not appear.
	hlist, tail = stripLeadingTrailingGlue(hlist, tail)
	if hlist == nil {
		return node.NewVList(), nil, nil
	}

	Hyphenate(hlist, p.Language)
	hlist = preventBreakBeforeClosingPunctuation(hlist)
	node.AppendLineEndAfter(hlist, tail)

	ls := node.NewLinebreakSettings()
	ls.HSize = p.hsize
	ls.Indent = p.IndentLeft
	ls.IndentRows = p.IndentLeftRows
	ls.Tolerance = 4
	if p.Tolerance != 0 {
		ls.Tolerance = p.Tolerance
	}
	if p.EmergencyStretch != 0 {
		ls.EmergencyStretch = p.EmergencyStretch
	}
	if p.HyphenPenalty != 0 {
		ls.Hyphenpenalty = p.HyphenPenalty
	}
	// Settings-driven overrides (e.g. CSS -bag-linebreak-tolerance and
	// -bag-linebreak-hyphen-penalty routed through htmlbag). The settings
	// path takes precedence over the option-based defaults above so that
	// per-element CSS can tune the line breaker's behaviour.
	if v, ok := te.Settings[SettingLinebreakTolerance]; ok {
		if t, ok := v.(float64); ok && t > 0 {
			ls.Tolerance = t
		}
	}
	if v, ok := te.Settings[SettingLinebreakEmergencyStretch]; ok {
		if es, ok := v.(bag.ScaledPoint); ok && es > 0 {
			ls.EmergencyStretch = es
		}
	}
	if v, ok := te.Settings[SettingHyphenPenalty]; ok {
		if hp, ok := v.(int); ok && hp != 0 {
			ls.Hyphenpenalty = hp
		}
	}
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
		lg.Attributes = node.H{"origin": "glue line end"}
		lg.Stretch = bag.Factor
		lg.StretchOrder = node.StretchFill
		lg.Subtype = node.GlueLineEnd
		ls.LineEndGlue = lg
	}
	if p.Alignment == HAlignRight || p.Alignment == HAlignCenter {
		lg := node.NewGlue()
		lg.Attributes = node.H{"origin": "glue line start"}
		lg.Stretch = bag.Factor
		lg.StretchOrder = node.StretchFill
		lg.Subtype = node.GlueLineStart
		ls.LineStartGlue = lg
	}
	vlist, info := node.Linebreak(hlist, ls)
	for _, inf := range info {
		pi.Widths = append(pi.Widths, inf.Width)
	}
	// UAX#9 L1 (trailing-whitespace reset) and L2-L4 (visual reorder) per
	// line. Pure-LTR paragraphs are handled by the helper as a fast no-op.
	var paragraphLevel uint8
	if dir, ok := te.Settings[SettingDirection]; ok {
		if d, ok := dir.(Direction); ok && d == DirectionRTL {
			paragraphLevel = 1
		}
	}
	bidiReorderVList(vlist, paragraphLevel)

	for _, cb := range fe.postLinebreakCallback {
		vlist = cb(vlist)
	}
	if htt, ok := te.Settings[SettingHeight]; ok {
		if ht, ok := htt.(bag.ScaledPoint); ok {
			moreHeight := ht - vlist.Height - vlist.Depth
			topGlue := node.NewGlue()
			bottomGlue := node.NewGlue()
			valign := VAlignMiddle
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

// collectParagraphText recursively concatenates the string content of a Text
// element into b for bidi auto-detection. Non-string items (images, tables,
// VLists) contribute nothing — they are direction-neutral.
func collectParagraphText(te *Text, b *bytes.Buffer) {
	for _, itm := range te.Items {
		switch t := itm.(type) {
		case string:
			b.WriteString(t)
		case *Text:
			collectParagraphText(t, b)
		}
	}
}

// propagateBidiLevels fills in BidiLevel for nodes that don't naturally
// carry one. Hyphenate inserts Disc nodes after Mknodes has run, so the
// shape-time level tagging never reaches them and they default to 0.
// A Disc sitting inside a level-2 LTR-in-RTL word would then split the
// level-2 run during the L2-L4 reorder, scattering the word visually.
// We adopt the level of the most recent preceding content node, which
// matches the semantic intent: a hyphenation point belongs to the word
// it breaks.
func propagateBidiLevels(line *node.HList) {
	if line == nil || line.List == nil {
		return
	}
	var prevLevel uint8
	for n := line.List; n != nil; n = n.Next() {
		switch n.(type) {
		case *node.Disc:
			if n.BidiLevel() == 0 {
				n.SetBidiLevel(prevLevel)
			}
		case *node.Glyph, *node.Glue, *node.Kern:
			prevLevel = n.BidiLevel()
		}
	}
}

// applyL1 implements UAX#9 rule L1: trailing whitespace at the end of a
// line is reset to the paragraph embedding level. Without this, a line
// that ends in a Glue belonging to an embedded run (e.g. an LTR-run-final
// space inside an RTL paragraph) would carry the run's elevated level
// into the L2-L4 reorder and end up mis-positioned, typically as a double
// space at one end of the visual line.
//
// We collect the line nodes into a slice, walk them backward, skip the
// linebreak's own end-of-line artefacts (Penalty, zero-width Glue/Kern),
// and reset the BidiLevel of the contiguous trailing whitespace to
// paragraphLevel. Walking stops at the first non-whitespace, non-Penalty
// node — that is the last *content* glyph and everything beyond it is
// real text whose level must be preserved.
func applyL1(line *node.HList, paragraphLevel uint8) {
	if line == nil || line.List == nil {
		return
	}
	var nodes []node.Node
	for n := line.List; n != nil; n = n.Next() {
		nodes = append(nodes, n)
	}
	for i := len(nodes) - 1; i >= 0; i-- {
		switch nodes[i].(type) {
		case *node.Glue, *node.Kern:
			nodes[i].SetBidiLevel(paragraphLevel)
		case *node.Penalty:
			// Linebreak inserts penalties around its end-of-line glue;
			// they carry no visible width and shouldn't gate the reset.
			continue
		default:
			// Glyph, Disc, Image, etc. — content boundary reached.
			return
		}
	}
}

// bidiReorderVList walks every HList line in vl and applies UAX#9 L1 and
// L2-L4 per line. paragraphLevel is the embedding level of the paragraph
// base direction (0 for LTR, 1 for RTL); it is the value to which trailing
// whitespace gets reset by L1.
func bidiReorderVList(vl *node.VList, paragraphLevel uint8) {
	if vl == nil {
		return
	}
	for n := vl.List; n != nil; n = n.Next() {
		if hl, ok := n.(*node.HList); ok {
			bidiReorderLine(hl, paragraphLevel)
		}
	}
}

// bidiReorderLine reorders the contents of a single line per UAX#9 L1
// (trailing-whitespace level reset) followed by L2-L4 (visual reorder).
// Operates at the *glyph* level: every node carries its own embedding
// level (assigned at shape time) and the algorithm reverses maximal
// contiguous sequences of nodes at level >= current, walking from the
// highest level down to 1. Nodes arrive in logical order (RTL run shaper
// output is reversed beforehand in shapeWithBidi) and leave in visual
// order. paragraphLevel is the paragraph base embedding level used by L1.
func bidiReorderLine(line *node.HList, paragraphLevel uint8) {
	if line == nil || line.List == nil {
		return
	}
	propagateBidiLevels(line)
	applyL1(line, paragraphLevel)
	var nodes []node.Node
	var levels []uint8
	var maxLevel uint8
	for n := line.List; n != nil; n = n.Next() {
		nodes = append(nodes, n)
		l := n.BidiLevel()
		levels = append(levels, l)
		if l > maxLevel {
			maxLevel = l
		}
	}
	if maxLevel == 0 || len(nodes) <= 1 {
		return
	}
	// L2-L4: reverse maximal sequences at level >= current, from the
	// highest level down through 1. Reversing the level array alongside
	// the node array keeps subsequent passes consistent.
	for level := maxLevel; level >= 1; level-- {
		i := 0
		for i < len(nodes) {
			if levels[i] >= level {
				j := i
				for j < len(nodes) && levels[j] >= level {
					j++
				}
				for a, b := i, j-1; a < b; a, b = a+1, b-1 {
					nodes[a], nodes[b] = nodes[b], nodes[a]
					levels[a], levels[b] = levels[b], levels[a]
				}
				i = j
			} else {
				i++
			}
		}
	}
	// Rewire the linked list in the new visual order.
	var first, last node.Node
	for _, n := range nodes {
		n.SetPrev(nil)
		n.SetNext(nil)
		if first == nil {
			first = n
			last = n
		} else {
			last.SetNext(n)
			n.SetPrev(last)
			last = n
		}
	}
	line.List = first
}

// shapeWithBidi runs UAX#9 over str using paragraphDir as the embedding
// base, splits the input into directional runs, shapes each run with its
// own direction, and returns the concatenated atoms in logical run order
// alongside a parallel slice of bidi levels (0 = LTR, 1 = RTL). The post-
// linebreak reorder consults these levels to flip RTL runs into visual
// position on each line.
func shapeWithBidi(fnt *font.Font, str string, features []ot.Feature, variations map[string]float64, paragraphDir Direction) ([]font.Atom, []uint8) {
	if str == "" {
		return nil, nil
	}
	// Soft-hyphen handling. The shaper is configured to drop default-ignorables
	// from the buffer (font.ShapeDir sets BufferFlagRemoveDefaultIgnorables), so
	// U+00AD never reaches the atom loop on its own. We split the input on
	// U+00AD here, shape each segment in its own bidi pass, and emit a sentinel
	// atom between segments (Components == "­", IsSpace=false). The atom
	// loop in BuildNodelistFromString recognises this sentinel and turns it
	// into a discretionary break (or drops it for `hyphens: none`).
	if strings.ContainsRune(str, '­') {
		segments := strings.Split(str, "­")
		var atoms []font.Atom
		var levels []uint8
		for i, seg := range segments {
			if i > 0 {
				lvl := uint8(0)
				if paragraphDir == DirectionRTL {
					lvl = 1
				}
				atoms = append(atoms, font.Atom{Components: "­"})
				levels = append(levels, lvl)
			}
			segAtoms, segLevels := shapeWithBidi(fnt, seg, features, variations, paragraphDir)
			atoms = append(atoms, segAtoms...)
			levels = append(levels, segLevels...)
		}
		return atoms, levels
	}
	defaultDir := bidi.LeftToRight
	if paragraphDir == DirectionRTL {
		defaultDir = bidi.RightToLeft
	}
	p := bidi.Paragraph{}
	if _, err := p.SetString(str, bidi.DefaultDirection(defaultDir)); err != nil {
		return shapeFallback(fnt, str, features, variations, paragraphDir)
	}
	o, err := p.Order()
	if err != nil || o.NumRuns() == 0 {
		return shapeFallback(fnt, str, features, variations, paragraphDir)
	}
	// Order() returns runs in visual order; we need logical order to build
	// the node list (linebreak walks logical order). Sort run indices by
	// their startpos.
	type runRef struct {
		idx, start int
	}
	refs := make([]runRef, o.NumRuns())
	for i := 0; i < o.NumRuns(); i++ {
		r := o.Run(i)
		s, _ := r.Pos()
		refs[i] = runRef{idx: i, start: s}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].start < refs[j].start })

	// For plain text without explicit embedding controls, the embedding
	// level of a run is one of three values:
	//   - paragraph base (0 in LTR, 1 in RTL) — used when the run direction
	//     matches the paragraph base
	//   - paragraph base + 1 — used for the opposite direction (LTR run in
	//     RTL paragraph → 2; RTL run in LTR paragraph → 1)
	// Without these stair-step levels the post-linebreak reorder cannot tell
	// LTR-in-RTL from LTR-in-LTR and a paragraph like
	// "Hebrew[RTL] english[LTR] Hebrew[RTL]" comes out with the spaces
	// trapped at the line edges instead of between the words.
	var baseLevel uint8
	if paragraphDir == DirectionRTL {
		baseLevel = 1
	}
	var atoms []font.Atom
	var levels []uint8
	for _, ref := range refs {
		run := o.Run(ref.idx)
		runDir := ot.DirectionLTR
		var lvl uint8
		if run.Direction() == bidi.RightToLeft {
			runDir = ot.DirectionRTL
			lvl = 1 // RTL is always odd; lowest odd level is 1
		} else if baseLevel == 1 {
			lvl = 2 // LTR within RTL paragraph → next-higher even level
		}
		// else: LTR within LTR paragraph stays at level 0 (zero value)
		runAtoms := fnt.ShapeDir(run.String(), features, variations, runDir)
		// HarfBuzz returns RTL output in reverse-logical (visual) order.
		// We store atoms in *logical* order so paragraph-edge whitespace
		// stripping and Knuth–Plass linebreaking see word boundaries at the
		// natural positions; the post-linebreak reorder rebuilds visual
		// order at line time.
		if runDir == ot.DirectionRTL {
			reverseAtoms(runAtoms)
		}
		atoms = append(atoms, runAtoms...)
		for range runAtoms {
			levels = append(levels, lvl)
		}
	}
	return atoms, levels
}

// reverseAtoms flips an atom slice in place and shifts Kernafter values so
// that the kerning between two glyphs stays attached to the same logical
// pair after a subsequent visual-order pass. HarfBuzz stores Kernafter on
// atom[i] meaning "kern between buffer position i and i+1". After reversing
// the array, the kern between visual pair (i, i+1) ends up between new
// atoms (N-1-i, N-2-i), so we need to shift Kernafter one position left.
func reverseAtoms(atoms []font.Atom) {
	n := len(atoms)
	if n < 2 {
		return
	}
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		atoms[i], atoms[j] = atoms[j], atoms[i]
	}
	for i := 0; i < n-1; i++ {
		atoms[i].Kernafter = atoms[i+1].Kernafter
	}
	atoms[n-1].Kernafter = 0
}

// shapeFallback shapes str as a single run when bidi analysis fails or the
// input has no recognisable directional content. Levels are derived from
// paragraphDir so the rest of the pipeline still has consistent metadata.
func shapeFallback(fnt *font.Font, str string, features []ot.Feature, variations map[string]float64, paragraphDir Direction) ([]font.Atom, []uint8) {
	dir := ot.DirectionLTR
	var lvl uint8
	if paragraphDir == DirectionRTL {
		dir = ot.DirectionRTL
		lvl = 1
	}
	atoms := fnt.ShapeDir(str, features, variations, dir)
	levels := make([]uint8, len(atoms))
	for i := range levels {
		levels[i] = lvl
	}
	return atoms, levels
}

// detectParagraphDirection runs UAX#9 over the concatenated text of te and
// returns DirectionRTL if the paragraph reads right-to-left, DirectionLTR
// otherwise. Mixed paragraphs fall back to LTR in this initial bidi stage —
// proper run-level handling will be added later.
func detectParagraphDirection(te *Text) Direction {
	var b bytes.Buffer
	collectParagraphText(te, &b)
	if b.Len() == 0 {
		return DirectionLTR
	}
	p := bidi.Paragraph{}
	if _, err := p.SetString(b.String()); err != nil {
		return DirectionLTR
	}
	// Order() must run before Direction() — the latter reads o.directions[0]
	// which is unset until Order() populates it.
	o, err := p.Order()
	if err != nil || o.NumRuns() == 0 {
		return DirectionLTR
	}
	if o.Direction() == bidi.RightToLeft {
		return DirectionRTL
	}
	return DirectionLTR
}

func parseOpenTypeFeatures(featurelist any) []ot.Feature {
	fontfeatures := []ot.Feature{}
	switch t := featurelist.(type) {
	case string:
		for str := range strings.SplitSeq(t, ",") {
			if f, ok := ot.FeatureFromString(str); ok {
				fontfeatures = append(fontfeatures, f)
			}
		}
	case []string:
		for _, single := range t {
			for str := range strings.SplitSeq(single, ",") {
				if f, ok := ot.FeatureFromString(str); ok {
					fontfeatures = append(fontfeatures, f)
				}
			}
		}
	}
	return fontfeatures
}

// BuildNodelistFromString returns a node list containing glyphs from the string
// with the settings in ts.
func (fe *Document) BuildNodelistFromString(ts TypesettingSettings, str string) (node.Node, error) {
	bag.Logger.Log(context.Background(), -8, "Document#BuildNodelistFromString")
	fontweight := FontWeight400
	fontstyle := FontStyleNormal
	var fontfamily *FontFamily
	fontsize := 12 * bag.Factor
	var col *color.Color
	var hyperlink document.Hyperlink
	var hasHyperlink bool
	var hasUnderline bool
	fontfeatures := make([]ot.Feature, 0, len(fe.DefaultFeatures))
	for _, f := range fe.DefaultFeatures {
		fontfeatures = append(fontfeatures, f)
	}
	preserveWhitespace := false
	letterSpacing := bag.ScaledPoint(0)
	yoffset := bag.ScaledPoint(0)
	direction := DirectionLTR
	hyphensMode := "" // CSS hyphens: "" (auto), "auto", "manual", "none"
	var settingFontFeatures []ot.Feature
	for k, v := range ts {
		switch k {
		case SettingFontWeight:
			switch t := v.(type) {
			case int:
				fontweight = FontWeight(t)
			case FontWeight:
				fontweight = t
			default:
				bag.Logger.Error("Unknown type for SettingFontWeight", "type", fmt.Sprintf("%T", t))
			}
		case SettingFontFamily:
			fontfamily = v.(*FontFamily)
		case SettingSize:
			switch t := v.(type) {
			case int64:
				// assume it is sp
				fontsize = bag.ScaledPoint(t)
			case bag.ScaledPoint:
				fontsize = t
			default:
				bag.Logger.Error("Unknown type for SettingSize", "type", fmt.Sprintf("%T", t))
			}
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
		case SettingDest:
			// handled after node list is built
		case SettingTextDecorationLine:
			if underlineType, ok := v.(TextDecorationLine); ok && underlineType == TextDecorationUnderline {
				hasUnderline = true
			}
		case SettingFontExpansion:
			// ignore
		case SettingStyle:
			fontstyle = v.(FontStyle)
		case SettingOpenTypeFeature:
			settingFontFeatures = parseOpenTypeFeatures(v)
		case SettingFontVariationSettings:
			// handled below when shaping
		case SettingMarginTop, SettingMarginRight, SettingMarginBottom, SettingMarginLeft, SettingPaddingRight, SettingPaddingBottom, SettingPaddingTop, SettingPaddingLeft:
			// ignore
		case SettingLetterSpacing:
			letterSpacing = v.(bag.ScaledPoint)
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
		case SettingWidth, SettingBox, SettingPageBreakAfter, SettingPageBreakBefore:
			// ignore
		case SettingPreserveWhitespace:
			preserveWhitespace = v.(bool)
		case SettingYOffset:
			yoffset = v.(bag.ScaledPoint)
		case SettingDirection:
			if d, ok := v.(Direction); ok {
				direction = d
			}
		case SettingLanguage:
			// consumed at the paragraph level (FormatParagraph) and at
			// sub-Text boundaries (Mknodes); the glyph builder ignores it.
		case SettingHyphens:
			if s, ok := v.(string); ok {
				hyphensMode = s
			}
		case SettingHyphenPenalty, SettingLinebreakTolerance, SettingLinebreakEmergencyStretch:
			// consumed at the paragraph level (FormatParagraph); the glyph
			// builder ignores them.
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
	bag.Logger.Log(context.Background(), -8, "GetFontSource", "fs", fs.Name)
	// fs.SizeAdjust is CSS size-adjust normalized so that 0 = 100% and negative = shrinking.
	if fs.SizeAdjust != 0 {
		fontsize = bag.ScaledPointFromFloat(fontsize.ToPT() * (1 - fs.SizeAdjust))
	}
	// First the font source default features should get applied, then the
	// features from the current settings.
	fontfeatures = append(fontfeatures, parseOpenTypeFeatures(fs.FontFeatures)...)
	fontfeatures = append(fontfeatures, settingFontFeatures...)

	// Collect variation settings: start with FontSource defaults, override with settings
	var variations map[string]float64
	if fs.VariationSettings != nil {
		variations = make(map[string]float64, len(fs.VariationSettings))
		maps.Copy(variations, fs.VariationSettings)
	}
	if settingsVars, ok := ts[SettingFontVariationSettings]; ok {
		if varMap, ok := settingsVars.(map[string]float64); ok {
			if variations == nil {
				variations = make(map[string]float64, len(varMap))
			}
			maps.Copy(variations, varMap)
		}
	}
	// Use LoadFaceWithVariations to get a face with the correct variation settings
	// This ensures each unique variation combination gets its own face for PDF subsetting
	if face, err = fe.LoadFaceWithVariations(fs, variations); err != nil {
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
		fnt = font.NewFont(face, fontsize)
		fnt.MissingGlyphFunc = fe.MissingGlyphFunc
		fe.usedFonts[face][fontsize] = fnt
	}

	var head, cur node.Node
	// Insert a destination anchor if SettingDest is set.
	if destName, ok := ts[SettingDest]; ok {
		destStart := node.NewStartStop()
		destStart.Action = node.ActionDest
		destStart.Value = destName
		head = destStart
	}
	var hyperlinkStart, hyperlinkStop *node.StartStop
	if hasHyperlink {
		hyperlinkStart = node.NewStartStop()
		hyperlinkStart.Action = node.ActionHyperlink
		hyperlinkStart.Value = &hyperlink
		if head != nil {
			node.InsertAfter(head, head, hyperlinkStart)
		} else {
			head = hyperlinkStart
		}
	}
	var underlineStart *node.StartStop
	if hasUnderline {
		underlineStart = node.NewStartStop()
		underlineStart.SetAttribute("underline", true)
		underlineStart.SetAttribute("underlinepos", -fontsize/6)
		underlineStart.SetAttribute("underlinelw", fontsize/20)
		underlineStart.SetAttribute("SettingTextDecorationLine", TextDecorationUnderline)
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
	atoms, atomLevels := shapeWithBidi(fnt, str, fontfeatures, variations, direction)
	for i, r := range atoms {
		level := atomLevels[i]
		// Capture cur before processing so we can tag all newly-inserted
		// nodes with this atom's bidi level after the branches run.
		prevCur := cur
		if r.IsSpace {
			if preserveWhitespace {
				switch r.Components {
				case " ", "\u00A0":
					g := node.NewRule()
					// Real glyph advance, not fnt.Space (TeX inter-word
					// glue default of size*333/1000), so monospace ASCII
					// art and code blocks line up column-for-column.
					g.Width = fnt.SpaceChar.Advance
					head = node.InsertAfter(head, cur, g)
					cur = g
					lastglue = g
				case "\t":
					// tab size...
					g := node.NewGlue()
					g.Attributes = node.H{"origin": "tab"}
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
					// Forced line break. Each "\n" emits one HardBreak;
					// consecutive newlines therefore produce consecutive
					// forced breaks, which the line breaker renders as
					// empty lines between them — that is how blank lines
					// in plain-text source come through.
					br := node.NewHardBreak()
					head = node.InsertAfter(head, cur, br)
					cur = br
					lastglue = br
				default:
					panic("unhandled whitespace type")
				}
			} else {
				if r.Components == "\n" {
					// Forced line break. The line breaker handles
					// alignment via per-paragraph LineStartGlue /
					// LineEndGlue, so no in-line stretch glue is
					// emitted here — that lets the alignment
					// configuration "own" line stretching unambiguously
					// (no more in-line fill glue competing with
					// per-line glue, and no more alignment lookup in
					// the atom loop). Each "\n" emits one HardBreak;
					// consecutive newlines therefore produce empty
					// HBoxes between them — that is how a blank line
					// in plain-text source comes through.
					br := node.NewHardBreak()
					head = node.InsertAfter(head, cur, br)
					cur = br
					// lastglue stays untouched: the type-switch in the
					// default-space block below recognises HardBreak
					// via cur and skips the leading space insert.
				}

				if lastglue == nil {
					switch cur.(type) {
					case *node.HardBreak:
						// A HardBreak already terminates the line and
						// consumes adjacent whitespace — do not insert
						// the default-space glue.
					default:
						if r.NoBreak {
							// NBSP: insert Penalty(10000) to prevent line break
							p := node.NewPenalty()
							p.Penalty = 10000
							head = node.InsertAfter(head, cur, p)
							cur = p
						}
						g := node.NewGlue()
						g.Attributes = node.H{"origin": "lastglue=nil"}
						g.Width = fnt.Space
						g.Stretch = fnt.SpaceStretch
						g.Shrink = fnt.SpaceShrink
						head = node.InsertAfter(head, cur, g)
						cur = g
						lastglue = g
					}
				}
			}
		} else if r.Components == "­" {
			// Soft hyphen (U+00AD). CSS Text 3 §6:
			//   * `hyphens: none`   → never break here, drop the codepoint
			//   * `hyphens: manual` → only break here (no pattern breaks)
			//   * `hyphens: auto`   → break here AND at language patterns
			// In all break-allowed modes we emit a Disc node so the line
			// breaker treats the position as a discretionary breakpoint
			// with the font's hyphenchar as the visible material.
			if hyphensMode != "none" {
				disc := node.NewDisc()
				hyphen := node.NewGlyph()
				hyphen.Font = fnt
				hyphen.Width = fnt.Hyphenchar.Advance
				hyphen.Components = fnt.Hyphenchar.Components
				hyphen.Codepoint = fnt.Hyphenchar.Codepoint
				disc.Pre = hyphen
				head = node.InsertAfter(head, cur, disc)
				cur = disc
				lastglue = nil
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
			// Apply GPOS positioning offsets for mark attachment
			n.XOffset = r.XOffset
			n.YOffset = yoffset + r.YOffset
			head = node.InsertAfter(head, cur, n)
			cur = n
			lastglue = nil

			if r.Kernafter != 0 {
				k := node.NewKern()
				k.Kern = r.Kernafter
				head = node.InsertAfter(head, cur, k)
				cur = k
			}
			if letterSpacing != 0 {
				k := node.NewKern()
				k.Kern = letterSpacing
				head = node.InsertAfter(head, cur, k)
				cur = k
			}
		}
		// Tag every node added in this iteration with the bidi level. Walk
		// from the first newly-inserted node (prevCur.Next, or head if the
		// list was empty) up to and including cur.
		var startNode node.Node
		if prevCur != nil {
			startNode = prevCur.Next()
		} else {
			startNode = head
		}
		for n := startNode; n != nil; n = n.Next() {
			n.SetBidiLevel(level)
			if n == cur {
				break
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
		underlineStop.SetAttribute("underline", false)
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
	bag.Logger.Log(context.Background(), -8, "Document#Mknodes")
	if len(ts.Items) == 0 {
		return nil, nil, nil
	}
	newSettings := make(TypesettingSettings)
	var nl, end node.Node
	maps.Copy(newSettings, ts.Settings)
	var hyperlinkStartNode *node.StartStop
	var hyperlinkDest string

	// Insert a destination anchor if SettingDest is set on this text block.
	if destName, ok := ts.Settings[SettingDest]; ok {
		destStart := node.NewStartStop()
		destStart.Action = node.ActionDest
		destStart.Value = destName
		head = node.InsertAfter(head, tail, destStart)
		tail = destStart
		delete(newSettings, SettingDest)
	}

	// Prepend custom nodes (e.g., list markers) before processing items.
	if prep, ok := ts.Settings[SettingPrepend]; ok {
		if pn, ok := prep.(node.Node); ok {
			cp := node.CopyList(pn)
			head = node.InsertAfter(head, tail, cp)
			tail = node.Tail(cp)
		}
	}

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
				// Inline padding is rigid space without break opportunity —
				// CSS does not allow a line break inside the padding of an
				// inline box. A Kern fits both constraints (fixed advance,
				// not a Knuth-Plass breakpoint); a Glue would be a break
				// candidate and the linebreaker would happily wrap a
				// trailing ")" onto the next line across the </code> font
				// boundary (see preventBreakBeforeClosingPunctuation for
				// the defense-in-depth pass that also catches stray
				// Glue-based break opportunities from other sources).
				if pl, ok := ts.Settings[SettingPaddingLeft]; ok {
					paddingLeft := pl.(bag.ScaledPoint)
					if paddingLeft > 0 {
						k := node.NewKern()
						k.Kern = paddingLeft
						k.Attributes = node.H{"origin": "padding left"}
						nl = node.InsertBefore(nl, nl, k)
					}
				}
				if pr, ok := ts.Settings[SettingPaddingRight]; ok {
					paddingRight := pr.(bag.ScaledPoint)
					if paddingRight > 0 {
						k := node.NewKern()
						k.Kern = paddingRight
						k.Attributes = node.H{"origin": "padding right"}
						node.InsertAfter(nl, node.Tail(nl), k)
					}
				}
				head = node.InsertAfter(head, tail, nl)
				tail = node.Tail(nl)
			}
		case *Text:
			// Leader: create pattern HList + leader Glue instead of recursing.
			if leaderStr, ok := t.Settings[SettingLeader]; ok {
				for k, v := range newSettings {
					if _, found := t.Settings[k]; !found {
						t.Settings[k] = v
					}
				}
				delete(t.Settings, SettingLeader)
				nl, err = fe.BuildNodelistFromString(t.Settings, leaderStr.(string))
				if err != nil {
					return nil, nil, err
				}
				if nl != nil {
					pattern := node.Hpack(nl)
					g := node.NewGlue()
					g.Stretch = bag.Factor
					g.StretchOrder = node.StretchFilll
					g.Leader = pattern
					g.LeaderType = node.LeaderAligned
					head = node.InsertAfter(head, tail, g)
					tail = g
				}
				continue
			}
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
			// we don't want to inherit hyperlinks, prepend (list markers), or destinations
			delete(t.Settings, SettingHyperlink)
			delete(t.Settings, SettingPrepend)
			delete(t.Settings, SettingDest)

			// Per-run language switch. If the child Text carries an explicit
			// SettingLanguage that differs from the surrounding context, emit
			// node.Lang markers around its node list so the Hyphenate walker
			// swaps patterns for the run and restores the parent's patterns
			// afterwards. Settings inheritance has already happened above, so
			// child-without-explicit-lang inherits the parent's value and the
			// pointers compare equal here — no spurious switch is emitted.
			parentLang, _ := newSettings[SettingLanguage].(*lang.Lang)
			childLang, _ := t.Settings[SettingLanguage].(*lang.Lang)
			needsLangSwitch := childLang != nil && childLang != parentLang

			if needsLangSwitch {
				opener := node.NewLang()
				opener.Lang = childLang
				head = node.InsertAfter(head, tail, opener)
				tail = opener
			}

			nl, end, err = fe.Mknodes(t)
			if err != nil {
				return nil, nil, err
			}
			if nl != nil {
				head = node.InsertAfter(head, tail, nl)
				tail = end
			}

			if needsLangSwitch {
				// Restore the surrounding language. If the parent had no
				// explicit SettingLanguage, the document default is what
				// FormatParagraph started with — that's the language the
				// run after this child should be hyphenated under.
				closerLang := parentLang
				if closerLang == nil {
					closerLang = fe.Doc.DefaultLanguage
				}
				if closerLang != nil {
					closer := node.NewLang()
					closer.Lang = closerLang
					head = node.InsertAfter(head, tail, closer)
					tail = closer
				}
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
