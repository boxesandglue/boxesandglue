package htmlstyle

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/frontend"
	"golang.org/x/net/html"
)

var (
	onlyUnitRE = regexp.MustCompile(`^(sp|mm|cm|in|pt|px|pc|m)$`)
	unitRE     = regexp.MustCompile(`(.*?)(sp|mm|cm|in|pt|px|pc|m)`)
	astRE      = regexp.MustCompile(`(\d*)\*`)
)

func processTr(item *HTMLItem, ss StylesStack, df *frontend.Document) (*frontend.TableRow, error) {
	defaultFontsize := ss.CurrentStyle().DefaultFontSize
	curFontsize := ss.CurrentStyle().Fontsize
	tr := &frontend.TableRow{}
	for _, itm := range item.Children {
		if itm.Data == "td" || itm.Data == "th" {
			styles := ss.PushStyles()
			tc := &frontend.TableCell{}
			borderLeftStyle := ""
			borderRightStyle := ""
			borderTopStyle := ""
			borderBottomStyle := ""
			for k, v := range itm.Styles {
				switch k {
				case "padding-top":
					tc.PaddingTop = ParseRelativeSize(v, curFontsize, defaultFontsize)
				case "padding-bottom":
					tc.PaddingBottom = ParseRelativeSize(v, curFontsize, defaultFontsize)
				case "padding-left":
					tc.PaddingLeft = ParseRelativeSize(v, curFontsize, defaultFontsize)
				case "padding-right":
					tc.PaddingRight = ParseRelativeSize(v, curFontsize, defaultFontsize)
				case "border-top-width":
					tc.BorderTopWidth = ParseRelativeSize(v, curFontsize, defaultFontsize)
				case "border-bottom-width":
					tc.BorderBottomWidth = ParseRelativeSize(v, curFontsize, defaultFontsize)
				case "border-left-width":
					tc.BorderLeftWidth = ParseRelativeSize(v, curFontsize, defaultFontsize)
				case "border-right-width":
					tc.BorderRightWidth = ParseRelativeSize(v, curFontsize, defaultFontsize)
				case "border-top-color":
					tc.BorderTopColor = df.GetColor(v)
				case "border-bottom-color":
					tc.BorderBottomColor = df.GetColor(v)
				case "border-left-color":
					tc.BorderLeftColor = df.GetColor(v)
				case "border-right-color":
					tc.BorderRightColor = df.GetColor(v)
				case "border-top-style":
					borderTopStyle = v
				case "border-bottom-style":
					borderBottomStyle = v
				case "border-left-style":
					borderLeftStyle = v
				case "border-right-style":
					borderRightStyle = v
				case "vertical-align":
					styles.Valign = ParseVerticalAlign(v, styles)
				case "text-align":
					styles.Halign = ParseHorizontalAlign(v, styles)
				default:
					// fmt.Println(v)
				}
			}
			if borderTopStyle == "none" {
				tc.BorderTopWidth = 0
			}
			if borderBottomStyle == "none" {
				tc.BorderBottomWidth = 0
			}
			if borderLeftStyle == "none" {
				tc.BorderLeftWidth = 0
			}
			if borderRightStyle == "none" {
				tc.BorderRightWidth = 0
			}

			for k, v := range itm.Attributes {
				switch k {
				case "rowspan":
					rs, err := strconv.Atoi(v)
					if err != nil {
						return nil, err
					}
					tc.ExtraRowspan = rs - 1
				case "colspan":
					cs, err := strconv.Atoi(v)
					if err != nil {
						return nil, err
					}
					tc.ExtraColspan = cs - 1
				}
			}
			tc.VAlign = styles.Valign
			tc.HAlign = styles.Halign

			text, err := Output(itm, ss, df)
			if err != nil {
				return nil, err
			}
			tc.Contents = append(tc.Contents, text)
			tr.Cells = append(tr.Cells, tc)
			ss.PopStyles()
		}
	}
	return tr, nil
}

func processTbody(item *HTMLItem, ss StylesStack, df *frontend.Document) (frontend.TableRows, error) {
	var trows frontend.TableRows
	for _, itm := range item.Children {
		if itm.Data == "tr" {
			styles := ss.PushStyles()
			for k, v := range itm.Styles {
				switch k {
				case "vertical-align":
					styles.Valign = ParseVerticalAlign(v, styles)
				case "text-align":
					styles.Halign = ParseHorizontalAlign(v, styles)
				}
			}

			tr, err := processTr(itm, ss, df)
			if err != nil {
				return nil, err
			}
			ss.PopStyles()
			trows = append(trows, tr)
		}
	}
	return trows, nil
}

func processTable(item *HTMLItem, ss StylesStack, df *frontend.Document) (*frontend.Table, error) {
	tbl := &frontend.Table{}
	tbl.Stretch = false
	var rows frontend.TableRows
	var err error
	for _, itm := range item.Children {
		if itm.Data == "colgroup" {
			for _, col := range itm.Children {
				if !(col.Typ == html.ElementNode && col.Data == "col") {
					// not a col element
					continue
				}
				g := node.NewGlue()
				split := strings.Split(col.Attributes["width"], "plus")
				var unitString string
				var stretchString string
				if len(split) == 1 {
					if unitRE.MatchString(split[0]) {
						unitString = split[0]
					} else if astRE.MatchString(split[0]) {
						stretchString = split[0]
					}
				} else {
					if unitRE.MatchString(split[0]) {
						unitString = split[0]
					}
					if astRE.MatchString(split[1]) {
						stretchString = split[1]
					}
				}

				if unitString != "" {
					g.Width = bag.MustSp(unitString)
				}
				if astRE.MatchString(stretchString) {
					astMatch := astRE.FindAllStringSubmatch(stretchString, -1)
					if c := astMatch[0][1]; c != "" {
						stretch, err := strconv.Atoi(c)
						if err != nil {
							return nil, err
						}
						g.Stretch = bag.ScaledPoint(stretch) * bag.Factor
					} else {
						g.Stretch = bag.Factor
					}
					g.StretchOrder = 1
				}
				cs := frontend.ColSpec{
					ColumnWidth: g,
				}
				tbl.ColSpec = append(tbl.ColSpec, cs)
			}

		}
		if itm.Data == "thead" || itm.Data == "tbody" {
			styles := ss.PushStyles()
			for k, v := range itm.Styles {
				switch k {
				case "vertical-align":
					styles.Valign = ParseVerticalAlign(v, styles)
				case "text-align":
					styles.Halign = ParseHorizontalAlign(v, styles)
				case "font-weight":
					var fontweight frontend.FontWeight = 400

					if i, err := strconv.Atoi(v); err == nil {
						fontweight = frontend.FontWeight(i)
					} else {
						switch strings.ToLower(v) {
						case "thin", "hairline":
							fontweight = 100
						case "extra light", "ultra light":
							fontweight = 200
						case "light":
							fontweight = 300
						case "normal":
							fontweight = 400
						case "medium":
							fontweight = 500
						case "semi bold", "demi bold":
							fontweight = 600
						case "bold":
							fontweight = 700
						case "extra bold", "ultra bold":
							fontweight = 800
						case "black", "heavy":
							fontweight = 900
						}
					}
					styles.Fontweight = fontweight
				}
			}
			if rows, err = processTbody(itm, ss, df); err != nil {
				return nil, err
			}
			tbl.Rows = append(tbl.Rows, rows...)
			ss.PopStyles()
		}
	}
	return tbl, nil
}
