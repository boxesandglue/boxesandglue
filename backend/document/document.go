package document

import (
	"fmt"
	"io"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/boxesandglue/backend/image"
	"github.com/speedata/boxesandglue/backend/lang"
	"github.com/speedata/boxesandglue/backend/node"
	"github.com/speedata/boxesandglue/pdfbackend/pdf"
)

// Object contains a vertical list and coordinates to be placed on a page.
type Object struct {
	X     bag.ScaledPoint
	Y     bag.ScaledPoint
	Vlist *node.VList
}

// A Hyperlink represents a clickable thing in the PDF.
type Hyperlink struct {
	URI         string
	Annotations []pdf.Annotation
	startposX   bag.ScaledPoint
	startposY   bag.ScaledPoint
}

// A Page struct represents a page in a PDF file.
type Page struct {
	document          *PDFDocument
	Height            bag.ScaledPoint
	Width             bag.ScaledPoint
	ExtraOffset       bag.ScaledPoint
	Background        []Object
	Objects           []Object
	Userdata          map[interface{}]interface{}
	Finished          bool
	StructureElements []*StructureElement
	Annotations       []pdf.Annotation
	Spotcolors        []*Color
}

const (
	pdfCodpointMode = 1
	pdfBracketMode  = 2
	pdfTextMode     = 3
	pdfOuterMode    = 4
)

// objectContext contains information about the current position of the cursor
// and about the images and faces used in the object.
type objectContext struct {
	p           *Page
	currentFont *font.Font
	textmode    uint8
	usedFaces   map[*pdf.Face]bool
	usedImages  map[*pdf.Imagefile]bool
	tag         *StructureElement
	s           *strings.Builder
	glueString  string
	shiftX      bag.ScaledPoint
	sumX        bag.ScaledPoint
	sumV        bag.ScaledPoint
	objX        bag.ScaledPoint
	objY        bag.ScaledPoint
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

// outputHorizontalItem outputs a single horizontal item and advance the cursor.
func (oc *objectContext) outputHorizontalItem(v *node.HList, itm node.Node) {
	switch n := itm.(type) {
	case *node.Glyph:
		if n.Font != oc.currentFont {
			oc.gotoTextMode(3)
			fmt.Fprintf(oc.s, "\n%s %s Tf ", n.Font.Face.InternalName(), n.Font.Size)
			oc.usedFaces[n.Font.Face] = true
			oc.currentFont = n.Font
			oc.glueString = ""
		}
		if oc.textmode > 3 {
			oc.gotoTextMode(3)
		}
		if oc.textmode > 2 {
			fmt.Fprintf(oc.s, "\n1 0 0 1 %s %s Tm ", oc.objX+oc.shiftX+oc.sumX, (oc.objY - oc.sumV))
			oc.shiftX = 0
		}
		n.Font.Face.RegisterChar(n.Codepoint)
		if oc.glueString != "" {
			oc.gotoTextMode(2)
			fmt.Fprintf(oc.s, "%s", oc.glueString)
			oc.glueString = ""
		}
		oc.gotoTextMode(1)
		fmt.Fprintf(oc.s, "%04x", n.Codepoint)
		oc.sumX += n.Width
	case *node.Glue:
		if oc.textmode == 1 {
			// Single glue at end should not be printed. Therefore we save it for later.
			oc.glueString = fmt.Sprintf(" %d ", -1*1000*n.Width/oc.currentFont.Size)
		}
		oc.sumX += n.Width
	case *node.Rule:
		if oc.textmode == 1 && oc.glueString != "" {
			oc.gotoTextMode(2)
			fmt.Fprint(oc.s, oc.glueString)
			oc.glueString = ""
		}
		oc.gotoTextMode(4)
		oc.glueString = ""
		posX := oc.objX + oc.sumX
		posY := oc.objY - oc.sumV
		fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", posX, posY)
		fmt.Fprint(oc.s, n.Pre)
		fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", -posX, -posY)
	case *node.Image:
		oc.gotoTextMode(4)
		img := n.Img
		if img.Used {
			bag.Logger.Warn(fmt.Sprintf("image node already in use, id: %d", v.ID))
		} else {
			img.Used = true
		}
		ifile := img.ImageFile
		oc.usedImages[ifile] = true

		scaleX := v.Width.ToPT() / ifile.ScaleX
		scaleY := v.Height.ToPT() / ifile.ScaleY
		ht := v.Height
		y := oc.objY - ht
		x := oc.objX
		fmt.Fprintf(oc.s, "q %f 0 0 %f %s %s cm %s Do Q\n", scaleX, scaleY, x, y, img.ImageFile.InternalName())
	case *node.StartStop:
		isStartNode := true
		action := n.Action

		var startNode *node.StartStop
		if n.StartNode != nil {
			// a stop node which has a link to a start node
			isStartNode = false
			startNode = n.StartNode
			action = startNode.Action
		} else {
			startNode = n
		}

		if oc.textmode == 1 && oc.glueString != "" {
			oc.gotoTextMode(2)
			fmt.Fprint(oc.s, oc.glueString)
			oc.glueString = ""
		}

		if action == node.ActionHyperlink {
			posX := oc.objX + oc.sumX
			posY := oc.objY - oc.sumV - oc.currentFont.Depth
			hyperlink := startNode.Value.(*Hyperlink)
			if isStartNode {
				hyperlink.startposX = posX
				hyperlink.startposY = posY
			} else {
				a := pdf.Annotation{
					Rect:    [4]bag.ScaledPoint{hyperlink.startposX, hyperlink.startposY, hyperlink.startposX + 50*bag.Factor, posY + v.Height + oc.currentFont.Depth},
					Subtype: "Link",
					URI:     hyperlink.URI,
				}
				oc.p.Annotations = append(oc.p.Annotations, a)
			}
		}
		switch n.Position {
		case node.PDFOutputPage:
			oc.gotoTextMode(4)
		case node.PDFOutputDirect:
			oc.gotoTextMode(3)
		case node.PDFOutputHere:
			oc.gotoTextMode(4)
			posX := oc.objX + oc.sumX
			posY := oc.objY - oc.sumV
			fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", posX, posY)
		case node.PDFOutputLowerLeft:
			oc.gotoTextMode(4)
		}
		oc.glueString = ""
		if n.Callback != nil {
			fmt.Fprint(oc.s, n.Callback(n))
		}
		switch n.Position {
		case node.PDFOutputHere:
			posX := oc.objX + oc.sumX
			posY := oc.objY - oc.sumV
			fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", -posX, -posY)
		}
	case *node.Lang, *node.Penalty:
		// ignore
	case *node.Disc:
		// ignore
	case *node.HList:
		oc.outputHorizontalItem(n, n.List)
	case *node.VList:
		for vItem := v.List; vItem != nil; vItem = vItem.Next() {
			oc.outputVerticalItem(n, vItem)
		}
	default:
		bag.Logger.DPanicf("Shipout: unknown node %v", itm)
	}
}

func (oc *objectContext) outputVerticalItem(vlist *node.VList, hElt node.Node) {
	oc.sumX = 0
	switch v := hElt.(type) {
	case *node.HList:
		// The first hlist: move cursor down
		if hElt == vlist.List {
			oc.sumV += v.Height
		}

		for itm := v.List; itm != nil; itm = itm.Next() {
			oc.outputHorizontalItem(v, itm)
		}
		oc.sumV += v.Height + v.Depth
		if oc.textmode < 3 {
			oc.gotoTextMode(3)
		}
	case *node.Image:
		img := v.Img
		if img.Used {
			bag.Logger.Warnf("image node already in use, id: %d", v.ID)
		} else {
			img.Used = true
		}
		ifile := img.ImageFile
		oc.usedImages[ifile] = true
		oc.gotoTextMode(4)

		scaleX := v.Width.ToPT() / ifile.ScaleX
		scaleY := v.Height.ToPT() / ifile.ScaleY

		ht := v.Height
		y := oc.objY - ht
		x := oc.objX
		if oc.p.document.IsTrace(VTraceImages) {
			fmt.Fprintf(oc.s, "q 0.2 w %s %s %s %s re S Q\n", x, y, v.Width, v.Height)
		}
		fmt.Fprintf(oc.s, "q %f 0 0 %f %s %s cm %s Do Q\n", scaleX, scaleY, x, y, img.ImageFile.InternalName())
	case *node.Glue:
		// Let's assume that the glue ratio has been determined and the
		// natural width is in v.Width for now.
		oc.sumV += v.Width
	case *node.Rule:
		posX := oc.objX + oc.sumX
		posY := oc.objY - oc.sumV
		fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", posX, posY)
		fmt.Fprint(oc.s, v.Pre)
		fmt.Fprintf(oc.s, " 1 0 0 1 %s %s cm ", -posX, -posY)
	case *node.VList:
		for vItem := v.List; vItem != nil; vItem = vItem.Next() {
			oc.outputVerticalItem(vlist, vItem)
		}
	default:
		bag.Logger.DPanicf("Shipout: unknown node %T in vertical mode", v)
	}

}

// OutputAt places the nodelist at the position.
func (p *Page) OutputAt(x bag.ScaledPoint, y bag.ScaledPoint, vlist *node.VList) {
	p.Objects = append(p.Objects, Object{x, y, vlist})
}

// Shipout places all objects on a page and finishes this page.
func (p *Page) Shipout() {
	bag.Logger.Debug("Shipout")
	if p.Finished {
		return
	}
	p.Finished = true
	var s strings.Builder
	if cb := p.document.preShipoutCallback; cb != nil {
		cb(p)
	}
	bleedamount := p.document.Bleed

	offsetX := bleedamount
	offsetY := bleedamount

	objs := make([]Object, 0, len(p.Background)+len(p.Objects))
	objs = append(objs, p.Background...)
	objs = append(objs, p.Objects...)
	usedFaces := make(map[*pdf.Face]bool)
	usedImages := make(map[*pdf.Imagefile]bool)

	for _, obj := range objs {
		oc := &objectContext{
			textmode:   4,
			s:          &s,
			usedFaces:  make(map[*pdf.Face]bool),
			usedImages: make(map[*pdf.Imagefile]bool),
			p:          p,
			objX:       obj.X + offsetX,
			objY:       obj.Y + offsetY,
		}

		vlist := obj.Vlist
		if vlist.Attibutes != nil {
			if r, ok := vlist.Attibutes["tag"]; ok {
				oc.tag = r.(*StructureElement)
				oc.tag.ID = len(p.StructureElements)
				p.StructureElements = append(p.StructureElements, oc.tag)
			}
		}
		// output vertical items
		for vItem := vlist.List; vItem != nil; vItem = vItem.Next() {
			oc.outputVerticalItem(vlist, vItem)
		}
		for k := range oc.usedFaces {
			usedFaces[k] = true
		}
		for k := range oc.usedImages {
			usedImages[k] = true
		}
		oc.gotoTextMode(4)
	}

	st := pdf.NewStream([]byte(s.String()))
	// st.SetCompression()
	page := p.document.pdf.AddPage(st)
	page.Dict = make(pdf.Dict)
	page.Width = p.Width + 2*offsetX
	page.Height = p.Height + 2*offsetY
	if bleedamount > 0 {
		page.Dict["/TrimBox"] = fmt.Sprintf("[%s %s %s %s]", bleedamount, bleedamount, page.Width-bleedamount, page.Height-bleedamount)
	}

	for f := range usedFaces {
		page.Faces = append(page.Faces, f)
	}
	for i := range usedImages {
		page.Images = append(page.Images, i)
	}

	var structureElementObjectIDs []string
	if p.document.RootStructureElement != nil {

		page.Annotations = p.Annotations
		for _, se := range p.StructureElements {
			parent := se.Parent
			if parent.Obj == nil {
				parent.Obj = p.document.pdf.NewObject()
			}
			se.Obj = p.document.pdf.NewObject()
			se.Obj.Dictionary = pdf.Dict{
				"/Type": "/StructElem",
				"/S":    "/" + se.Role,
				"/K":    fmt.Sprintf("%d", se.ID),
				"/Pg":   page.Dictnum.Ref(),
				"/P":    parent.Obj.ObjectNumber.Ref(),
			}
			if se.ActualText != "" {
				se.Obj.Dictionary["/ActualText"] = pdf.StringToPDF(se.ActualText)
			}
			se.Obj.Save()
			structureElementObjectIDs = append(structureElementObjectIDs, se.Obj.ObjectNumber.Ref())
		}
		po := p.document.newPDFStructureObject()
		po.refs = strings.Join(structureElementObjectIDs, " ")
		page.Dict["/StructParents"] = fmt.Sprintf("%d", po.id)
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
	Languages            map[string]*lang.Lang
	Faces                []*pdf.Face
	DefaultPageWidth     bag.ScaledPoint
	DefaultPageHeight    bag.ScaledPoint
	DefaultLanguage      *lang.Lang
	Pages                []*Page
	CurrentPage          *Page
	Filename             string
	Bleed                bag.ScaledPoint
	Title                string
	Spotcolors           []*Color
	pdf                  *pdf.PDF
	tracing              VTrace
	RootStructureElement *StructureElement
	ColorProfile         *ColorProfile
	pdfStructureObjects  []*pdfStructureObject
	preShipoutCallback   CallbackShipout
	usedPDFImages        map[string]*pdf.Imagefile
}

// NewDocument creates an empty document.
func NewDocument(w io.Writer) *PDFDocument {
	d := &PDFDocument{
		DefaultPageWidth:  bag.MustSp("210mm"),
		DefaultPageHeight: bag.MustSp("297mm"),
		Title:             "document",
		Languages:         make(map[string]*lang.Lang),
		pdf:               pdf.NewPDFWriter(w),
		usedPDFImages:     make(map[string]*pdf.Imagefile),
	}
	return d
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
	bag.Logger.Debugf("LoadFace %s", filename)

	f, err := pdf.LoadFace(d.pdf, filename, index)
	if err != nil {
		return nil, err
	}
	d.Faces = append(d.Faces, f)
	return f, nil
}

// LoadImageFile loads an image file. Images that should be placed in the PDF
// file must be derived from the file.
func (d *PDFDocument) LoadImageFile(filename string) (*pdf.Imagefile, error) {
	if imgf, ok := d.usedPDFImages[filename]; ok {
		return imgf, nil
	}
	imgf, err := pdf.LoadImageFile(d.pdf, filename)
	if err != nil {
		return nil, err
	}
	d.usedPDFImages[filename] = imgf
	return imgf, nil
}

// CreateImage returns a new Image derived from the image file. The parameter
// pagenumber is honored only in PDF files.
func (d *PDFDocument) CreateImage(imgfile *pdf.Imagefile, pagenumber int) *image.Image {
	img := &image.Image{}
	img.ImageFile = imgfile
	img.PageNumber = pagenumber
	switch img.ImageFile.Format {
	case "pdf":
		thisPageSizes := imgfile.PageSizes[pagenumber]
		mb := thisPageSizes["/MediaBox"]
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
	d.pdf.Catalog = pdf.Dict{}
	if d.ColorProfile != nil {
		cp := d.pdf.NewObject()
		cp.Data.Write(d.ColorProfile.data)
		cp.Dictionary = pdf.Dict{
			"/N": fmt.Sprintf("%d", d.ColorProfile.Colors),
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
			sep.ID = fmt.Sprintf("/CS%d", col.Spotcolorid)

			sepObj := d.pdf.NewObject()
			sepObj.Array = []interface{}{
				"/Separation",
				pdf.Name(col.Basecolor),
				pdf.Array{"/ICCBased", cp.ObjectNumber},
				pdf.Dict{
					"/C0":           pdf.ArrayToString(pdf.Array{0, 0, 0, 0}),
					"/C1":           pdf.ArrayToString(pdf.Array{sep.C, sep.M, sep.Y, sep.K}),
					"/Domain":       "[ 0 1 ]",
					"/FunctionType": "2",
					"/N":            "1",
				},
			}
			sepObj.Save()
			sep.Obj = sepObj.ObjectNumber
			d.pdf.Colorspaces = append(d.pdf.Colorspaces, &sep)
		}
	}

	if se := d.RootStructureElement; se != nil {
		if se.Obj == nil {
			se.Obj = d.pdf.NewObject()
		}
		var poStr strings.Builder

		// structure objects are a used to lookup stucture elements for a page
		for _, po := range d.pdfStructureObjects {
			poStr.WriteString(fmt.Sprintf("%d [%s]", po.id, po.refs))
		}
		childObjectNumbers := []string{}
		for _, childSe := range se.children {
			childObjectNumbers = append(childObjectNumbers, childSe.Obj.ObjectNumber.Ref())
		}
		structRoot := d.pdf.NewObject()
		structRoot.Dictionary = pdf.Dict{
			"/Type":       "/StructTreeRoot",
			"/ParentTree": fmt.Sprintf("<< /Nums [ %s ] >>", poStr.String()),
			"/K":          se.Obj.ObjectNumber.Ref(),
		}
		structRoot.Save()
		se.Obj.Dictionary = pdf.Dict{
			"/S":    "/" + se.Role,
			"/K":    fmt.Sprintf("%s", childObjectNumbers),
			"/P":    structRoot.ObjectNumber.Ref(),
			"/Type": "/StructElem",
			"/T":    pdf.StringToPDF(d.Title),
		}
		se.Obj.Save()

		d.pdf.Catalog["/StructTreeRoot"] = structRoot.ObjectNumber.Ref()
		d.pdf.Catalog["/ViewerPreferences"] = "<< /DisplayDocTitle true >>"
		d.pdf.Catalog["/Lang"] = "(en)"
		d.pdf.Catalog["/MarkInfo"] = `<< /Marked true /Suspects false  >>`

	}

	rdf := d.pdf.NewObject()
	rdf.Data.WriteString(d.getMetadata())
	rdf.Dictionary = pdf.Dict{
		"/Type":    "/Metadata",
		"/Subtype": "/XML",
	}
	err = rdf.Save()
	if err != nil {
		return err
	}
	d.pdf.Catalog["/Metadata"] = rdf.ObjectNumber.Ref()
	d.pdf.DefaultPageWidth = d.DefaultPageWidth
	d.pdf.DefaultPageHeight = d.DefaultPageHeight
	if err = d.pdf.Finish(); err != nil {
		return err
	}
	if d.Filename != "" {
		bag.Logger.Infof("Output written to %s (%d bytes)", d.Filename, d.pdf.Size())
	} else {
		bag.Logger.Info("Output written (%d bytes)", d.pdf.Size())
	}
	bag.Logger.Sync()
	return nil
}

// Callback represents the type of callback to register.
type Callback int

const (
	// CallbackPreShipout is called right before a page shipout. It is called once for each page.
	CallbackPreShipout Callback = iota
)

// RegisterCallback registers the callback in fn.
func (d *PDFDocument) RegisterCallback(cb Callback, fn interface{}) {
	switch cb {
	case CallbackPreShipout:
		d.preShipoutCallback = fn.(func(page *Page))
	}
}
