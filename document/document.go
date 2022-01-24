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

// A Page struct represents a page in a PDF file.
type Page struct {
	document          *Document
	Height            bag.ScaledPoint
	Width             bag.ScaledPoint
	ExtraOffset       bag.ScaledPoint
	Background        []Object
	Objects           []Object
	Userdata          map[interface{}]interface{}
	Finished          bool
	StructureElements []*StructureElement
	Annotations       []pdf.Annotation
}

const (
	pdfCodpointMode = 1
	pdfBracketMode  = 2
	pdfTextMode     = 3
	pdfOuterMode    = 4
)

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
	if cb := p.document.preShipoutCallback; cb != nil {
		cb(p)
	}
	usedFaces := make(map[*pdf.Face]bool)
	usedImages := make(map[*pdf.Imagefile]bool)
	var currentFont *font.Font
	var s strings.Builder
	bleedamount := p.document.Bleed
	textmode := 4
	var tag *StructureElement
	gotoTextMode := func(newMode int) {
		if newMode > textmode {
			if textmode == 1 {
				fmt.Fprint(&s, ">")
				textmode = 2
			}
			if textmode == 2 && textmode < newMode {
				fmt.Fprint(&s, "]TJ\n")
				textmode = 3
			}
			if textmode == 3 && textmode < newMode {
				fmt.Fprint(&s, "ET\n")
				if tag != nil {
					fmt.Fprint(&s, "EMC\n")
					tag = nil
				}
				textmode = 4

			}
			return
		}
		if newMode < textmode {
			if textmode == 4 {
				if tag != nil {
					fmt.Fprintf(&s, "/%s<</MCID %d>>BDC ", tag.Role, tag.ID)
				}
				fmt.Fprint(&s, "BT ")
				textmode = 3
			}
			if textmode == 3 && newMode < textmode {
				fmt.Fprint(&s, "[")
				textmode = 2
			}
			if textmode == 2 && newMode < textmode {
				fmt.Fprint(&s, "<")
				textmode = 1
			}
		}
	}
	offsetX := bleedamount
	offsetY := bleedamount

	objs := make([]Object, 0, len(p.Background)+len(p.Objects))
	objs = append(objs, p.Background...)
	objs = append(objs, p.Objects...)
	for _, obj := range objs {
		objX := obj.X + offsetX
		objY := obj.Y + offsetY
		sumV := bag.ScaledPoint(0)
		vlist := obj.Vlist
		if vlist.Attibutes != nil {
			if r, ok := vlist.Attibutes["tag"]; ok {
				tag = r.(*StructureElement)
				tag.ID = len(p.StructureElements)
				p.StructureElements = append(p.StructureElements, tag)
			}
		}
		// horizontal elements
		for hElt := vlist.List; hElt != nil; hElt = hElt.Next() {
			var glueString string
			var shiftX bag.ScaledPoint
			sumx := bag.ScaledPoint(0)
			switch v := hElt.(type) {
			case *node.HList:
				// The first hlist: move cursor down
				if hElt == vlist.List {
					sumV += v.Height
				}
				hlist := v

				for itm := hlist.List; itm != nil; itm = itm.Next() {
					switch n := itm.(type) {
					case *node.Glyph:
						if n.Font != currentFont {
							gotoTextMode(3)
							fmt.Fprintf(&s, "\n%s %s Tf ", n.Font.Face.InternalName(), n.Font.Size)
							usedFaces[n.Font.Face] = true
							currentFont = n.Font
							glueString = ""
						}
						if textmode > 3 {
							gotoTextMode(3)
						}
						if textmode > 2 {
							fmt.Fprintf(&s, "\n1 0 0 1 %s %s Tm ", objX+shiftX+sumx, (objY - sumV))
							shiftX = 0
						}
						n.Font.Face.RegisterChar(n.Codepoint)
						if glueString != "" {
							gotoTextMode(2)
							fmt.Fprintf(&s, "%s", glueString)
							glueString = ""
						}
						gotoTextMode(1)
						fmt.Fprintf(&s, "%04x", n.Codepoint)
						sumx += n.Width
					case *node.Glue:
						if textmode == 1 {
							// Single glue at end should not be printed. Therefore we save it for later.
							glueString = fmt.Sprintf(" %d ", -1*1000*n.Width/currentFont.Size)
						}
						sumx += n.Width
					case *node.Rule:
						if textmode == 1 && glueString != "" {
							gotoTextMode(2)
							fmt.Fprint(&s, glueString)
							glueString = ""
						}
						gotoTextMode(4)
						glueString = ""
						posX := objX + sumx
						posY := objY - sumV
						fmt.Fprintf(&s, " 1 0 0 1 %s %s cm ", posX, posY)
						fmt.Fprint(&s, n.Pre)
						fmt.Fprintf(&s, " 1 0 0 1 %s %s cm ", -posX, -posY)
					case *node.Image:
						gotoTextMode(4)
						img := n.Img
						if img.Used {
							bag.Logger.Warn(fmt.Sprintf("image node already in use, id: %d", v.ID))
						} else {
							img.Used = true
						}
						ifile := img.ImageFile
						usedImages[ifile] = true

						scaleX := v.Width.ToPT() / ifile.ScaleX
						scaleY := v.Height.ToPT() / ifile.ScaleY
						ht := v.Height
						y := objY - ht
						x := objX
						if p.document.IsTrace(VTraceImages) {
							fmt.Fprintf(&s, "q 0.2 w %s %s %s %s re S Q\n", x, y, v.Width, v.Height)
						}
						fmt.Fprintf(&s, "q %f 0 0 %f %s %s cm %s Do Q\n", scaleX, scaleY, x, y, img.ImageFile.InternalName())
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

						if textmode == 1 && glueString != "" {
							gotoTextMode(2)
							fmt.Fprint(&s, glueString)
							glueString = ""
						}

						if action == node.ActionHyperlink {
							posX := objX + sumx
							posY := objY - sumV - currentFont.Depth
							hyperlink := startNode.Value.(*Hyperlink)
							if isStartNode {
								hyperlink.startposX = posX
								hyperlink.startposY = posY
							} else {
								a := pdf.Annotation{
									Rect:    [4]bag.ScaledPoint{hyperlink.startposX, hyperlink.startposY, hyperlink.startposX + 50*bag.Factor, posY + hlist.Height + currentFont.Depth},
									Subtype: "Link",
									URI:     hyperlink.URI,
								}
								p.Annotations = append(p.Annotations, a)
							}
						}
						switch n.Position {
						case node.PDFOutputPage:
							gotoTextMode(4)
						case node.PDFOutputDirect:
							gotoTextMode(3)
						case node.PDFOutputHere:
							gotoTextMode(4)
							posX := objX + sumx
							posY := objY - sumV
							fmt.Fprintf(&s, " 1 0 0 1 %s %s cm ", posX, posY)
						case node.PDFOutputLowerLeft:
							gotoTextMode(4)
						}
						glueString = ""
						if n.Callback != nil {
							fmt.Fprint(&s, n.Callback(n))
						}
						switch n.Position {
						case node.PDFOutputHere:
							posX := objX + sumx
							posY := objY - sumV
							fmt.Fprintf(&s, " 1 0 0 1 %s %s cm ", -posX, -posY)
						}
					case *node.Lang, *node.Penalty:
						// ignore
					case *node.Disc:
						// ignore
					default:
						bag.Logger.DPanicf("Shipout: unknown node %v", itm)
					}
				}
				sumV += hlist.Height + hlist.Depth
				if textmode < 3 {
					gotoTextMode(3)
				}
			case *node.Image:
				img := v.Img
				if img.Used {
					bag.Logger.Warn(fmt.Sprintf("image node already in use, id: %d", v.ID))
				} else {
					img.Used = true
				}
				ifile := img.ImageFile
				usedImages[ifile] = true
				gotoTextMode(4)

				scaleX := v.Width.ToPT() / ifile.ScaleX
				scaleY := v.Height.ToPT() / ifile.ScaleY

				ht := v.Height
				y := objY - ht
				x := objX
				if p.document.IsTrace(VTraceImages) {
					fmt.Fprintf(&s, "q 0.2 w %s %s %s %s re S Q\n", x, y, v.Width, v.Height)
				}
				fmt.Fprintf(&s, "q %f 0 0 %f %s %s cm %s Do Q\n", scaleX, scaleY, x, y, img.ImageFile.InternalName())
			case *node.Glue:
				// Let's assume that the glue ratio has been determined and the
				// natural width is in v.Width for now.
				sumV += v.Width
			case *node.Rule:
				posX := objX + sumx
				posY := objY - sumV
				fmt.Fprintf(&s, " 1 0 0 1 %s %s cm ", posX, posY)
				fmt.Fprint(&s, v.Pre)
				fmt.Fprintf(&s, " 1 0 0 1 %s %s cm ", -posX, -posY)

			default:
				bag.Logger.DPanicf("Shipout: unknown node %T in vertical mode", v)
			}
		}
		gotoTextMode(4)
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
}

// CallbackShipout gets called before the shipout process starts.
type CallbackShipout func(page *Page)

// StructureElement represents a tagged PDF element such as H1 or P.
type StructureElement struct {
	ID         int
	Role       string
	ActualText string
	Children   []*StructureElement
	Parent     *StructureElement
	Obj        *pdf.Object
}

// pdfStructureObject holds information about the PDF/UA structures for each
// page, annotation and XObject.
type pdfStructureObject struct {
	id   int
	refs string
}

func (d *Document) newPDFStructureObject() *pdfStructureObject {
	po := &pdfStructureObject{}
	po.id = len(d.pdfStructureObjects)
	return po
}

// Document contains all references to a document
type Document struct {
	Languages            map[string]*lang.Lang
	Faces                []*pdf.Face
	Images               []*pdf.Imagefile
	FontFamilies         []*FontFamily
	DefaultPageWidth     bag.ScaledPoint
	DefaultPageHeight    bag.ScaledPoint
	DefaultLanguage      *lang.Lang
	Pages                []*Page
	CurrentPage          *Page
	Filename             string
	Bleed                bag.ScaledPoint
	Title                string
	pdf                  *pdf.PDF
	tracing              VTrace
	usedFonts            map[*pdf.Face]map[bag.ScaledPoint]*font.Font
	colors               map[string]*Color
	RootStructureElement *StructureElement
	pdfStructureObjects  []*pdfStructureObject
	preShipoutCallback   CallbackShipout
}

// NewDocument creates an empty document.
func NewDocument(w io.Writer) *Document {
	d := &Document{}
	d.DefaultPageHeight = bag.MustSp("297mm")
	d.DefaultPageWidth = bag.MustSp("210mm")
	d.pdf = pdf.NewPDFWriter(w)
	d.Title = "document"
	d.colors = csscolors
	d.Languages = make(map[string]*lang.Lang)
	return d
}

// LoadPatternFile loads a hyphenation pattern file.
func (d *Document) LoadPatternFile(filename string, langname string) (*lang.Lang, error) {
	l, err := lang.LoadPatternFile(filename)
	if err != nil {
		return nil, err
	}
	d.Languages[langname] = l
	return l, nil
}

// SetDefaultLanguage sets the document default language.
func (d *Document) SetDefaultLanguage(l *lang.Lang) {
	d.DefaultLanguage = l
}

// LoadFace loads a font from a TrueType or OpenType collection.
func (d *Document) LoadFace(fs *FontSource) (*pdf.Face, error) {
	bag.Logger.Debugf("LoadFace %s", fs)
	if fs.face != nil {
		return fs.face, nil
	}

	f, err := pdf.LoadFace(d.pdf, fs.Source, fs.Index)
	if err != nil {
		return nil, err
	}
	fs.face = f
	d.Faces = append(d.Faces, f)
	return f, nil
}

// LoadImageFile loads an image file. Images that should be placed in the PDF
// file must be derived from the file.
func (d *Document) LoadImageFile(filename string) (*pdf.Imagefile, error) {
	img, err := pdf.LoadImageFile(d.pdf, filename)
	if err != nil {
		return nil, err
	}
	d.Images = append(d.Images, img)
	return img, nil
}

// CreateImage returns a new Image derived from the image file.
func (d *Document) CreateImage(imgfile *pdf.Imagefile) *image.Image {
	img := &image.Image{}
	img.ImageFile = imgfile
	return img
}

// NewPage creates a new Page object and adds it to the page list in the document.
func (d *Document) NewPage() *Page {
	d.CurrentPage = &Page{
		document: d,
		Width:    d.DefaultPageWidth,
		Height:   d.DefaultPageHeight,
	}
	d.Pages = append(d.Pages, d.CurrentPage)
	return d.CurrentPage
}

// OutputAt places the nodelist at the position.
func (d *Document) OutputAt(x bag.ScaledPoint, y bag.ScaledPoint, vlist *node.VList) {
	if d.CurrentPage == nil {
		d.CurrentPage = d.NewPage()
	}
	d.CurrentPage.OutputAt(x, y, vlist)
}

// CreateFont returns a new Font object for this face at a given size.
func (d *Document) CreateFont(face *pdf.Face, size bag.ScaledPoint) *font.Font {
	return font.NewFont(face, size)
}

// Finish writes all objects to the PDF and writes the XRef section. Finish does
// not close the writer.
func (d *Document) Finish() error {
	var err error
	d.pdf.Catalog = pdf.Dict{}

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
		for _, childSe := range se.Children {
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
	d.pdf.Faces = d.Faces
	d.pdf.ImageFiles = d.Images
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
func (d *Document) RegisterCallback(cb Callback, fn interface{}) {
	switch cb {
	case CallbackPreShipout:
		d.preShipoutCallback = fn.(func(page *Page))
	}
}
