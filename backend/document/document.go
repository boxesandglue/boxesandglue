package document

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/lang"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/boxesandglue/frontend/pdfdraw"
	"github.com/boxesandglue/svgreader"
)

const (
	// OneCM has the width of one centimeter in ScaledPoints
	OneCM bag.ScaledPoint = 1857685
)

var cutmarkLength bag.ScaledPoint = OneCM

// Object contains a vertical list and coordinates to be placed on a page.
type Object struct {
	Vlist *node.VList
	X     bag.ScaledPoint
	Y     bag.ScaledPoint
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
	Userdata          map[any]any
	outputDebug       *outputDebug
	Background        []Object
	Objects           []Object
	StructureElements []*StructureElement
	Annotations       []pdf.Annotation
	Spotcolors        []*color.Color
	Height            bag.ScaledPoint
	Width             bag.ScaledPoint
	ExtraOffset       bag.ScaledPoint
	Objectnumber      pdf.Objectnumber
	nextMCID          int
	pageIndex         int // index of this page in PDFDocument.Pages
	Finished          bool
}

// Format returns the PDF flavor
type Format int

const (
	// FormatPDF is the standard PDF format
	FormatPDF Format = iota
	// FormatPDFA3b is the PDF/A-3b format
	FormatPDFA3b
	// FormatPDFX3 is the PDF/X-3 format
	FormatPDFX3
	// FormatPDFX4 is the PDF/X-4 format
	FormatPDFX4
	// FormatPDFUA is the PDF/UA format
	FormatPDFUA
)

// TextScope represents the nesting level within PDF content stream.
// The values form a hierarchy from innermost (ScopeGlyph) to outermost (ScopePage).
type TextScope uint8

const (
	// ScopeGlyph is inside hex string < ... > for glyph codepoints
	ScopeGlyph TextScope = 1
	// ScopeArray is inside square brackets [ ... ] for TJ arrays
	ScopeArray TextScope = 2
	// ScopeText is inside BT ... ET text object
	ScopeText TextScope = 3
	// ScopePage is in page content stream, outside text object
	ScopePage TextScope = 4
)

// objectContext contains information about the current position of the cursor
// and about the images and faces used in the object.
type objectContext struct {
	s                io.Writer
	p                *Page
	currentFont      *font.Font
	usedFaces        map[*pdf.Face]bool
	usedImages       map[*pdf.Imagefile]bool
	tag              *StructureElement
	artifactType     ArtifactType
	outputDebug      *outputDebug
	curOutputDebug   *outputDebug
	pageObjectnumber pdf.Objectnumber
	currentExpand    int
	currentVShift    bag.ScaledPoint
	shiftX           bag.ScaledPoint
	textmode         TextScope
	hasNewline       bool
	inArtifact       bool
}

// emitBDC writes a BDC operator with MCID. ActualText is placed on the
// structure element object only (Table 355), not in the BDC properties
// dictionary (see pdf-association/pdf-issues#60).
func (oc *objectContext) emitBDC(se *StructureElement, mcid int) {
	oc.writef("/%s <</MCID %d>> BDC\n", se.Role, mcid)
}

// writef writes a formatted string to the object stream
func (oc *objectContext) writef(format string, args ...any) {
	fmt.Fprintf(oc.s, format, args...)
	oc.hasNewline = false
}

// write emits a non-formatted string to the object stream
func (oc *objectContext) write(args ...any) {
	fmt.Fprint(oc.s, args...)
	oc.hasNewline = false
}

// newline makes sure a newline is inserted at the current position
func (oc *objectContext) newline() {
	if !oc.hasNewline {
		fmt.Fprintln(oc.s)
		oc.hasNewline = true
	}
}

func (oc *objectContext) moveto(x, y bag.ScaledPoint) {
	// Tm must be inside a BT...ET block, so ensure we're in ScopeText
	if oc.textmode > ScopeText {
		oc.gotoTextMode(ScopeText)
	}
	oc.newline()
	oc.writef("1 0 0 1 %s %s Tm ", x, y)
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

// gotoTextMode inserts PDF instructions to switch to an inner/outer text mode.
// The modes form a hierarchy from innermost to outermost:
//   - ScopeGlyph (1): inside hex string < ... >
//   - ScopeArray (2): inside TJ array [ ... ]
//   - ScopeText  (3): inside text object BT ... ET
//   - ScopePage  (4): page content stream, outside text object
//
// gotoTextMode makes sure all necessary brackets are opened/closed so you can
// write the data you need at the requested scope level.
func (oc *objectContext) gotoTextMode(newMode TextScope) {
	if newMode > oc.textmode {
		if oc.textmode == ScopeGlyph {
			oc.writef(">")
			oc.textmode = ScopeArray
		}
		if oc.textmode == ScopeArray && oc.textmode < newMode {
			oc.writef("]TJ")
			oc.newline()
			oc.textmode = ScopeText
		}
		if oc.textmode == ScopeText && oc.textmode < newMode {
			oc.newline()
			oc.writef("ET")
			oc.newline()
			oc.textmode = ScopePage
		}
		return
	}
	if newMode < oc.textmode {
		if oc.textmode == ScopePage {
			oc.writef("BT ")
			if oc.currentExpand != 0 {
				oc.writef("100 Tz ")
				oc.currentExpand = 0
			}
			// Reset text rise (Ts) at start of each text object.
			// PDF text state persists across BT...ET blocks, but objectContext
			// is created fresh for each object with currentVShift=0. Without this
			// reset, a non-zero Ts from a previous object would persist.
			oc.writef("0 Ts ")
			oc.textmode = ScopeText
		}
		if oc.textmode == ScopeText && newMode < oc.textmode {
			oc.writef("[")
			oc.textmode = ScopeArray
		}
		if oc.textmode == ScopeArray && newMode < oc.textmode {
			oc.writef("<")
			oc.textmode = ScopeGlyph
		}
	}
}

// outputHorizontalItems outputs a list of horizontal item and advances the
// cursor. x and y must be the start of the base line coordinate.
func (oc *objectContext) outputHorizontalItems(x, y bag.ScaledPoint, hlist *node.HList) {
	var od, saveCurOutputDebug *outputDebug
	if oc.p.document.DumpOutput {
		od = &outputDebug{
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
		saveCurOutputDebug = oc.curOutputDebug
		oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
		oc.curOutputDebug = od
	}
	sumX := bag.ScaledPoint(0)
	for hItem := hlist.List; hItem != nil; hItem = hItem.Next() {
		switch v := hItem.(type) {
		case *node.Glyph:
			if oc.p.document.DumpOutput {
				od = &outputDebug{
					Name: "glyph",
					Attributes: map[string]any{
						"cp":         v.Codepoint,
						"fontsize":   v.Font.Size,
						"faceid":     v.Font.Face.FaceID,
						"components": v.Components,
					},
				}
				oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
			}
			if v.Font != oc.currentFont {
				oc.gotoTextMode(ScopeText)
				oc.newline()
				oc.writef("%s %s Tf ", v.Font.Face.InternalName(), bag.MultiplyFloat(v.Font.Size, v.Font.Face.Scale))
				oc.usedFaces[v.Font.Face] = true
				oc.currentFont = v.Font
			}
			if exp, ok := hlist.Attributes["expand"]; ok {
				if ex, ok := exp.(int); ok {
					if ex != oc.currentExpand {
						oc.gotoTextMode(ScopeText)
						oc.writef("%d Tz ", 100+ex)
						oc.currentExpand = ex
					}
				}
			} else {
				if oc.currentExpand != 0 {
					oc.gotoTextMode(ScopeText)
					oc.writef("100 Tz ")
					oc.currentExpand = 0
				}
			}
			if v.YOffset != oc.currentVShift {
				oc.gotoTextMode(ScopeText)
				oc.writef("%s Ts", v.YOffset)
				oc.currentVShift = v.YOffset
			}
			if oc.textmode > ScopeText {
				oc.gotoTextMode(ScopeText)
			}
			if oc.textmode > ScopeArray {
				yPos := y
				if hlist.VAlign == node.VAlignTop {
					yPos -= v.Height
				}
				oc.moveto(x+oc.shiftX+sumX, yPos)
				oc.shiftX = 0
			}
			v.Font.Face.RegisterCodepoint(v.Codepoint)
			// Handle GPOS XOffset for mark positioning (visual shift without affecting text flow)
			var xOffsetMove int
			if v.XOffset != 0 && oc.currentFont != nil && oc.currentFont.Size != 0 {
				adv := v.XOffset.ToPT() / oc.currentFont.Size.ToPT()
				scale := oc.currentFont.Face.Scale
				xOffsetMove = int(-1 * 1000 / scale * adv)
				if xOffsetMove != 0 {
					oc.gotoTextMode(ScopeArray)
					oc.writef(" %d ", xOffsetMove)
				}
			}
			oc.gotoTextMode(ScopeGlyph)
			oc.writef("%04x", v.Codepoint)
			// Reverse XOffset adjustment to restore text position
			if xOffsetMove != 0 {
				oc.gotoTextMode(ScopeArray)
				oc.writef(" %d ", -xOffsetMove)
			}
			sumX += bag.MultiplyFloat(v.Width, float64(100+oc.currentExpand)/100.0)
		case *node.Glue:
			var od *outputDebug
			if oc.p.document.DumpOutput {
				od = &outputDebug{
					Name: "glue",
					Attributes: map[string]any{
						"width": v.Width,
					},
				}

				if origin, ok := v.Attributes["origin"]; ok {
					od.Attributes["origin"] = origin
				}
				oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
			}
			if v.Leader != nil && v.Leader.Width > 0 && v.Width > 0 {
				oc.gotoTextMode(ScopePage)
				absX := x + sumX
				endX := absX + v.Width
				pw := v.Leader.Width

				// Global grid: first full copy starts at the next
				// multiple of pw at or after absX.
				firstStart := (absX / pw) * pw
				if firstStart < absX {
					firstStart += pw
				}

				for copyX := firstStart; copyX+pw <= endX; copyX += pw {
					oc.outputHorizontalItems(copyX, y, v.Leader)
					oc.gotoTextMode(ScopePage)
				}
				sumX += v.Width
			} else {
				if oc.textmode < ScopeText {
					if curFont := oc.currentFont; curFont != nil {
						if oc.currentFont.Size != 0 {
							// Emit a space glyph so that PDF readers can
							// extract proper word boundaries.
							spaceGID := curFont.Face.Codepoint(' ')
							if spaceGID != 0 {
								curFont.Face.RegisterCodepoint(spaceGID)
								oc.gotoTextMode(ScopeGlyph)
								oc.writef("%04x", spaceGID)
							}
							adv := v.Width.ToPT() / oc.currentFont.Size.ToPT()
							scale := curFont.Face.Scale
							// Subtract space glyph advance from the move
							var spaceAdv float64
							if spaceGID != 0 {
								spaceAdv = curFont.Face.AdvanceWidth(spaceGID)
							}
							move := int(-1 * 1000 / scale * (adv - spaceAdv))
							if move != 0 {
								oc.gotoTextMode(ScopeArray)
								oc.writef(" %d ", move)
							}
						}
					}
				}
				sumX += bag.MultiplyFloat(v.Width, float64(100+oc.currentExpand)/100.0)
			}
		case *node.Rule:
			if oc.p.document.DumpOutput {
				od = &outputDebug{
					Name: "rule",
					Attributes: map[string]any{
						"width":  v.Width,
						"height": v.Height,
						"depth":  v.Depth,
					},
				}
				if origin, ok := v.Attributes["origin"]; ok {
					od.Attributes["origin"] = origin
				}
				oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
			}
			oc.gotoTextMode(ScopePage)
			hasVisibleOutput := v.Pre != "" || v.Post != "" || !v.Hide
			hRuleNeedsArtifact := oc.p.document.Format == FormatPDFUA && oc.tag == nil && !oc.inArtifact && hasVisibleOutput
			if hRuleNeedsArtifact {
				oc.writef("/Artifact BMC\n")
			}
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
			if v.Attributes != nil {
				if faces, ok := v.Attributes["usedFaces"]; ok {
					if faceList, ok := faces.([]*pdf.Face); ok {
						for _, face := range faceList {
							oc.usedFaces[face] = true
						}
					}
				}
			}
			pdfinstructions = append(pdfinstructions, v.Post)
			sumX += v.Width
			pdfinstructions = append(pdfinstructions, fmt.Sprintf("1 0 0 1 %s %s cm\n", -posX, -posY))
			oc.write(strings.Join(pdfinstructions, " "))
			if hRuleNeedsArtifact {
				oc.writef("EMC\n")
			}
		case *node.Image:
			oc.gotoTextMode(ScopePage)
			if v.Used {
				bag.Logger.Warn(fmt.Sprintf("image node already in use, id: %d", hlist.ID))
			} else {
				v.Used = true
			}
			ifile := v.ImageFile
			oc.usedImages[ifile] = true
			scaleX := v.Width.ToPT() / ifile.ScaleX
			scaleY := v.Height.ToPT() / ifile.ScaleY
			posy := y
			posx := x + sumX
			oc.writef("q %f 0 0 %f %s %s cm %s Do Q\n", scaleX, scaleY, posx, posy, v.ImageFile.InternalName())
			sumX += v.Width
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
			switch action {
			case node.ActionHyperlink:
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
						a.Action = fmt.Sprintf("<</Type/Action/S/GoTo/D %s>>", pdf.Serialize(pdf.String(hyperlink.Local)))
					} else if hyperlink.URI != "" {
						a.Action = fmt.Sprintf("<</Type/Action/S/URI/URI %s>>", pdf.Serialize(hyperlink.URI))
					}
					// PDF/UA: link annotations need Contents and StructParent
					if oc.p.document.Format == FormatPDFUA {
						contents := hyperlink.URI
						if contents == "" {
							contents = hyperlink.Local
						}
						a.Dictionary["Contents"] = pdf.Serialize(pdf.String(contents))
						// Reserve an object number for the annotation
						a.Objectnumber = oc.p.document.PDFWriter.NextObject()
						// Create a Link SE and register the OBJR
						linkSE := &StructureElement{Role: "Link"}
						if oc.tag != nil {
							oc.tag.AddChild(linkSE)
						} else if oc.p.document.RootStructureElement != nil {
							oc.p.document.RootStructureElement.AddChild(linkSE)
						}
						sp := oc.p.document.newPDFStructureObject()
						sp.pageObjnum = oc.pageObjectnumber
						sp.pageIndex = oc.p.pageIndex
						sp.annotSE = linkSE
						oc.p.document.pdfStructureObjects = append(oc.p.document.pdfStructureObjects, sp)
						a.Dictionary["StructParent"] = fmt.Sprintf("%d", sp.id)
						linkSE.objRefs = append(linkSE.objRefs, objRefEntry{
							pageIndex:    oc.p.pageIndex,
							annotObjNum:  a.Objectnumber,
							structParent: sp.id,
						})
					}

					oc.p.Annotations = append(oc.p.Annotations, a)
					if oc.p.document.IsTrace(VTraceHyperlinks) {
						oc.gotoTextMode(ScopeText)
						oc.writef("q 0.4 w %s %s %s %s re S Q ", hyperlink.startposX, hyperlink.startposY, rectWD, rectHT)
					}
				}
			case node.ActionDest:
				// dest should be in the top left corner of the current position
				y := posY + hlist.Height + hlist.Depth
				var destname string
				switch t := v.Value.(type) {
				case string:
					d := &pdf.NameDest{
						Name:             pdf.String(t),
						X:                posX.ToPT(),
						Y:                y.ToPT(),
						PageObjectnumber: oc.pageObjectnumber,
					}
					destname = t
					oc.p.document.PDFWriter.NameDestinations[d.Name] = d
				case int:
					if oc.p.document.numDestinations == nil {
						oc.p.document.numDestinations = make(map[int]NumDest)
					}
					oc.p.document.numDestinations[t] = NumDest{
						PageObjectnumber: oc.pageObjectnumber,
						Num:              t,
						X:                posX.ToPT(),
						Y:                y.ToPT(),
					}
					destname = fmt.Sprintf("numdest-%d", t)
				}

				if oc.p.document.IsTrace(VTraceDest) {
					oc.gotoTextMode(ScopePage)
					black := color.Color{Space: color.ColorGray, R: 0, G: 0, B: 0, A: 1}
					circ := pdfdraw.New().ColorStroking(black).Circle(0, 0, 2*bag.Factor, 2*bag.Factor).Fill().String()
					oc.writef(" 1 0 0 1 %s %s cm ", posX, y)
					oc.write(circ)
					oc.writef(" 1 0 0 1 %s %s cm ", -posX, -y)
					oc.debugAt(posX, y, destname)
				}
			case node.ActionNone, node.ActionUserSetting:
				// ignore
			default:
				bag.Logger.Warn("start/stop node: unhandled action", "action", action)
			}

			switch v.Position {
			case node.PDFOutputPage:
				oc.gotoTextMode(ScopePage)
			case node.PDFOutputDirect:
				oc.gotoTextMode(ScopeText)
			case node.PDFOutputHere:
				oc.gotoTextMode(ScopePage)
				oc.writef(" 1 0 0 1 %s %s cm ", posX, posY)
			case node.PDFOutputLowerLeft:
				oc.gotoTextMode(ScopePage)
			}
			if v.ShipoutCallback != nil {
				oc.write(v.ShipoutCallback(v))
			}
			if v.Position == node.PDFOutputHere {
				oc.moveto(-posX, -posY)
			}
		case *node.Kern:
			if oc.p.document.DumpOutput {
				od = &outputDebug{
					Name: "kern",
					Attributes: map[string]any{
						"kern": v.Kern,
					},
				}
				if origin, ok := v.Attributes["origin"]; ok {
					od.Attributes["origin"] = origin
				}
				oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
			}
			if oc.textmode > ScopeArray {
				oc.moveto(x+oc.shiftX+sumX, (y))
				oc.shiftX = 0
			}

			if oc.currentFont != nil {
				y := v.Kern.ToPT() / oc.currentFont.Size.ToPT()
				if kern := int(math.Round(-1000 * y)); kern != 0 {
					oc.gotoTextMode(ScopeArray)
					oc.writef(" %d ", kern)
				}
			}
			sumX += v.Kern
		case *node.Lang, *node.Penalty:
			// ignore
		case *node.Disc:
			// ignore
		case *node.HList:
			moveY := y
			if hlist.VAlign == node.VAlignTop {
				moveY -= v.Height
			}
			oc.outputHorizontalItems(x+sumX, moveY, v)
			sumX += v.Width
		case *node.VList:
			oc.gotoTextMode(ScopePage)
			saveX := x
			saveY := y
			moveY := y + hlist.Height
			if hlist.VAlign == node.VAlignTop {
				moveY = y
			}
			x += sumX

			// PDF/UA: check for tag/artifact on child VLists
			var childTag *StructureElement
			var childArtifact bool
			var childArtifactType ArtifactType
			if v.Attributes != nil {
				if r, ok := v.Attributes["tag"]; ok {
					childTag = r.(*StructureElement)
				}
				if a, ok := v.Attributes["artifact"]; ok {
					childArtifactType = a.(ArtifactType)
					childArtifact = true
				}
			}
			if oc.p.document.Format == FormatPDFUA && childTag == nil && !childArtifact && oc.tag == nil && !oc.inArtifact {
				if !vlistHasTaggedDescendant(v) {
					childArtifact = true
				}
			}

			if childArtifact {
				if childArtifactType != "" {
					oc.writef("/Artifact <</Type /%s>> BDC\n", childArtifactType)
				} else {
					oc.writef("/Artifact BMC\n")
				}
			} else if childTag != nil {
				mcid := oc.p.nextMCID
				oc.p.nextMCID++
				childTag.mcids = append(childTag.mcids, mcidEntry{pageIndex: oc.p.pageIndex, mcid: mcid})
				childTag.ID = mcid
				oc.p.StructureElements = append(oc.p.StructureElements, childTag)
				oc.emitBDC(childTag, mcid)
				// Set BBox for Figure elements (PDF/UA requirement)
				if childTag.Role == "Figure" {
					pageHeightPT := (oc.p.Height + 2*oc.p.ExtraOffset).ToPT()
					llx := (x + v.ShiftX).ToPT()
					lly := pageHeightPT - moveY.ToPT()
					urx := llx + v.Width.ToPT()
					ury := lly + (v.Height + v.Depth).ToPT()
					childTag.BBox = [4]float64{llx, lly, urx, ury}
					childTag.HasBBox = true
				}
			}

			savedTag := oc.tag
			savedInArtifact := oc.inArtifact
			if childTag != nil {
				oc.tag = childTag
			}
			if childArtifact {
				oc.inArtifact = true
			}

			oc.outputVerticalItems(x+v.ShiftX, moveY, v)

			if childTag != nil || childArtifact {
				oc.gotoTextMode(ScopePage)
				oc.writef("EMC\n")
			}
			oc.tag = savedTag
			oc.inArtifact = savedInArtifact

			sumX += v.Width
			x = saveX
			y = saveY
		default:
			bag.Logger.Warn(fmt.Sprintf("Shipout: unknown node %v", hItem))
		}
	}
	if oc.p.document.DumpOutput {
		oc.curOutputDebug = saveCurOutputDebug
	}
}

// outputVerticalItems iterates through the vlist's list and outputs each item
// beneath each other.
func (oc *objectContext) outputVerticalItems(x, y bag.ScaledPoint, vlist *node.VList) {
	var od, saveCurOutputDebug *outputDebug
	if oc.p.document.DumpOutput {
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
		saveCurOutputDebug = oc.curOutputDebug
		oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
		oc.curOutputDebug = od
	}
	// PDF/UA: track whether we're inside an untagged container
	// (no tag, no artifact set). Content in such containers that
	// is not a tagged child VList must be wrapped as Artifact.
	untaggedContainer := oc.p.document.Format == FormatPDFUA && oc.tag == nil && !oc.inArtifact

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
				p := pdfdraw.NewStandalone().LineWidth(bag.MustSP("0.4pt")).Rect(0, -v.Depth, v.Width, v.Height+v.Depth).Stroke()
				r.Pre = p.String()
				v.List = node.InsertBefore(v.List, v.List, r)
			}
			// PDF/UA: HList inside untagged container needs Artifact wrapping
			// (unless it contains tagged descendants)
			needsArtifact := untaggedContainer && !hlistHasTaggedDescendant(v)
			if needsArtifact {
				oc.gotoTextMode(ScopePage)
				oc.writef("/Artifact BMC\n")
			}
			oc.outputHorizontalItems(x, shiftDown, v)
			if needsArtifact {
				oc.gotoTextMode(ScopePage)
				oc.writef("EMC\n")
			}
			sumY += v.Height
			sumY += v.Depth
			if oc.textmode < ScopeText {
				oc.gotoTextMode(ScopeText)
			}
		case *node.Image:
			if v.Used {
				bag.Logger.Warn(fmt.Sprintf("image node already in use, id: %d", v.ID))
			} else {
				v.Used = true
			}
			ifile := v.ImageFile
			oc.usedImages[ifile] = true
			oc.gotoTextMode(ScopePage)

			scaleX := v.Width.ToPT() / ifile.ScaleX
			scaleY := v.Height.ToPT() / ifile.ScaleY

			ht := v.Height
			posy := y - ht
			posx := x
			if oc.p.document.IsTrace(VTraceImages) {
				oc.writef("q 0.2 w %s %s %s %s re S Q\n", x, y, v.Width, v.Height)
			}
			oc.writef("q %f 0 0 %f %s %s cm %s Do Q\n", scaleX, scaleY, posx, posy, v.ImageFile.InternalName())
		case *node.Glue:
			if oc.p.document.DumpOutput {
				if oc.p.document.DumpOutput {
					od = &outputDebug{
						Name: "glue",
						Attributes: map[string]any{
							"width":   v.Width,
							"stretch": v.Stretch,
							"shrink":  v.Shrink,
						},
					}
					oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
				}
			}
			// Let's assume that the glue ratio has been determined and the
			// natural width is in v.Width for now.
			sumY += v.Width
		case *node.Kern:
			if oc.p.document.DumpOutput {
				if oc.p.document.DumpOutput {
					od = &outputDebug{
						Name: "kern",
						Attributes: map[string]any{
							"kern": v.Kern,
						},
					}
					oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
				}
			}
			sumY += v.Kern
		case *node.Rule:
			if oc.p.document.DumpOutput {
				if oc.p.document.DumpOutput {
					od = &outputDebug{
						Name: "rule",
						Attributes: map[string]any{
							"width":  v.Width,
							"height": v.Height,
							"depth":  v.Depth,
						},
					}
					if origin, ok := v.Attributes["origin"]; ok {
						od.Attributes["origin"] = origin
					}
					oc.curOutputDebug.Items = append(oc.curOutputDebug.Items, od)
				}
			}
			posX := x
			posY := y - sumY
			sumY += v.Height + v.Depth
			hasVisibleOutput := v.Pre != "" || v.Post != "" || !v.Hide
			ruleNeedsArtifact := untaggedContainer && hasVisibleOutput
			oc.gotoTextMode(ScopePage)
			if ruleNeedsArtifact {
				oc.writef("/Artifact BMC\n")
			}
			pdfinstructions := []string{fmt.Sprintf("1 0 0 1 %s %s cm", posX, posY)}
			if v.Pre != "" {
				pdfinstructions = append(pdfinstructions, v.Pre)
			}
			if !v.Hide {
				pdfinstructions = append(pdfinstructions, fmt.Sprintf("q 0 0 %s %s re f Q", v.Width, -1*(v.Height+v.Depth)))
			}
			if v.Attributes != nil {
				if faces, ok := v.Attributes["usedFaces"]; ok {
					if faceList, ok := faces.([]*pdf.Face); ok {
						for _, face := range faceList {
							oc.usedFaces[face] = true
						}
					}
				}
			}
			if v.Post != "" {
				pdfinstructions = append(pdfinstructions, v.Post)
			}
			pdfinstructions = append(pdfinstructions, fmt.Sprintf("1 0 0 1 %s %s cm\n", -posX, -posY))
			oc.write(strings.Join(pdfinstructions, " "))
			if ruleNeedsArtifact {
				oc.writef("EMC\n")
			}
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

			switch action {
			case node.ActionHyperlink:
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
						a.Action = fmt.Sprintf("<</Type/Action/S/GoTo/D %s>>", pdf.Serialize(pdf.String(hyperlink.Local)))
					} else if hyperlink.URI != "" {
						a.Action = fmt.Sprintf("<</Type/Action/S/URI/URI %s>>", pdf.Serialize(hyperlink.URI))
					}
					// PDF/UA: link annotations need Contents and StructParent
					if oc.p.document.Format == FormatPDFUA {
						contents := hyperlink.URI
						if contents == "" {
							contents = hyperlink.Local
						}
						a.Dictionary["Contents"] = pdf.Serialize(pdf.String(contents))
						// Reserve an object number for the annotation
						a.Objectnumber = oc.p.document.PDFWriter.NextObject()
						// Create a Link SE and register the OBJR
						linkSE := &StructureElement{Role: "Link"}
						if oc.tag != nil {
							oc.tag.AddChild(linkSE)
						} else if oc.p.document.RootStructureElement != nil {
							oc.p.document.RootStructureElement.AddChild(linkSE)
						}
						sp := oc.p.document.newPDFStructureObject()
						sp.pageObjnum = oc.pageObjectnumber
						sp.pageIndex = oc.p.pageIndex
						sp.annotSE = linkSE
						oc.p.document.pdfStructureObjects = append(oc.p.document.pdfStructureObjects, sp)
						a.Dictionary["StructParent"] = fmt.Sprintf("%d", sp.id)
						linkSE.objRefs = append(linkSE.objRefs, objRefEntry{
							pageIndex:    oc.p.pageIndex,
							annotObjNum:  a.Objectnumber,
							structParent: sp.id,
						})
					}

					oc.p.Annotations = append(oc.p.Annotations, a)
					if oc.p.document.IsTrace(VTraceHyperlinks) {
						oc.gotoTextMode(ScopeText)
						oc.writef("q 0.4 w %s %s %s %s re S Q ", hyperlink.startposX, hyperlink.startposY, rectWD, rectHT)
					}
				}
			case node.ActionDest:
				// dest should be in the top left corner of the current position
				y := posY + vlist.Height + vlist.Depth
				var destname string // for debugging only
				switch t := v.Value.(type) {
				case string:
					d := &pdf.NameDest{
						Name:             pdf.String(t),
						X:                posX.ToPT(),
						Y:                y.ToPT(),
						PageObjectnumber: oc.pageObjectnumber,
					}
					destname = t
					oc.p.document.PDFWriter.NameDestinations[d.Name] = d
				case int:
					if oc.p.document.numDestinations == nil {
						oc.p.document.numDestinations = make(map[int]NumDest)
					}
					oc.p.document.numDestinations[t] = NumDest{
						PageObjectnumber: oc.pageObjectnumber,
						Num:              t,
						X:                posX.ToPT(),
						Y:                y.ToPT(),
					}
					destname = fmt.Sprintf("numdest-%d", t)
				}

				if oc.p.document.IsTrace(VTraceDest) {
					oc.gotoTextMode(ScopePage)
					black := color.Color{Space: color.ColorGray, R: 0, G: 0, B: 0, A: 1}
					circ := pdfdraw.New().ColorStroking(black).Circle(0, 0, 2*bag.Factor, 2*bag.Factor).Fill().String()
					oc.writef(" 1 0 0 1 %s %s cm ", posX, y)
					oc.write(circ)
					oc.writef(" 1 0 0 1 %s %s cm ", -posX, -y)
					oc.debugAt(posX, y, destname)
				}
			case node.ActionNone:
				// ignore
			default:
				bag.Logger.Warn("start/stop node: unhandled action", "action", action)
			}
			switch v.Position {
			case node.PDFOutputPage:
				oc.gotoTextMode(ScopePage)
			case node.PDFOutputDirect:
				oc.gotoTextMode(ScopeText)
			case node.PDFOutputHere:
				oc.gotoTextMode(ScopePage)
				oc.writef(" 1 0 0 1 %s %s cm ", posX, posY)
			case node.PDFOutputLowerLeft:
				oc.gotoTextMode(ScopePage)
			}
			if v.ShipoutCallback != nil {
				oc.write(v.ShipoutCallback(v))
			}
			if v.Position == node.PDFOutputHere {
				oc.moveto(-posX, -posY)
			}
		case *node.VList:
			// PDF/UA: check for tag/artifact on child VLists
			var childTag *StructureElement
			var childArtifact bool
			var childArtifactType ArtifactType
			if v.Attributes != nil {
				if r, ok := v.Attributes["tag"]; ok {
					childTag = r.(*StructureElement)
				}
				if a, ok := v.Attributes["artifact"]; ok {
					childArtifactType = a.(ArtifactType)
					childArtifact = true
				}
			}
			// If this child VList has no tag and no artifact and
			// parent also has no tag (i.e. we're in an untagged
			// container), mark as artifact in PDF/UA mode — but only
			// if there are no tagged descendants (otherwise let them
			// emit their own BDC/EMC).
			if oc.p.document.Format == FormatPDFUA && childTag == nil && !childArtifact && oc.tag == nil && !oc.inArtifact {
				if !vlistHasTaggedDescendant(v) {
					childArtifact = true
				}
			}

			if childArtifact {
				oc.gotoTextMode(ScopePage)
				if childArtifactType != "" {
					oc.writef("/Artifact <</Type /%s>> BDC\n", childArtifactType)
				} else {
					oc.writef("/Artifact BMC\n")
				}
			} else if childTag != nil {
				oc.gotoTextMode(ScopePage)
				// Save and swap the current tag
				mcid := oc.p.nextMCID
				oc.p.nextMCID++
				childTag.mcids = append(childTag.mcids, mcidEntry{pageIndex: oc.p.pageIndex, mcid: mcid})
				childTag.ID = mcid
				oc.p.StructureElements = append(oc.p.StructureElements, childTag)
				oc.emitBDC(childTag, mcid)
				// Set BBox for Figure elements (PDF/UA requirement)
				if childTag.Role == "Figure" {
					pageHeightPT := (oc.p.Height + 2*oc.p.ExtraOffset).ToPT()
					posX := (x + v.ShiftX).ToPT()
					posY := (y - sumY).ToPT()
					childTag.BBox = [4]float64{
						posX,
						pageHeightPT - posY,
						posX + v.Width.ToPT(),
						pageHeightPT - posY + (v.Height + v.Depth).ToPT(),
					}
					childTag.HasBBox = true
				}
			}

			savedTag := oc.tag
			savedInArtifact := oc.inArtifact
			if childTag != nil {
				oc.tag = childTag
			}
			if childArtifact {
				oc.inArtifact = true
			}

			oc.outputVerticalItems(x+v.ShiftX, y-sumY, v)

			if childTag != nil || childArtifact {
				oc.gotoTextMode(ScopePage)
				oc.writef("EMC\n")
			}
			oc.tag = savedTag
			oc.inArtifact = savedInArtifact

			sumY += v.Height + v.Depth
		default:
			bag.Logger.Error(fmt.Sprintf("Shipout: unknown node %T in vertical mode", v))
		}
	}
	if oc.p.document.DumpOutput {
		oc.curOutputDebug = saveCurOutputDebug
	}
}

func (oc objectContext) debugAt(x, y bag.ScaledPoint, text string) {
	if len(oc.p.document.Faces) == 0 {
		return
	}
	oc.gotoTextMode(ScopeText)
	f0 := oc.p.document.Faces[0]
	oc.writef(" q %s 8 Tf 0 0 1 rg ", f0.InternalName())
	oc.moveto(x, y)
	oc.writef("[<")
	fnt := font.NewFont(f0, 4*bag.Factor)
	cp := []int{}
	for _, v := range fnt.Shape(text, nil, nil) {
		oc.writef("%04x", v.Codepoint)
		cp = append(cp, v.Codepoint)
	}
	f0.RegisterCodepoints(cp)
	oc.writef(">]TJ Q ")
	oc.gotoTextMode(ScopePage)
}

// OutputAt places the nodelist at the position.
func (p *Page) OutputAt(x bag.ScaledPoint, y bag.ScaledPoint, vlist *node.VList) {
	p.Objects = append(p.Objects, Object{X: x, Y: y, Vlist: vlist})
}

// Shipout places all objects on a page and finishes this page.
func (p *Page) Shipout() {
	bag.Logger.Debug("Shipout")
	if p.Finished {
		return
	}
	p.Finished = true

	pageObjectNumber := p.document.PDFWriter.NextObject()
	p.Objectnumber = pageObjectNumber
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
		rule := node.NewRule()
		rule.Pre = s.String()
		rule.Hide = true
		vl := node.Vpack(rule)
		p.Background = append(p.Background, Object{X: -offsetX, Y: -offsetY, Vlist: vl})
	}

	// In PDF/UA mode, mark background objects as artifacts automatically.
	if p.document.Format == FormatPDFUA {
		for i := range p.Background {
			vl := p.Background[i].Vlist
			if vl.Attributes == nil {
				vl.Attributes = node.H{}
			}
			if _, hasTag := vl.Attributes["tag"]; !hasTag {
				if _, hasArt := vl.Attributes["artifact"]; !hasArt {
					vl.Attributes["artifact"] = ArtifactBackground
				}
			}
		}
	}
	objs := make([]Object, 0, len(p.Background)+len(p.Objects))
	objs = append(objs, p.Background...)
	objs = append(objs, p.Objects...)
	usedFaces := make(map[*pdf.Face]bool)
	usedImages := make(map[*pdf.Imagefile]bool)
	if p.document.DumpOutput {
		p.outputDebug = &outputDebug{
			Name: "page",
		}
	}
	st := p.document.PDFWriter.NewObject()
	st.SetCompression(p.document.CompressLevel)

	for _, obj := range objs {
		oc := &objectContext{
			textmode:         ScopePage,
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
			}
			if a, ok := vlist.Attributes["artifact"]; ok {
				oc.artifactType = a.(ArtifactType)
				oc.inArtifact = true
			}
		}
		// PDF/UA: untagged top-level VLists without tagged descendants
		// default to Artifact. VLists with tagged descendants are left
		// untagged so children can emit their own BDC/EMC.
		if p.document.Format == FormatPDFUA && oc.tag == nil && !oc.inArtifact {
			if !vlistHasTaggedDescendant(vlist) {
				oc.inArtifact = true
			}
		}

		x := obj.X + offsetX
		y := obj.Y + offsetY

		// For top-level objects with a direct tag or artifact, wrap
		// the entire output in a single BDC/EMC.
		// For container VLists without a tag (e.g. from htmlbag),
		// let outputVerticalItems handle per-child tagging.
		if oc.inArtifact {
			if oc.artifactType != "" {
				oc.writef("/Artifact <</Type /%s>> BDC\n", oc.artifactType)
			} else {
				oc.writef("/Artifact BMC\n")
			}
		} else if oc.tag != nil {
			mcid := oc.p.nextMCID
			oc.p.nextMCID++
			oc.tag.mcids = append(oc.tag.mcids, mcidEntry{pageIndex: oc.p.pageIndex, mcid: mcid})
			oc.tag.ID = mcid
			oc.p.StructureElements = append(oc.p.StructureElements, oc.tag)
			oc.emitBDC(oc.tag, mcid)
			// Set BBox for Figure elements (PDF/UA requirement)
			if oc.tag.Role == "Figure" {
				pageHeightPT := (oc.p.Height + 2*oc.p.ExtraOffset).ToPT()
				llx := x.ToPT()
				lly := pageHeightPT - y.ToPT()
				urx := llx + vlist.Width.ToPT()
				ury := lly + (vlist.Height + vlist.Depth).ToPT()
				oc.tag.BBox = [4]float64{llx, lly, urx, ury}
				oc.tag.HasBBox = true
			}
		}

		oc.outputVerticalItems(x, y, vlist)
		for k := range oc.usedFaces {
			usedFaces[k] = true
		}
		for k := range oc.usedImages {
			usedImages[k] = true
		}
		oc.gotoTextMode(ScopePage)

		// Close the object-level BDC
		if oc.tag != nil || oc.inArtifact {
			oc.newline()
			oc.writef("EMC\n")
		}
		if oc.p.document.DumpOutput {
			p.outputDebug.Items = append(p.outputDebug.Items, oc.outputDebug)
		}
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

	// annotations are hyperlinks and structure elements
	page.Annotations = p.Annotations

	// PDF/UA: pages with annotations must have /Tabs /S
	if p.document.Format == FormatPDFUA && len(p.Annotations) > 0 {
		page.Dict["Tabs"] = "/S"
	}

	if p.document.RootStructureElement != nil && len(p.StructureElements) > 0 {
		// Record StructParents index for this page. The actual SE objects
		// are serialized later in Finish() via serializeStructureElement.
		po := p.document.newPDFStructureObject()
		po.pageObjnum = page.Objnum
		po.pageIndex = p.pageIndex
		page.Dict["StructParents"] = fmt.Sprintf("%d", po.id)
		p.document.pdfStructureObjects = append(p.document.pdfStructureObjects, po)
	}
	for _, s := range p.document.Spotcolors {
		p.Spotcolors = append(p.Spotcolors, s)
	}
}

// CallbackShipout gets called before the shipout process starts.
type CallbackShipout func(page *Page)

// vlistHasTaggedDescendant checks if a VList or any of its child VLists
// have a "tag" attribute. Used to decide whether an untagged top-level
// container should be auto-marked as Artifact or left untagged for its
// children to emit their own BDC/EMC.
func vlistHasTaggedDescendant(vl *node.VList) bool {
	for cur := vl.List; cur != nil; cur = cur.Next() {
		switch n := cur.(type) {
		case *node.VList:
			if n.Attributes != nil {
				if _, ok := n.Attributes["tag"]; ok {
					return true
				}
			}
			if vlistHasTaggedDescendant(n) {
				return true
			}
		case *node.HList:
			if hlistHasTaggedDescendant(n) {
				return true
			}
		}
	}
	return false
}

func hlistHasTaggedDescendant(hl *node.HList) bool {
	for cur := hl.List; cur != nil; cur = cur.Next() {
		switch n := cur.(type) {
		case *node.VList:
			if n.Attributes != nil {
				if _, ok := n.Attributes["tag"]; ok {
					return true
				}
			}
			if vlistHasTaggedDescendant(n) {
				return true
			}
		}
	}
	return false
}

// ArtifactType represents the type of a PDF artifact.
type ArtifactType string

const (
	// ArtifactPagination marks page numbers, headers, footers.
	ArtifactPagination ArtifactType = "Pagination"
	// ArtifactLayout marks layout aids such as rules and backgrounds.
	ArtifactLayout ArtifactType = "Layout"
	// ArtifactPage marks page-level decorations.
	ArtifactPage ArtifactType = "Page"
	// ArtifactBackground marks background content.
	ArtifactBackground ArtifactType = "Background"
)

// mcidEntry records a marked-content identifier on a specific page.
type mcidEntry struct {
	pageIndex int
	mcid      int
}

// objRefEntry records an object reference (OBJR) for annotations.
type objRefEntry struct {
	pageIndex    int
	annotObjNum  pdf.Objectnumber
	structParent int // StructParents index for the annotation
}

// StructureElement represents a tagged PDF element such as H1 or P.
type StructureElement struct {
	Parent     *StructureElement
	Obj        *pdf.Object
	Role       string
	ActualText string
	Alt        string     // alternative text (for Figure, Formula etc.)
	Lang       string     // BCP 47 language tag for this element
	Scope      string     // for TH: "Row", "Column", "Both"
	BBox       [4]float64 // bounding box [x, y, width, height] in PDF points; set during shipout
	HasBBox    bool
	children   []*StructureElement
	mcids      []mcidEntry
	objRefs    []objRefEntry
	ID         int
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
	pageObjnum pdf.Objectnumber
	annotSE    *StructureElement // non-nil for annotation StructParent entries
	id         int
	pageIndex  int
}

func (d *PDFDocument) newPDFStructureObject() *pdfStructureObject {
	po := &pdfStructureObject{}
	po.id = len(d.pdfStructureObjects)
	return po
}

// serializeStructureElement recursively creates PDF objects for the structure
// tree. parentObjnum is the object number of the parent (either StructTreeRoot
// or a parent StructElem). pageObjnums maps page indices to page object numbers.
func (d *PDFDocument) serializeStructureElement(se *StructureElement, parentObjnum pdf.Objectnumber, pageObjnums map[int]pdf.Objectnumber) {
	if se.Obj == nil {
		se.Obj = d.PDFWriter.NewObject()
	}
	se.Obj.Dictionary = pdf.Dict{
		"Type": "/StructElem",
		"S":    "/" + se.Role,
		"P":    parentObjnum.Ref(),
	}
	if se.ActualText != "" {
		se.Obj.Dictionary["ActualText"] = pdf.Serialize(pdf.String(se.ActualText))
	}
	if se.Alt != "" {
		se.Obj.Dictionary["Alt"] = pdf.Serialize(pdf.String(se.Alt))
	}
	if se.Lang != "" {
		se.Obj.Dictionary["Lang"] = fmt.Sprintf("(%s)", se.Lang)
	}
	if se.Scope != "" {
		se.Obj.Dictionary["A"] = fmt.Sprintf("<< /O /Table /Scope /%s >>", se.Scope)
	}
	if se.HasBBox {
		se.Obj.Dictionary["A"] = fmt.Sprintf("<< /O /Layout /BBox [ %s %s %s %s ] >>",
			pdf.FloatToPoint(se.BBox[0]), pdf.FloatToPoint(se.BBox[1]),
			pdf.FloatToPoint(se.BBox[2]), pdf.FloatToPoint(se.BBox[3]))
	}

	// Build /K array: MCR dicts for marked content, OBJR dicts for
	// annotations, and child SE refs
	var kItems []string

	// Add marked content references
	for _, m := range se.mcids {
		pgRef := pageObjnums[m.pageIndex].Ref()
		kItems = append(kItems, fmt.Sprintf("<< /Type /MCR /Pg %s /MCID %d >>", pgRef, m.mcid))
	}

	// Add object references (for link annotations etc.)
	for _, o := range se.objRefs {
		pgRef := pageObjnums[o.pageIndex].Ref()
		kItems = append(kItems, fmt.Sprintf("<< /Type /OBJR /Obj %s /Pg %s >>", o.annotObjNum.Ref(), pgRef))
	}

	// Recursively serialize children
	for _, child := range se.children {
		d.serializeStructureElement(child, se.Obj.ObjectNumber, pageObjnums)
		kItems = append(kItems, child.Obj.ObjectNumber.Ref())
	}

	switch len(kItems) {
	case 0:
		// no /K entry needed
	case 1:
		se.Obj.Dictionary["K"] = kItems[0]
	default:
		se.Obj.Dictionary["K"] = fmt.Sprintf("[ %s ]", strings.Join(kItems, " "))
	}

	se.Obj.Save()
}

// PDFDocument contains all references to a document
type PDFDocument struct {
	CreationDate         time.Time
	ColorProfile         *ColorProfile
	CurrentPage          *Page
	DefaultLanguage      *lang.Lang
	Languages            map[string]*lang.Lang
	PDFWriter            *pdf.PDF
	RoleMap              map[string]string // maps custom roles to standard PDF roles
	RootStructureElement *StructureElement
	ViewerPreferences    map[string]string
	outputDebug          *outputDebug
	curOutputDebug       *outputDebug
	usedPDFImages        map[string]*pdf.Imagefile
	numDestinations      map[int]NumDest
	DefaultLanguageTag   string // BCP 47 language tag for the document catalog (e.g. "de", "en-US")
	Author               string
	Creator              string
	Filename             string
	Keywords             string
	Subject              string
	Title                string
	producer             string
	Attachments          []Attachment
	Faces                []*pdf.Face
	Pages                []*Page
	Spotcolors           []*color.Color
	pdfStructureObjects  []*pdfStructureObject
	preShipoutCallback   []CallbackShipout
	Bleed                bag.ScaledPoint
	CompressLevel        uint
	DefaultPageHeight    bag.ScaledPoint
	DefaultPageWidth     bag.ScaledPoint
	Format               Format // The PDF format (PDF/X-1, PDF/X-3, PDF/A, etc.)
	tracing              VTrace
	DumpOutput           bool
	ShowCutmarks         bool
	ShowHyperlinks       bool
	SuppressInfo         bool
	xmpExtensions        []XMPExtension
}

// NewDocument creates an empty document.
func NewDocument(w io.Writer) *PDFDocument {
	d := &PDFDocument{
		DefaultPageWidth:  bag.MustSP("210mm"),
		DefaultPageHeight: bag.MustSP("297mm"),
		Creator:           "boxesandglue.dev",
		CreationDate:      time.Now(),
		Languages:         make(map[string]*lang.Lang),
		ViewerPreferences: make(map[string]string),
		PDFWriter:         pdf.NewPDFWriter(w),
		CompressLevel:     9,
		producer:          "boxesandglue.dev",
		usedPDFImages:     make(map[string]*pdf.Imagefile),
		outputDebug: &outputDebug{
			Name: "pdfdocument",
		},
	}
	d.curOutputDebug = d.outputDebug
	pdf.Logger = bag.Logger
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

// LoadFaceFromData creates a Face from a byte stream.
func (d *PDFDocument) LoadFaceFromData(data []byte, index int) (*pdf.Face, error) {
	f, err := d.PDFWriter.NewFaceFromData(data, index)
	if err != nil {
		return nil, err
	}
	d.Faces = append(d.Faces, f)
	return f, nil
}

// LoadFace loads a font from a TrueType or OpenType collection.
func (d *PDFDocument) LoadFace(filename string, index int) (*pdf.Face, error) {
	// face already loaded? TODO: check index TODO: use PostscriptName instead
	// of file name, since the face can be loaded from data
	for _, fce := range d.Faces {
		if fce.Filename == filename {
			return fce, nil
		}
	}
	bag.Logger.Debug("LoadFace", "filename", filename)

	f, err := d.PDFWriter.LoadFace(filename, index)
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
	imgf, err := d.PDFWriter.LoadImageFileWithBox(filename, box, pagenumber)
	if err != nil {
		return nil, err
	}
	d.usedPDFImages[key] = imgf
	return imgf, nil
}

func GetDimensions(imgf *pdf.Imagefile, pagenumber int, box string) (width bag.ScaledPoint, height bag.ScaledPoint, err error) {
	switch imgf.Format {
	case "pdf":
		if box == "" {
			box = "/MediaBox"
		}
		mb, err := imgf.GetPDFBoxDimensions(pagenumber, box)
		if err != nil {
			return 0, 0, err
		}

		width = bag.ScaledPointFromFloat(mb["w"])
		height = bag.ScaledPointFromFloat(mb["h"])
	case "jpeg", "png":
		width = bag.ScaledPoint(imgf.W) * bag.Factor
		height = bag.ScaledPoint(imgf.H) * bag.Factor
	}
	return
}

// CreateImageNodeFromImagefile returns a new Image derived from the image file. The parameter
// pagenumber is honored only in PDF files. The Box is one of "/MediaBox", "/CropBox",
// "/TrimBox", "/BleedBox" or "/ArtBox"
func (d *PDFDocument) CreateImageNodeFromImagefile(imgfile *pdf.Imagefile, pagenumber int, box string) *node.Image {
	img := &node.Image{}
	img.ImageFile = imgfile
	wd, ht, err := GetDimensions(imgfile, pagenumber, box)
	if err != nil {
		return nil
	}
	img.Width = wd
	img.Height = ht
	imgfile.PageNumber = pagenumber
	return img
}

// CreateSVGNodeFromDocument renders a parsed SVG document into a Rule node
// containing PDF drawing operators. If width or height is 0, it is derived from
// the SVG's viewBox to preserve the aspect ratio. If both are 0, the SVG's
// natural size is used (1 SVG user unit = 1 DTP point).
// CreateSVGNodeFromDocument creates a Rule node from a parsed SVG document.
// The width and height specify the desired output size. If one is 0, it is
// calculated from the aspect ratio. If both are 0, the SVG's natural size is
// used. An optional TextRenderer enables SVG <text> rendering; if nil, text
// elements are skipped.
func (d *PDFDocument) CreateSVGNodeFromDocument(svgDoc *svgreader.Document, width bag.ScaledPoint, height bag.ScaledPoint, textRenderer ...svgreader.TextRenderer) *node.Rule {
	naturalW := svgDoc.Width
	naturalH := svgDoc.Height

	wPt := width.ToPT()
	hPt := height.ToPT()

	switch {
	case wPt == 0 && hPt == 0:
		// Use natural SVG dimensions
		wPt = naturalW
		hPt = naturalH
	case wPt == 0:
		// Scale width to match height, preserving aspect ratio
		if naturalH > 0 {
			wPt = hPt * naturalW / naturalH
		}
	case hPt == 0:
		// Scale height to match width, preserving aspect ratio
		if naturalW > 0 {
			hPt = wPt * naturalH / naturalW
		}
	}

	width = bag.ScaledPointFromFloat(wPt)
	height = bag.ScaledPointFromFloat(hPt)

	opts := svgreader.RenderOptions{
		Width:  wPt,
		Height: hPt,
	}
	if len(textRenderer) > 0 && textRenderer[0] != nil {
		opts.TextRenderer = textRenderer[0]
	}

	stream := svgDoc.RenderPDF(opts)

	rule := node.NewRule()
	rule.Width = width
	rule.Height = height
	rule.Hide = true
	rule.Pre = stream

	// If the TextRenderer tracks used faces, store them as attributes
	// so the page renderer can register them as PDF resources.
	if opts.TextRenderer != nil {
		type faceProvider interface {
			UsedFaces() []*pdf.Face
		}
		if fp, ok := opts.TextRenderer.(faceProvider); ok {
			if faces := fp.UsedFaces(); len(faces) > 0 {
				rule.Attributes = node.H{"usedFaces": faces}
			}
		}
	}

	return rule
}

// NewPage creates a new Page object and adds it to the page list in the
// document. The CurrentPage field of the document is set to the page.
func (d *PDFDocument) NewPage() *Page {
	d.CurrentPage = &Page{
		document:  d,
		Width:     d.DefaultPageWidth,
		Height:    d.DefaultPageHeight,
		pageIndex: len(d.Pages),
	}
	if d.ShowCutmarks {
		d.CurrentPage.ExtraOffset = cutmarkLength
	}
	d.CurrentPage.ExtraOffset += d.Bleed

	d.Pages = append(d.Pages, d.CurrentPage)
	return d.CurrentPage
}

func formatDate(t time.Time) string {
	// base format
	base := t.Format("20060102150405")

	// timezone offset
	_, offset := t.Zone()
	offsetHours := offset / 3600
	offsetMinutes := (offset % 3600) / 60

	sign := "+"
	if offset < 0 {
		sign = "-"
		offsetHours = -offsetHours
		offsetMinutes = -offsetMinutes
	}

	return fmt.Sprintf("(D:%s%s%02d'%02d')", base, sign, offsetHours, offsetMinutes)
}

// Finish writes all objects to the PDF and writes the XRef section. Finish does
// not close the writer.
func (d *PDFDocument) Finish() error {
	var err error
	d.PDFWriter.Catalog = pdf.Dict{}

	// Automatically create root structure element for PDF/UA if not set
	if d.Format == FormatPDFUA && d.RootStructureElement == nil {
		d.RootStructureElement = &StructureElement{Role: "Document"}
	}

	switch d.Format {
	case FormatPDFA3b, FormatPDFX3, FormatPDFX4:
		if d.ColorProfile == nil {
			d.ColorProfile, err = d.LoadDefaultColorprofile()
			if err != nil {
				return err
			}
		}
	}

	var cp *pdf.Object

	if d.ColorProfile != nil {
		cp = d.PDFWriter.NewObject()
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
					"C0":           pdf.Serialize(pdf.Array{0, 0, 0, 0}),
					"C1":           pdf.Serialize(pdf.Array{sep.C, sep.M, sep.Y, sep.K}),
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

	switch d.Format {
	case FormatPDFA3b, FormatPDFX3, FormatPDFX4:
		outputIntent := d.PDFWriter.NewObject()
		outputIntent.Dictionary = pdf.Dict{
			"DestOutputProfile":         cp.ObjectNumber.Ref(),
			"Info":                      pdf.Serialize(pdf.String(d.ColorProfile.Info)),
			"OutputCondition":           pdf.Serialize(pdf.String(d.ColorProfile.Condition)),
			"OutputConditionIdentifier": pdf.Serialize(pdf.String(d.ColorProfile.Identifier)),
			"RegistryName":              pdf.Serialize(pdf.String(d.ColorProfile.Registry)),
			"Type":                      pdf.Name("OutputIntent"),
		}
		switch d.Format {
		case FormatPDFA3b:
			outputIntent.Dictionary["S"] = pdf.Name("GTS_PDFA1")
		case FormatPDFX3, FormatPDFX4:
			outputIntent.Dictionary["S"] = pdf.Name("GTS_PDFX")
		}
		outputIntent.Save()
		d.PDFWriter.Catalog["OutputIntents"] = pdf.Array{outputIntent.ObjectNumber}
	}

	if se := d.RootStructureElement; se != nil {
		// Collect page object numbers for each page index
		pageObjnums := make(map[int]pdf.Objectnumber)
		for _, pg := range d.Pages {
			pageObjnums[pg.pageIndex] = pg.Objectnumber
		}

		structRoot := d.PDFWriter.NewObject()

		// Recursively serialize all structure elements
		d.serializeStructureElement(se, structRoot.ObjectNumber, pageObjnums)

		// Build ParentTree: maps (StructParents index, MCID) → leaf SE object
		var poStr strings.Builder
		for _, po := range d.pdfStructureObjects {
			if po.annotSE != nil {
				// Annotation StructParent: maps directly to the Link SE object
				if po.annotSE.Obj != nil {
					fmt.Fprintf(&poStr, "%d %s ", po.id, po.annotSE.Obj.ObjectNumber.Ref())
				}
			} else {
				// Page StructParent: maps MCID array to leaf SE objects
				pg := d.Pages[po.pageIndex]
				var refs []string
				for _, pse := range pg.StructureElements {
					if pse.Obj != nil {
						refs = append(refs, pse.Obj.ObjectNumber.Ref())
					}
				}
				fmt.Fprintf(&poStr, "%d [%s] ", po.id, strings.Join(refs, " "))
			}
		}

		structRoot.Dictionary = pdf.Dict{
			"Type":       "/StructTreeRoot",
			"ParentTree": fmt.Sprintf("<< /Nums [ %s] >>", poStr.String()),
			"K":          se.Obj.ObjectNumber.Ref(),
		}
		if len(d.RoleMap) > 0 {
			rm := pdf.Dict{}
			for custom, standard := range d.RoleMap {
				rm[pdf.Name(custom)] = "/" + standard
			}
			structRoot.Dictionary["RoleMap"] = rm
		}
		structRoot.Save()

		d.PDFWriter.Catalog["StructTreeRoot"] = structRoot.ObjectNumber.Ref()
		d.ViewerPreferences["DisplayDocTitle"] = "true"
		langStr := "en"
		if d.DefaultLanguageTag != "" {
			langStr = d.DefaultLanguageTag
		} else if d.DefaultLanguage != nil && d.DefaultLanguage.Name != "" {
			langStr = d.DefaultLanguage.Name
		}
		d.PDFWriter.Catalog["Lang"] = fmt.Sprintf("(%s)", langStr)
		d.PDFWriter.Catalog["MarkInfo"] = `<< /Marked true /Suspects false >>`

	}

	rdf := d.PDFWriter.NewObject()
	d.getMetadata(rdf.Data)
	rdf.Dictionary = pdf.Dict{
		"Type":    "/Metadata",
		"Subtype": "/XML",
	}
	err = rdf.Save()
	if err != nil {
		return err
	}
	d.PDFWriter.Catalog["Metadata"] = rdf.ObjectNumber.Ref()
	vp := make(pdf.Dict, len(d.ViewerPreferences))
	for k, v := range d.ViewerPreferences {
		vp[pdf.Name(k)] = v
	}
	d.PDFWriter.Catalog["ViewerPreferences"] = vp

	d.PDFWriter.DefaultPageWidth = d.DefaultPageWidth.ToPT()
	d.PDFWriter.DefaultPageHeight = d.DefaultPageHeight.ToPT()

	d.PDFWriter.InfoDict = pdf.Dict{
		"Producer": pdf.Serialize(pdf.String(d.producer)),
	}
	if t := d.Title; t != "" {
		d.PDFWriter.InfoDict["Title"] = pdf.String(t)
	}
	if t := d.Author; t != "" {
		d.PDFWriter.InfoDict["Author"] = pdf.Serialize(pdf.String(t))
	}
	if t := d.Creator; t != "" {
		d.PDFWriter.InfoDict["Creator"] = pdf.Serialize(pdf.String(t))
	}
	if t := d.Subject; t != "" {
		d.PDFWriter.InfoDict["Subject"] = pdf.Serialize(pdf.String(t))
	}
	if t := d.Keywords; t != "" {
		d.PDFWriter.InfoDict["Keywords"] = pdf.Serialize(pdf.String(t))
	}
	d.PDFWriter.InfoDict["CreationDate"] = formatDate(d.CreationDate)
	if d.Format == FormatPDFX3 {
		d.PDFWriter.InfoDict["GTS_PDFXVersion"] = pdf.String("PDF/X-3:2002")
		d.PDFWriter.InfoDict["ModDate"] = formatDate(d.CreationDate)
		d.PDFWriter.InfoDict["Trapped"] = pdf.Name("False")
	}

	af := pdf.Array{}
	nameTreeData := pdf.NameTreeData{}
	for _, attachment := range d.Attachments {
		pdfAttachment := d.PDFWriter.NewObject()
		pdfAttachment.Dictionary = pdf.Dict{
			"Type":    "/EmbeddedFile",
			"Length":  fmt.Sprintf("%d", len(attachment.Data)),
			"Subtype": pdf.Name(attachment.MimeType),
			"Params": pdf.Dict{
				"Size": fmt.Sprintf("%d", len(attachment.Data)),
			},
		}
		if !attachment.ModDate.IsZero() {
			pdfAttachment.Dictionary["Params"].(pdf.Dict)["ModDate"] = formatDate(attachment.ModDate.UTC())
		}
		pdfAttachment.SetCompression(9)
		pdfAttachment.Data.Write(attachment.Data)
		if err = pdfAttachment.Save(); err != nil {
			return err
		}
		filespec := d.PDFWriter.NewObject()
		filespec.Dictionary = pdf.Dict{
			"Type":           "/Filespec",
			"AFRelationship": pdf.Name("Alternative"),
			"F":              pdf.String(attachment.Name),
			"UF":             pdf.String(attachment.Name),
			"EF": pdf.Dict{
				"F":  pdfAttachment.ObjectNumber.Ref(),
				"UF": pdfAttachment.ObjectNumber.Ref(),
			},
			"Desc": pdf.String(attachment.Description),
		}
		nameTreeData[pdf.String(attachment.Name)] = filespec.ObjectNumber
		if err = filespec.Save(); err != nil {
			return err
		}
		af = append(af, filespec.ObjectNumber)
	}
	if len(af) > 0 {
		ef := d.PDFWriter.GetCatalogNameTreeDict("EmbeddedFiles")
		ef["Names"] = nameTreeData
		d.PDFWriter.Catalog["AF"] = pdf.Serialize(af)
	}
	if err = d.PDFWriter.Finish(); err != nil {
		return err
	}
	if d.Filename != "" {
		bag.Logger.Info("Output written", "filename", d.Filename, "bytes", d.PDFWriter.Size())
	} else {
		bag.Logger.Info("Output written", "bytes", d.PDFWriter.Size())
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
	if cb == CallbackPreShipout {
		d.preShipoutCallback = append(d.preShipoutCallback, fn.(func(page *Page)))
	}
}
