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
	document *Document
	Height   bag.ScaledPoint
	Width    bag.ScaledPoint
	Objects  []Object
	Finished bool
}

// Shipout places all objects on a page and finishes this page.
func (p *Page) Shipout() {
	if p.Finished {
		return
	}
	p.Finished = true

	usedFaces := make(map[*pdf.Face]bool)
	usedImages := make(map[*pdf.Imagefile]bool)
	var currentFont *font.Font
	var s strings.Builder
	inTextMode := false
	stopTextMode := func() {
		if inTextMode {
			fmt.Fprintln(&s, "ET")
			inTextMode = false
		}
	}

	for _, obj := range p.Objects {
		sumV := bag.ScaledPoint(0)
		vlist := obj.Vlist
		for vl := vlist.List; vl != nil; vl = vl.Next() {
			switch v := vl.(type) {
			case *node.HList:
				hlist := v

				if inTextMode {
					fmt.Fprintf(&s, " 1 0 0 1 %s %s Tm  [<", obj.X.String(), (obj.Y - sumV).String())
				}
				for hl := hlist.List; hl != nil; hl = hl.Next() {
					switch n := hl.(type) {
					case *node.Glyph:
						if !inTextMode {
							fmt.Fprintf(&s, "BT %s %s Tf ", n.Font.Face.InternalName(), n.Font.Size.String())
							usedFaces[n.Font.Face] = true
							currentFont = n.Font
							inTextMode = true
							fmt.Fprintf(&s, " 1 0 0 1 %s %s Tm  [<", obj.X.String(), (obj.Y - sumV).String())
						}
						if n.Font != currentFont {
							fmt.Fprintf(&s, `>] %s %s Tf [<`, n.Font.Face.InternalName(), n.Font.Size.String())
							usedFaces[n.Font.Face] = true
							currentFont = n.Font
						}
						n.Font.Face.RegisterChar(n.Codepoint)
						fmt.Fprintf(&s, "%04x", n.Codepoint)
					case *node.Glue:
						fmt.Fprintf(&s, "> -%d <", 1000*n.Width/currentFont.Size)
						if false {
							fmt.Println(currentFont)
						}
					case *node.Lang:
						// ignore
					default:
						fmt.Println(hl)
						panic("nyi")
					}
				}
				sumV += hlist.Height
				fmt.Fprintln(&s, ">]TJ ")
			case *node.Image:
				img := v.Img
				if img.Used {
					bag.LogWarn("image node already in use, id: ", v.ID)
				} else {
					img.Used = true
				}
				ifile := img.ImageFile
				usedImages[ifile] = true
				stopTextMode()

				scaleX := v.Width.ToPT() / ifile.ScaleX
				scaleY := v.Height.ToPT() / ifile.ScaleY

				ht := v.Height
				y := obj.Y - ht
				x := obj.X
				if p.document.IsTrace(VTraceImages) {
					fmt.Fprintf(&s, "q 0.2 w %s %s %s %s re S Q\n", x.String(), y.String(), v.Width.String(), v.Height.String())
				}
				fmt.Fprintf(&s, "q %f 0 0 %f %s %s cm %s Do Q\n", scaleX, scaleY, x.String(), y.String(), img.ImageFile.InternalName())
			}
		}
		stopTextMode()
	}
	st := pdf.NewStream([]byte(s.String()))
	st.SetCompression()
	page := p.document.pdf.AddPage(st)

	page.Width = p.Width
	page.Height = p.Height
	for f := range usedFaces {
		page.Faces = append(page.Faces, f)
	}
	for i := range usedImages {
		page.Images = append(page.Images, i)
	}
}

// Document contains all references to a document
type Document struct {
	Languages         []*lang.Lang
	Faces             []*pdf.Face
	Images            []*pdf.Imagefile
	DefaultPageWidth  bag.ScaledPoint
	DefaultPageHeight bag.ScaledPoint
	Pages             []*Page
	CurrentPage       *Page
	pdf               *pdf.PDF
	tracing           VTrace
}

// NewDocument creates an empty document.
func NewDocument(w io.Writer) *Document {
	d := &Document{}
	d.DefaultPageHeight = bag.MustSp("297mm")
	d.DefaultPageWidth = bag.MustSp("210mm")
	d.pdf = pdf.NewPDFWriter(w)
	return d
}

// LoadPatternFile loads a hyphenation pattern file
func (d *Document) LoadPatternFile(filename string) (*lang.Lang, error) {
	l, err := lang.Load(filename)
	if err != nil {
		return nil, err
	}
	d.Languages = append(d.Languages, l)
	return l, nil
}

// LoadFace loads a font from a TrueType or OpenType collection. The index
// is the 0 based number of font in the file. In most cases there is only one
// font in the font file.
func (d *Document) LoadFace(filename string, index int) (*pdf.Face, error) {
	f, err := pdf.LoadFace(d.pdf, filename, index)
	if err != nil {
		return nil, err
	}

	d.Faces = append(d.Faces, f)
	return f, nil
}

// LoadImageFile loads an image file. Images that should be placed in the PDF file must be
// derived from the file.
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
	d.CurrentPage.Objects = append(d.CurrentPage.Objects, Object{x, y, vlist})
}

// CreateFont returns a new Font object for this face at a given size.
func (d *Document) CreateFont(face *pdf.Face, size bag.ScaledPoint) *font.Font {
	mag := int(size) / int(face.UnitsPerEM)
	return &font.Font{
		Space:        size * 333 / 1000,
		SpaceStretch: size * 167 / 1000,
		SpaceShrink:  size * 111 / 1000,
		Size:         size,
		Face:         face,
		Mag:          mag,
	}
}

// Finish writes all objects to the PDF and writes the XRef section. Finish does not close the writer.
func (d *Document) Finish() error {
	var err error
	d.pdf.Faces = d.Faces
	d.pdf.ImageFiles = d.Images
	if err = d.pdf.Finish(); err != nil {
		return err
	}
	return nil
}
