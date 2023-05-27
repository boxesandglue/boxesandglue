package htmlstyle

import (
	"strconv"

	"github.com/speedata/boxesandglue/frontend"
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

			x, err := Output(itm, ss, df)
			if err != nil {
				return nil, err
			}
			tc.Contents = append(tc.Contents, x)
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
		if itm.Data == "thead" || itm.Data == "tbody" {
			styles := ss.PushStyles()
			for k, v := range itm.Styles {
				switch k {
				case "vertical-align":
					styles.Valign = ParseVerticalAlign(v, styles)
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
