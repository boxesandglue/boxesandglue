package document

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	pdf "github.com/speedata/baseline-pdf"
	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/color"
	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/boxesandglue/backend/image"
	"github.com/speedata/boxesandglue/backend/lang"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/frontend/pdfdraw"
	"golang.org/x/exp/slog"
)

const (
	// OneCM has the width of one centimeter in ScaledPoints
	OneCM bag.ScaledPoint = 1857685
)

var (
	cutmarkLength bag.ScaledPoint = OneCM
)

// Object contains a vertical list and coordinates to be placed on a page.
type Object struct {
	X     bag.ScaledPoint
	Y     bag.ScaledPoint
	Vlist *node.VList
}

// A Hyperlink represents a clickable thing in the PDF.
type Hyperlink struct {
	URI       string
	Local     string
	startposX bag.ScaledPoint
	startposY bag.ScaledPoint
}

// A Page struct represents a page in a PDF file.
type Page struct {
	document          *PDFDocument
	Height            bag.ScaledPoint
	Width             bag.ScaledPoint
	ExtraOffset       bag.ScaledPoint
	Background        []Object
	Objects           []Object
	Userdata          map[any]any
	Finished          bool
	StructureElements []*StructureElement
	Annotations       []pdf.Annotation
	Spotcolors        []*color.Color
	Objectnumber      pdf.Objectnumber
	outputDebug       *outputDebug
}

const (
	pdfCodepointMode = 1
	pdfBracketMode   = 2
	pdfTextMode      = 3
	pdfOuterMode     = 4
)

// objectContext contains information about the current position of the cursor
// and about the images and faces used in the object.
type objectContext struct {
	p                *Page
	pageObjectnumber pdf.Objectnumber
	currentFont      *font.Font
	currentExpand    int
	currentVShift    bag.ScaledPoint
	textmode         uint8
	usedFaces        map[*pdf.Face]bool
	usedImages       map[*pdf.Imagefile]bool
	tag              *StructureElement
	s                io.Writer
	shiftX           bag.ScaledPoint
	outputDebug      *outputDebug
	curOutputDebug   *outputDebug
}

func (oc *objectContext) moveto(x, y bag.ScaledPoint) {
	fmt.Fprintf(oc.s, "\n1 0 0 1 %s %s Tm ", x, y)
}

type outputDebug struct {
	Name       string
	Attributes map[string]any
	Items      []any
}

func (od outputDebug) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	startElt := xml.StartElement{
		Name: xml.Name{Local: od.Name},
	}

	keys := make([]string, 0, len(od.Attributes))

	for attrname := range od.Attributes {
		keys = append(keys, attrname)
	}

	sort.Strings(keys)
	for _, key := range keys {
		startElt.Attr = append(startElt.Attr,
			xml.Attr{Name: xml.Name{Local: key}, Value: fmt.Sprint(od.Attributes[key])})

	}
	e.EncodeToken(startElt)
	for _, itm := range od.Items {
		e.Encode(itm)
	}
	e.EncodeToken(startElt.End())
	return nil
}

// gotoTextMode inserts PDF instructions to switch to a inner/outer text mode.
// Textmode 1 is within collection of hexadecimal digits inside angle brackets
// (< ... >), textmode 2 is inside square brackets ([ ... ]), textmode 3 is
// within BT ... ET and textmode 4 is inside the page content stream.
// gotoTextMode makes sure all necessary brackets are opened so you can write
// the data you need.
func (oc *objectContext) gotoTextMode(newMode uint8) {
	if newMode > oc.textmode {
		if oc.textmode == 1 {
			fmt.Fprint(oc.s, ">")
			oc.textmode = 2
		}
		if oc.textmode == 2 && oc.textmode < newMode {
			fmt.Fprint(oc.s, "]TJ\n")
			oc.textmode = 3
		}
		if oc.textmode == 3 && oc.textmode < newMode {
			fmt.Fprint(oc.s, "ET\n")
			if oc.tag != nil {
				fmt.Fprint(oc.s, "EMC\n")
				oc.tag = nil
			}
			oc.textmode = 4

		}
		return
	}
	if newMode < oc.textmode {
		if oc.textmode == 4 {
			if oc.tag != nil {
				fmt.Fprintf(oc.s, "/%s<</MCID %d>>BDC ", oc.tag.Role, oc.tag.ID)
			}
			fmt.Fprint(oc.s, "BT ")
			if oc.currentExpand != 0 {
				fmt.Fprint(oc.s, "100 Tz ")
				oc.currentExpand = 0
			}
			oc.textmode = 3
		}
		if oc.textmode == 3 && newMode < oc.textmode {
			fmt.Fprint(oc.s, "[")
			oc.textmode = 2
		}
		if oc.textmode == 2 && newMode < oc.textmode {
			fmt.Fprint(oc.s, "<")
			oc.textmode = 1
		}
	}
}

// outputHorizontalItems outputs a list of horizontal item and advances the
// cursor. x and y must be the start of the base line coordinate.
func (oc *objectContext) outputHorizontalItems(x, y bag.ScaledPoint, hlist *node.HList) {
	od := &outputDebug{
		Name: "hlist",
		Attributes: map[string]any{
			"valign": hlist.VAlign,
			"width":  hlist.Width,
			"height": hlist.Height,
			"depth":  hlist.Depth,
			"x":      x,
			"y":      y,
		},
	}
	if origin, ok := hlist.Attributes["origin"]; ok {
		od.Attributes["origin"] = origin
	}
	saveCurOutputDebug := oc.curOutputDebug
	oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
	oc.curOutputDebug = od
	sumX := bag.ScaledPoint(0)
	for hItem := hlist.List; hItem != nil; hItem = hItem.Next() {
		switch v := hItem.(type) {
		case *node.Glyph:
			od = &outputDebug{
				Name: "glyph",
				Attributes: map[string]any{
					"cp":         v.Codepoint,
					"fontsize":   v.Font.Size,
					"faceid":     v.Font.Face.FaceID,
					"components": v.Components,
				}}
			oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
			if v.Font != oc.currentFont {
				oc.gotoTextMode(3)
				fmt.Fprintf(oc.s, "\n%s %s Tf ", v.Font.Face.InternalName(), v.Font.Size)
				oc.usedFaces[v.Font.Face] = true
				oc.currentFont = v.Font
			}
			if exp, ok := hlist.Attributes["expand"]; ok {
				if ex, ok := exp.(int); ok {
					if ex != oc.currentExpand {
						oc.gotoTextMode(3)
						fmt.Fprintf(oc.s, "%d Tz ", 100+ex)
						oc.currentExpand = ex
					}
				}
			} else {
				if oc.currentExpand != 0 {
					oc.gotoTextMode(3)
					fmt.Fprint(oc.s, "100 Tz ")
					oc.currentExpand = 0
				}
			}
			if v.YOffset != oc.currentVShift {
				oc.gotoTextMode(3)
				fmt.Fprintf(oc.s, "%s Ts", v.YOffset)
				oc.currentVShift = v.YOffset
			}
			if oc.textmode > 3 {
				oc.gotoTextMode(3)
			}
			if oc.textmode > 2 {
				yPos := y
				if hlist.VAlign == node.VAlignTop {
					yPos -= v.Height
				}
				oc.moveto(x+oc.shiftX+sumX, yPos)
				oc.shiftX = 0
			}
			v.Font.Face.RegisterChar(v.Codepoint)
			oc.gotoTextMode(1)
			fmt.Fprintf(oc.s, "%04x", v.Codepoint)
			sumX = sumX + bag.MultiplyFloat(v.Width, float64(100+oc.currentExpand)/100.0)
		case *node.Glue:
			od := &outputDebug{
				Name: "glue",
				Attributes: map[string]any{
					"width": v.Width,
				}}

			if origin, ok := v.Attributes["origin"]; ok {
				od.Attributes["origin"] = origin
			}
			oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
			if oc.textmode == 2 {
				oc.gotoTextMode(1)
			}
			if oc.textmode == 1 {
				var goBackwards bag.ScaledPoint
				if curFont := oc.currentFont; curFont != nil {
					oc.gotoTextMode(1)
					fmt.Fprintf(oc.s, "%04x", curFont.SpaceChar.Codepoint)
					curFont.Face.RegisterChar(curFont.SpaceChar.Codepoint)
					goBackwards = curFont.SpaceChar.Advance

					if oc.currentFont.Size != 0 {
						oc.gotoTextMode(2)
						fmt.Fprintf(oc.s, " %d ", -1*1000*(v.Width-goBackwards)/oc.currentFont.Size)
					}
				}
			}
			sumX = sumX + bag.MultiplyFloat(v.Width, float64(100+oc.currentExpand)/100.0)
		case *node.Rule:
			od = &outputDebug{
				Name: "rule",
				Attributes: map[string]any{
					"width":  v.Width,
					"height": v.Height,
					"depth":  v.Depth,
				}}
			if origin, ok := v.Attributes["origin"]; ok {
				od.Attributes["origin"] = origin
			}
			oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)

			oc.gotoTextMode(4)
			posX := x + sumX
			posY := y
			if hlist.VAlign == node.VAlignTop {
				posY = y - v.Height - v.Depth
			}
			pdfinstructions := []string{fmt.Sprintf("1 0 0 1 %s %s cm", posX, posY)}

			if v.Pre != "" {
				pdfinstructions = append(pdfinstructions, v.Pre)
			}
			if !v.Hide {
				pdfinstructions = append(pdfinstructions, fmt.Sprintf("q 0 %s %s %s re f Q ", -1*v.Depth, v.Width, v.Height+v.Depth))
			}
			pdfinstructions = append(pdfinstructions, v.Post)
			sumX += v.Width
			pdfinstructions = append(pdfinstructions, fmt.Sprintf("1 0 0 1 %s %s cm\n", -posX, -posY))
			fmt.Fprintf(oc.s, strings.Join(pdfinstructions, " "))
		case *node.Image:
			oc.gotoTextMode(4)
			img := v.Img
			if img.Used {
				slog.Warn(fmt.Sprintf("image node already in use, id: %d", hlist.ID))
			} else {
				img.Used = true
			}
			ifile := img.ImageFile
			oc.usedImages[ifile] = true

			scaleX := hlist.Width.ToPT() / ifile.ScaleX
			scaleY := hlist.Height.ToPT() / ifile.ScaleY
			posy := y
			posx := x + sumX
			fmt.Fprintf(oc.s, "q %f 0 0 %f %s %s cm %s Do Q\n", scaleX, scaleY, posx, posy, img.ImageFile.InternalName())
		case *node.StartStop:
			posX := x + sumX
			posY := y

			isStartNode := true
			action := v.Action

			var startNode *node.StartStop
			if v.StartNode != nil {
				// a stop node which has a link to a start node
				isStartNode = false
				startNode = v.StartNode
				action = startNode.Action
			} else {
				startNode = v
			}

			if action == node.ActionHyperlink {
				hyperlink := startNode.Value.(*Hyperlink)
				if isStartNode {
					hyperlink.startposX = posX
					hyperlink.startposY = posY
				} else {
					rectHT := posY - hyperlink.startposY + hlist.Height + hlist.Depth
					rectWD := posX - hyperlink.startposX
					a := pdf.Annotation{
						Rect:    [4]float64{hyperlink.startposX.ToPT(), hyperlink.startposY.ToPT(), posX.ToPT(), (posY + rectHT).ToPT()},
						Subtype: "Link",
					}
					if oc.p.document.ShowHyperlinks {
						a.Dictionary = pdf.Dict{
							"Border": "[0 0 1]",
						}
					} else {
						a.Dictionary = pdf.Dict{
							"Border": "[0 0 0]",
						}
					}

					if hyperlink.Local != "" {
						a.Action = fmt.Sprintf("<</Type/Action/S/GoTo/D %s>>", pdf.StringToPDF(hyperlink.Local))
					} else if hyperlink.URI != "" {
						a.Action = fmt.Sprintf("<</Type/Action/S/URI/URI %s>>", pdf.StringToPDF(hyperlink.URI))
					}

					oc.p.Annotations = append(oc.p.Annotations, a)
					if oc.p.document.IsTrace(VTraceHyperlinks) {
						oc.gotoTextMode(3)
						fmt.Fprintf(oc.s, "q 0.4 w %s %s %s %s re S Q ", hyperlink.startposX, hyperlink.startposY, rectWD, rectHT)
					}
				}
			} else if action == node.ActionDest {
				// dest should be in the top left corner of the current position
				y := posY + hlist.Height + hlist.Depth
				var destname string
				switch t := v.Value.(type) {
				case int:
					destnum := t
					d := &pdf.NumDest{
						Num:              destnum,
						X:                posX.ToPT(),
						Y:                y.ToPT(),
						PageObjectnumber: oc.pageObjectnumber,
					}
					destname = fmt.Sprintf("%d", destnum)
					oc.p.document.PDFWriter.NumDestinations[destnum] = d
				case string:
					d := &pdf.NameDest{
						Name:             pdf.String(t),
						X:                posX.ToPT(),
						Y:                y.ToPT(),
						PageObjectnumber: oc.pageObjectnumber,
					}
					destname = t
					oc.p.document.PDFWriter.NameDestinations = append(oc.p.document.PDFWriter.NameDestinations, d)
				}

				if oc.p.document.IsTrace(VTraceDest) {
					oc.gotoTextMode(4)
					black := color.Color{Space: color.ColorGray, R: 0, G: 0, B: 0, A: 1}
					circ := pdfdraw.New().ColorStroking(black).Circle(0, 0, 2*bag.Factor, 2*bag.Factor).Fill().String()
					fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", posX, y)
					fmt.Fprint(oc.s, circ)
					fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", -posX, -y)
					oc.debugAt(posX, y, destname)
					// oc.gotoTextMode(4)
				}
			} else if action == node.ActionNone || action == node.ActionUserSetting {
				// ignore
			} else {
				slog.Warn("start/stop node: unhandled action %s", action)
			}
			switch v.Position {
			case node.PDFOutputPage:
				oc.gotoTextMode(4)
			case node.PDFOutputDirect:
				oc.gotoTextMode(3)
			case node.PDFOutputHere:
				oc.gotoTextMode(4)
				fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", posX, posY)
			case node.PDFOutputLowerLeft:
				oc.gotoTextMode(4)
			}
			if v.ShipoutCallback != nil {
				fmt.Fprint(oc.s, v.ShipoutCallback(v))
			}
			switch v.Position {
			case node.PDFOutputHere:
				oc.moveto(-posX, -posY)
			}
		case *node.Kern:
			od = &outputDebug{
				Name: "kern",
				Attributes: map[string]any{
					"kern": v.Kern,
				}}
			if origin, ok := v.Attributes["origin"]; ok {
				od.Attributes["origin"] = origin
			}
			oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)

			if oc.textmode > 2 {
				oc.moveto(x+oc.shiftX+sumX, (y))
				oc.shiftX = 0
			}

			if oc.currentFont != nil {
				oc.gotoTextMode(2)
				fmt.Fprintf(oc.s, " %d ", -1000*v.Kern/oc.currentFont.Size)
			}
			sumX += v.Kern
		case *node.Lang, *node.Penalty:
			// ignore
		case *node.Disc:
			// ignore
		case *node.HList:
			moveY := y
			if hlist.VAlign == node.VAlignTop {
				moveY = moveY - v.Height
			}
			oc.outputHorizontalItems(x+sumX, moveY, v)
			sumX += v.Width
		case *node.VList:
			oc.gotoTextMode(4)
			saveX := x
			saveY := y
			moveY := y + hlist.Height
			if hlist.VAlign == node.VAlignTop {
				moveY = y
			}
			x += sumX
			oc.outputVerticalItems(x+v.ShiftX, moveY, v)
			sumX += v.Width
			x = saveX
			y = saveY
		default:
			slog.Warn(fmt.Sprintf("Shipout: unknown node %v", hItem))
		}
	}
	oc.curOutputDebug = saveCurOutputDebug
}

// outputVerticalItems iterates through the vlist's list and outputs each item
// beneath each other.
func (oc *objectContext) outputVerticalItems(x, y bag.ScaledPoint, vlist *node.VList) {
	od := &outputDebug{
		Name: "vlist",
		Attributes: map[string]any{
			"width":  vlist.Width,
			"height": vlist.Height,
			"depth":  vlist.Depth,
			"x":      x,
			"y":      y,
		},
	}
	saveCurOutputDebug := oc.curOutputDebug
	oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
	oc.curOutputDebug = od
	sumY := bag.ScaledPoint(0)
	for vItem := vlist.List; vItem != nil; vItem = vItem.Next() {
		switch v := vItem.(type) {
		case *node.HList:
			shiftDown := y - sumY - v.Height
			if v.VAlign == node.VAlignTop {
				shiftDown = y - sumY
			}
			if oc.p.document.IsTrace(VTraceHBoxes) {
				r := node.NewRule()
				r.Hide = true
				p := pdfdraw.NewStandalone().LineWidth(bag.MustSp("0.4pt")).Rect(0, -v.Depth, v.Width, v.Height+v.Depth).Stroke()
				r.Pre = p.String()
				v.List = node.InsertBefore(v.List, v.List, r)
			}
			oc.outputHorizontalItems(x, shiftDown, v)
			sumY += v.Height
			sumY += v.Depth
			if oc.textmode < 3 {
				oc.gotoTextMode(3)
			}
		case *node.Image:
			img := v.Img
			if img.Used {
				slog.Warn(fmt.Sprintf("image node already in use, id: %d", v.ID))
			} else {
				img.Used = true
			}
			ifile := img.ImageFile
			oc.usedImages[ifile] = true
			oc.gotoTextMode(4)

			scaleX := v.Width.ToPT() / ifile.ScaleX
			scaleY := v.Height.ToPT() / ifile.ScaleY

			ht := v.Height
			posy := y - ht
			posx := x
			if oc.p.document.IsTrace(VTraceImages) {
				fmt.Fprintf(oc.s, "q 0.2 w %s %s %s %s re S Q\n", x, y, v.Width, v.Height)
			}
			fmt.Fprintf(oc.s, "q %f 0 0 %f %s %s cm %s Do Q\n", scaleX, scaleY, posx, posy, img.ImageFile.InternalName())
		case *node.Glue:
			od = &outputDebug{
				Name: "glue",
				Attributes: map[string]any{
					"width":   v.Width,
					"stretch": v.Stretch,
					"shrink":  v.Shrink,
				}}
			oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)

			// Let's assume that the glue ratio has been determined and the
			// natural width is in v.Width for now.
			sumY += v.Width
		case *node.Rule:
			od = &outputDebug{
				Name: "rule",
				Attributes: map[string]any{
					"width":  v.Width,
					"height": v.Height,
					"depth":  v.Depth,
				}}
			if origin, ok := v.Attributes["origin"]; ok {
				od.Attributes["origin"] = origin
			}
			oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)

			posX := x
			posY := y - sumY
			sumY += v.Height + v.Depth
			pdfinstructions := []string{fmt.Sprintf("1 0 0 1 %s %s cm", posX, posY)}
			if v.Pre != "" {
				pdfinstructions = append(pdfinstructions, v.Pre)
			}
			if !v.Hide {
				pdfinstructions = append(pdfinstructions, fmt.Sprintf("q 0 0 %s %s re f Q", v.Width, -1*(v.Height+v.Depth)))
			}
			if v.Post != "" {
				pdfinstructions = append(pdfinstructions, v.Post)
			}
			pdfinstructions = append(pdfinstructions, fmt.Sprintf("1 0 0 1 %s %s cm\n", -posX, -posY))
			fmt.Fprintf(oc.s, strings.Join(pdfinstructions, " "))
		case *node.StartStop:
			posX := x
			posY := y

			isStartNode := true
			action := v.Action

			var startNode *node.StartStop
			if v.StartNode != nil {
				// a stop node which has a link to a start node
				isStartNode = false
				startNode = v.StartNode
				action = startNode.Action
			} else {
				startNode = v
			}

			if action == node.ActionHyperlink {
				hyperlink := startNode.Value.(*Hyperlink)
				if isStartNode {
					hyperlink.startposX = posX
					hyperlink.startposY = posY
				} else {
					rectHT := posY - hyperlink.startposY + vlist.Height + vlist.Depth
					rectWD := posX - hyperlink.startposX
					a := pdf.Annotation{
						Rect:    [4]float64{hyperlink.startposX.ToPT(), hyperlink.startposY.ToPT(), posX.ToPT(), (posY + rectHT).ToPT()},
						Subtype: "Link",
					}
					if oc.p.document.ShowHyperlinks {
						a.Dictionary = pdf.Dict{
							"Border": "[0 0 1]",
						}
					} else {
						a.Dictionary = pdf.Dict{
							"Border": "[0 0 0]",
						}
					}

					if hyperlink.Local != "" {
						a.Action = fmt.Sprintf("<</Type/Action/S/GoTo/D %s>>", pdf.StringToPDF(hyperlink.Local))
					} else if hyperlink.URI != "" {
						a.Action = fmt.Sprintf("<</Type/Action/S/URI/URI %s>>", pdf.StringToPDF(hyperlink.URI))
					}

					oc.p.Annotations = append(oc.p.Annotations, a)
					if oc.p.document.IsTrace(VTraceHyperlinks) {
						oc.gotoTextMode(3)
						fmt.Fprintf(oc.s, "q 0.4 w %s %s %s %s re S Q ", hyperlink.startposX, hyperlink.startposY, rectWD, rectHT)
					}
				}
			} else if action == node.ActionDest {
				// dest should be in the top left corner of the current position
				y := posY + vlist.Height + vlist.Depth
				var destname string // for debugging only
				switch t := v.Value.(type) {
				case int:
					destnum := t
					d := &pdf.NumDest{
						Num:              destnum,
						X:                posX.ToPT(),
						Y:                y.ToPT(),
						PageObjectnumber: oc.pageObjectnumber,
					}
					destname = fmt.Sprintf("%d", destnum)
					oc.p.document.PDFWriter.NumDestinations[destnum] = d
				case string:
					d := &pdf.NameDest{
						Name:             pdf.String(t),
						X:                posX.ToPT(),
						Y:                y.ToPT(),
						PageObjectnumber: oc.pageObjectnumber,
					}
					destname = t
					oc.p.document.PDFWriter.NameDestinations = append(oc.p.document.PDFWriter.NameDestinations, d)
				}

				if oc.p.document.IsTrace(VTraceDest) {
					oc.gotoTextMode(4)
					black := color.Color{Space: color.ColorGray, R: 0, G: 0, B: 0, A: 1}
					circ := pdfdraw.New().ColorStroking(black).Circle(0, 0, 2*bag.Factor, 2*bag.Factor).Fill().String()
					fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", posX, y)
					fmt.Fprint(oc.s, circ)
					fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", -posX, -y)
					oc.debugAt(posX, y, destname)
				}
			} else if action == node.ActionNone {
				// ignore
			} else {
				slog.Warn(fmt.Sprintf("start/stop node: unhandled action %s", action))
			}
			switch v.Position {
			case node.PDFOutputPage:
				oc.gotoTextMode(4)
			case node.PDFOutputDirect:
				oc.gotoTextMode(3)
			case node.PDFOutputHere:
				oc.gotoTextMode(4)
				fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", posX, posY)
			case node.PDFOutputLowerLeft:
				oc.gotoTextMode(4)
			}
			if v.ShipoutCallback != nil {
				fmt.Fprint(oc.s, v.ShipoutCallback(v))
			}
			switch v.Position {
			case node.PDFOutputHere:
				oc.moveto(-posX, -posY)
			}
		case *node.VList:
			oc.outputVerticalItems(x+v.ShiftX, y-sumY, v)
			sumY += v.Height + v.Depth
		default:
			slog.Error(fmt.Sprintf("Shipout: unknown node %T in vertical mode", v))
		}
	}
	oc.curOutputDebug = saveCurOutputDebug
}

func (oc objectContext) debugAt(x, y bag.ScaledPoint, text string) {
	if len(oc.p.document.Faces) == 0 {
		return
	}
	oc.gotoTextMode(3)
	f0 := oc.p.document.Faces[0]
	fmt.Fprintf(oc.s, " q %s 8 Tf 0 0 1 rg ", f0.InternalName())
	oc.moveto(x, y)
	fmt.Fprint(oc.s, "[<")
	fnt := oc.p.document.CreateFont(f0, 4*bag.Factor)
	cp := []int{}
	for _, v := range fnt.Shape(text, nil) {
		fmt.Fprintf(oc.s, "%04x", v.Codepoint)
		cp = append(cp, v.Codepoint)
	}
	f0.RegisterChars(cp)
	fmt.Fprint(oc.s, ">]TJ Q ")
	oc.gotoTextMode(4)
}

// OutputAt places the nodelist at the position.
func (p *Page) OutputAt(x bag.ScaledPoint, y bag.ScaledPoint, vlist *node.VList) {
	p.Objects = append(p.Objects, Object{x, y, vlist})
}

// Shipout places all objects on a page and finishes this page.
func (p *Page) Shipout() {
	slog.Debug("Shipout")
	if p.Finished {
		return
	}
	p.Finished = true

	pageObjectNumber := p.document.PDFWriter.NextObject()
	var s strings.Builder
	for _, cb := range p.document.preShipoutCallback {
		cb(p)
	}
	bleedamount := p.document.Bleed

	// ExtraOffset is cutmarks length + bleed amount
	offsetX := p.ExtraOffset
	offsetY := p.ExtraOffset

	if p.document.ShowCutmarks {
		x, y, wd, ht := p.ExtraOffset, p.ExtraOffset, p.Width+p.ExtraOffset, p.Height+p.ExtraOffset
		distance, length, width := 5*bag.Factor, cutmarkLength, bag.Factor/2
		if distance < bleedamount {
			distance = bleedamount
		}

		instructions := []string{fmt.Sprintf("q 1 1 1 1 K %s w 1 0 0 1 %s %s Tm", width, bag.ScaledPoint(0), bag.ScaledPoint(0))}
		// top right
		instructions = append(instructions, fmt.Sprintf("%s %s m %s %s l S", wd, ht+distance, wd, ht+distance+length))
		instructions = append(instructions, fmt.Sprintf("%s %s m %s %s l S", wd+distance, ht, wd+distance+length, ht))

		// top left
		instructions = append(instructions, fmt.Sprintf("%s %s m %s %s l S", x, ht+distance, x, ht+distance+length))
		instructions = append(instructions, fmt.Sprintf("%s %s m %s %s l S", x-distance, ht, x-length-distance, ht))

		// bottom left
		instructions = append(instructions, fmt.Sprintf("%s %s m %s %s l S", x, y-distance, x, y-length-distance))
		instructions = append(instructions, fmt.Sprintf("%s %s m %s %s l S", x-distance, y, x-length-distance, y))

		// bottom right
		instructions = append(instructions, fmt.Sprintf("%s %s m %s %s l S", wd, y-distance, wd, y-length-distance))
		instructions = append(instructions, fmt.Sprintf("%s %s m %s %s l S", wd+distance, y, wd+distance+length, y))

		instructions = append(instructions, "Q ")
		s.WriteString(strings.Join(instructions, "\n"))
	}

	objs := make([]Object, 0, len(p.Background)+len(p.Objects))
	objs = append(objs, p.Background...)
	objs = append(objs, p.Objects...)
	usedFaces := make(map[*pdf.Face]bool)
	usedImages := make(map[*pdf.Imagefile]bool)
	p.outputDebug = &outputDebug{
		Name: "page",
	}

	st := p.document.PDFWriter.NewObject()
	st.SetCompression(p.document.CompressLevel)

	for _, obj := range objs {
		oc := &objectContext{
			textmode:         4,
			s:                st.Data,
			usedFaces:        make(map[*pdf.Face]bool),
			usedImages:       make(map[*pdf.Imagefile]bool),
			p:                p,
			pageObjectnumber: pageObjectNumber,
			outputDebug: &outputDebug{
				Name: "object",
			},
		}
		oc.curOutputDebug = oc.outputDebug

		vlist := obj.Vlist
		if vlist.Attributes != nil {
			if r, ok := vlist.Attributes["tag"]; ok {
				oc.tag = r.(*StructureElement)
				oc.tag.ID = len(p.StructureElements)
				p.StructureElements = append(p.StructureElements, oc.tag)
			}
		}

		x := obj.X + offsetX
		y := obj.Y + offsetY
		// output vertical items
		oc.outputVerticalItems(x, y, vlist)
		for k := range oc.usedFaces {
			usedFaces[k] = true
		}
		for k := range oc.usedImages {
			usedImages[k] = true
		}
		oc.gotoTextMode(4)
		p.outputDebug.Items = append(p.outputDebug.Items, oc.outputDebug)
	}

	page := p.document.PDFWriter.AddPage(st, pageObjectNumber)
	page.Dict = make(pdf.Dict)
	page.Width = (p.Width + 2*offsetX).ToPT()
	page.Height = (p.Height + 2*offsetY).ToPT()
	page.Dict["TrimBox"] = fmt.Sprintf("[%s %s %s %s]", p.ExtraOffset, p.ExtraOffset, pdf.FloatToPoint(page.Width-p.ExtraOffset.ToPT()), pdf.FloatToPoint(page.Height-p.ExtraOffset.ToPT()))
	if bleedamount > 0 {
		page.Dict["BleedBox"] = fmt.Sprintf("[%s %s %s %s]", p.ExtraOffset-bleedamount, p.ExtraOffset-bleedamount, pdf.FloatToPoint(page.Width-p.ExtraOffset.ToPT()+bleedamount.ToPT()), pdf.FloatToPoint(page.Height-p.ExtraOffset.ToPT()+bleedamount.ToPT()))
	}

	for f := range usedFaces {
		page.Faces = append(page.Faces, f)
	}
	for i := range usedImages {
		page.Images = append(page.Images, i)
	}

	var structureElementObjectIDs []string
	// annotations are hyperlinks and structure elements
	page.Annotations = p.Annotations

	if p.document.RootStructureElement != nil {

		for _, se := range p.StructureElements {
			parent := se.Parent
			if parent.Obj == nil {
				parent.Obj = p.document.PDFWriter.NewObject()
			}
			se.Obj = p.document.PDFWriter.NewObject()
			se.Obj.Dictionary = pdf.Dict{
				"Type": "/StructElem",
				"S":    "/" + se.Role,
				"K":    fmt.Sprintf("%d", se.ID),
				"Pg":   page.Dictnum.Ref(),
				"P":    parent.Obj.ObjectNumber.Ref(),
			}
			if se.ActualText != "" {
				se.Obj.Dictionary["ActualText"] = pdf.StringToPDF(se.ActualText)
			}
			se.Obj.Save()
			structureElementObjectIDs = append(structureElementObjectIDs, se.Obj.ObjectNumber.Ref())
		}
		po := p.document.newPDFStructureObject()
		po.refs = strings.Join(structureElementObjectIDs, " ")
		page.Dict["StructParents"] = fmt.Sprintf("%d", po.id)
		p.document.pdfStructureObjects = append(p.document.pdfStructureObjects, po)
	}
	for _, s := range p.document.Spotcolors {
		p.Spotcolors = append(p.Spotcolors, s)
	}

}

// CallbackShipout gets called before the shipout process starts.
type CallbackShipout func(page *Page)

// StructureElement represents a tagged PDF element such as H1 or P.
type StructureElement struct {
	ID         int
	Role       string
	ActualText string
	children   []*StructureElement
	Parent     *StructureElement
	Obj        *pdf.Object
}

// AddChild adds a child element (such as a span in a paragraph) to the element
// given with se. AddChild sets the parent pointer of the child.
func (se *StructureElement) AddChild(cld *StructureElement) {
	se.children = append(se.children, cld)
	cld.Parent = se
}

// pdfStructureObject holds information about the PDF/UA structures for each
// page, annotation and XObject.
type pdfStructureObject struct {
	id   int
	refs string
}

func (d *PDFDocument) newPDFStructureObject() *pdfStructureObject {
	po := &pdfStructureObject{}
	po.id = len(d.pdfStructureObjects)
	return po
}

// PDFDocument contains all references to a document
type PDFDocument struct {
	Author               string
	Bleed                bag.ScaledPoint
	ColorProfile         *ColorProfile
	CompressLevel        uint
	Creator              string
	CreationDate         time.Time
	CurrentPage          *Page
	DefaultLanguage      *lang.Lang
	DefaultPageHeight    bag.ScaledPoint
	DefaultPageWidth     bag.ScaledPoint
	Faces                []*pdf.Face
	Filename             string
	Keywords             string
	Languages            map[string]*lang.Lang
	Pages                []*Page
	PDFWriter            *pdf.PDF
	RootStructureElement *StructureElement
	ShowCutmarks         bool
	ShowHyperlinks       bool
	Spotcolors           []*color.Color
	Subject              string
	SuppressInfo         bool
	Title                string
	ViewerPreferences    map[string]string
	producer             string
	tracing              VTrace
	outputDebug          *outputDebug
	curOutputDebug       *outputDebug
	pdfStructureObjects  []*pdfStructureObject
	preShipoutCallback   []CallbackShipout
	usedPDFImages        map[string]*pdf.Imagefile
}

// NewDocument creates an empty document.
func NewDocument(w io.Writer) *PDFDocument {
	d := &PDFDocument{
		DefaultPageWidth:  bag.MustSp("210mm"),
		DefaultPageHeight: bag.MustSp("297mm"),
		CreationDate:      time.Now(),
		Languages:         make(map[string]*lang.Lang),
		ViewerPreferences: make(map[string]string),
		PDFWriter:         pdf.NewPDFWriter(w),
		CompressLevel:     9,
		producer:          "speedata/boxesandglue",
		usedPDFImages:     make(map[string]*pdf.Imagefile),
		outputDebug: &outputDebug{
			Name: "pdfdocument",
		},
	}
	d.curOutputDebug = d.outputDebug
	d.PDFWriter.Logger = slog.Default()
	return d
}

// OutputXMLDump writes an XML dump of the document to w.
func (d *PDFDocument) OutputXMLDump(w io.Writer) error {
	for _, pg := range d.Pages {
		d.outputDebug.Items = append(d.outputDebug.Items, pg.outputDebug)
	}
	b, err := xml.MarshalIndent(d.outputDebug, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// LoadPatternFile loads a hyphenation pattern file.
func (d *PDFDocument) LoadPatternFile(filename string, langname string) (*lang.Lang, error) {
	l, err := lang.LoadPatternFile(filename)
	if err != nil {
		return nil, err
	}
	d.Languages[langname] = l
	return l, nil
}

// SetDefaultLanguage sets the document default language.
func (d *PDFDocument) SetDefaultLanguage(l *lang.Lang) {
	d.DefaultLanguage = l
}

// LoadFace loads a font from a TrueType or OpenType collection.
func (d *PDFDocument) LoadFace(filename string, index int) (*pdf.Face, error) {
	// face already loaded? TODO: check index
	for _, fce := range d.Faces {
		if fce.Filename == filename {
			return fce, nil
		}
	}
	slog.Debug("LoadFace", "filename", filename)

	f, err := pdf.LoadFace(d.PDFWriter, filename, index)
	if err != nil {
		return nil, err
	}
	d.Faces = append(d.Faces, f)
	return f, nil
}

// LoadImageFile loads an image file. Images that should be placed in the PDF
// file must be derived from the file. For PDF files this defaults to the
// /MediaBox and page 1.
func (d *PDFDocument) LoadImageFile(filename string) (*pdf.Imagefile, error) {
	return d.LoadImageFileWithBox(filename, "/MediaBox", 1)
}

// LoadImageFileWithBox loads an image file. Images that should be placed in the PDF
// file must be derived from the file.
func (d *PDFDocument) LoadImageFileWithBox(filename string, box string, pagenumber int) (*pdf.Imagefile, error) {
	key := fmt.Sprintf("%s-%s-%d", filename, box, pagenumber)
	if imgf, ok := d.usedPDFImages[key]; ok {
		return imgf, nil
	}
	imgf, err := pdf.LoadImageFileWithBox(d.PDFWriter, filename, box, pagenumber)
	if err != nil {
		return nil, err
	}
	d.usedPDFImages[key] = imgf
	return imgf, nil
}

// CreateImage returns a new Image derived from the image file. The parameter
// pagenumber is honored only in PDF files. The Box is one of "/MediaBox", "/CropBox",
// "/TrimBox", "/BleedBox" or "/ArtBox"
func (d *PDFDocument) CreateImage(imgfile *pdf.Imagefile, pagenumber int, box string) *image.Image {
	img := &image.Image{}
	img.ImageFile = imgfile
	img.PageNumber = pagenumber
	switch img.ImageFile.Format {
	case "pdf":
		mb, err := imgfile.GetPDFBoxDimensions(pagenumber, box)
		if err != nil {
			return nil
		}

		img.Width = bag.ScaledPointFromFloat(mb["w"])
		img.Height = bag.ScaledPointFromFloat(mb["h"])
	case "jpeg", "png":
		img.Width = bag.ScaledPoint(imgfile.W) * bag.Factor
		img.Height = bag.ScaledPoint(imgfile.H) * bag.Factor
	}
	return img
}

// NewPage creates a new Page object and adds it to the page list in the
// document. The CurrentPage field of the document is set to the page.
func (d *PDFDocument) NewPage() *Page {
	d.CurrentPage = &Page{
		document: d,
		Width:    d.DefaultPageWidth,
		Height:   d.DefaultPageHeight,
	}
	if d.ShowCutmarks {
		d.CurrentPage.ExtraOffset = cutmarkLength
	}
	d.CurrentPage.ExtraOffset += d.Bleed

	d.Pages = append(d.Pages, d.CurrentPage)
	return d.CurrentPage
}

// CreateFont returns a new Font object for this face at a given size.
func (d *PDFDocument) CreateFont(face *pdf.Face, size bag.ScaledPoint) *font.Font {
	return font.NewFont(face, size)
}

// Finish writes all objects to the PDF and writes the XRef section. Finish does
// not close the writer.
func (d *PDFDocument) Finish() error {
	var err error
	d.PDFWriter.Catalog = pdf.Dict{}
	if d.ColorProfile != nil {
		cp := d.PDFWriter.NewObject()
		cp.Data.Write(d.ColorProfile.data)
		cp.Dictionary = pdf.Dict{
			"N": fmt.Sprintf("%d", d.ColorProfile.Colors),
		}
		if err = cp.Save(); err != nil {
			return err
		}
		for _, col := range d.Spotcolors {
			sep := pdf.Separation{}
			sep.ICCProfile = cp.ObjectNumber
			sep.C = col.C
			sep.M = col.M
			sep.Y = col.Y
			sep.K = col.K
			sep.Name = col.Basecolor
			sep.ID = fmt.Sprintf("/CS%d", col.SpotcolorID)

			sepObj := d.PDFWriter.NewObject()
			sepObj.Array = []any{
				"/Separation",
				pdf.Name(col.Basecolor),
				pdf.Array{"/ICCBased", cp.ObjectNumber},
				pdf.Dict{
					"C0":           pdf.ArrayToString(pdf.Array{0, 0, 0, 0}),
					"C1":           pdf.ArrayToString(pdf.Array{sep.C, sep.M, sep.Y, sep.K}),
					"Domain":       "[ 0 1 ]",
					"FunctionType": "2",
					"N":            "1",
				},
			}
			sepObj.Save()
			sep.Obj = sepObj.ObjectNumber
			d.PDFWriter.Colorspaces = append(d.PDFWriter.Colorspaces, &sep)
		}
	}

	if se := d.RootStructureElement; se != nil {
		if se.Obj == nil {
			se.Obj = d.PDFWriter.NewObject()
		}
		var poStr strings.Builder

		// structure objects are a used to lookup structure elements for a page
		for _, po := range d.pdfStructureObjects {
			poStr.WriteString(fmt.Sprintf("%d [%s]", po.id, po.refs))
		}
		childObjectNumbers := []string{}
		for _, childSe := range se.children {
			childObjectNumbers = append(childObjectNumbers, childSe.Obj.ObjectNumber.Ref())
		}
		structRoot := d.PDFWriter.NewObject()
		structRoot.Dictionary = pdf.Dict{
			"Type":       "/StructTreeRoot",
			"ParentTree": fmt.Sprintf("<< /Nums [ %s ] >>", poStr.String()),
			"K":          se.Obj.ObjectNumber.Ref(),
		}
		structRoot.Save()
		se.Obj.Dictionary = pdf.Dict{
			"S":    "/" + se.Role,
			"K":    fmt.Sprintf("%s", childObjectNumbers),
			"P":    structRoot.ObjectNumber.Ref(),
			"Type": "/StructElem",
			"T":    pdf.StringToPDF(d.Title),
		}
		se.Obj.Save()

		d.PDFWriter.Catalog["StructTreeRoot"] = structRoot.ObjectNumber.Ref()
		d.ViewerPreferences["ViewerPreferences"] = "<< /DisplayDocTitle true >>"
		d.PDFWriter.Catalog["Lang"] = "(en)"
		d.PDFWriter.Catalog["MarkInfo"] = `<< /Marked true /Suspects false  >>`

	}

	rdf := d.PDFWriter.NewObject()
	rdf.Data.WriteString(d.getMetadata())
	rdf.Dictionary = pdf.Dict{
		"Type":    "/Metadata",
		"Subtype": "/XML",
	}
	err = rdf.Save()
	if err != nil {
		return err
	}
	d.PDFWriter.Catalog["Metadata"] = rdf.ObjectNumber.Ref()
	for k, v := range d.ViewerPreferences {
		d.PDFWriter.Catalog[pdf.Name(k)] = v
	}
	d.PDFWriter.DefaultPageWidth = d.DefaultPageWidth.ToPT()
	d.PDFWriter.DefaultPageHeight = d.DefaultPageHeight.ToPT()

	d.PDFWriter.InfoDict = pdf.Dict{
		"Producer": pdf.StringToPDF(d.producer),
	}
	if t := d.Title; t != "" {
		d.PDFWriter.InfoDict["Title"] = pdf.String(t)
	}
	if t := d.Author; t != "" {
		d.PDFWriter.InfoDict["Author"] = pdf.StringToPDF(t)
	}
	if t := d.Creator; t != "" {
		d.PDFWriter.InfoDict["Creator"] = pdf.StringToPDF(t)
	}
	if t := d.Subject; t != "" {
		d.PDFWriter.InfoDict["Subject"] = pdf.StringToPDF(t)
	}
	if t := d.Keywords; t != "" {
		d.PDFWriter.InfoDict["Keywords"] = pdf.StringToPDF(t)
	}
	d.PDFWriter.InfoDict["CreationDate"] = d.CreationDate.Format("(D:20060102150405)")

	if err = d.PDFWriter.Finish(); err != nil {
		return err
	}
	if d.Filename != "" {
		slog.Info("Output written", "filename", d.Filename, "bytes", d.PDFWriter.Size())
	} else {
		slog.Info("Output written (%d bytes)", d.PDFWriter.Size())
	}

	return nil
}

// Callback represents the type of callback to register.
type Callback int

const (
	// CallbackPreShipout is called right before a page shipout. It is called once for each page.
	CallbackPreShipout Callback = iota
)

// RegisterCallback registers the callback in fn.
func (d *PDFDocument) RegisterCallback(cb Callback, fn any) {
	switch cb {
	case CallbackPreShipout:
		d.preShipoutCallback = append(d.preShipoutCallback, fn.(func(page *Page)))
	}
}
