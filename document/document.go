package document

import (
	"fmt"
	"io"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/font"
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
	var currentFont *font.Font
	var s strings.Builder
	sumV := bag.ScaledPoint(0)
	for _, obj := range p.Objects {
		vlist := obj.Vlist
		firstFont := vlist.FirstFont
		fmt.Fprintf(&s, "BT %s %f Tf ", firstFont.Face.InternalName(), float64(firstFont.Size/bag.Factor))
		usedFaces[vlist.FirstFont.Face] = true
		currentFont = vlist.FirstFont

		for vl := vlist.List.Front(); vl != nil; vl = vl.Next() {
			hlist := vl.Value.(*node.HList)
			fmt.Fprintf(&s, " 1 0 0 1 %4f %4f Tm  [<", obj.X.Float()/bag.Factor.Float(), (obj.Y-sumV).Float()/bag.Factor.Float())
			for hl := hlist.List.Front(); hl != nil; hl = hl.Next() {
				switch n := hl.Value.(type) {
				case *node.Glyph:
					if n.NewFont {
						fmt.Fprintf(&s, `>] %s %f Tf [<`, n.Font.Face.InternalName(), float64(n.Font.Size/bag.Factor))
						usedFaces[n.Font.Face] = true
						currentFont = n.Font
					}
					n.Font.Face.RegisterChar(n.Codepoint)
					fmt.Fprintf(&s, "%04x", n.Codepoint)
				case *node.Glue:
					fmt.Fprintf(&s, "> -%.4f <", 1000*n.Width.Float()*bag.Factor.Float()/currentFont.Size.Float())
					if false {
						fmt.Println(currentFont)
					}
				case *node.Lang:
					// ignore
				default:
					fmt.Println(hl.Value)
					panic("nyi")
				}
			}
			sumV += hlist.Height
			fmt.Fprintln(&s, ">]TJ ")
		}

		fmt.Fprintln(&s, "ET")

	}
	st := pdf.NewStream([]byte(s.String()))
	// st.SetCompression()
	page := p.document.pdf.AddPage(st)
	for f := range usedFaces {
		page.Faces = append(page.Faces, f)
	}
}

// Document contains all references to a document
type Document struct {
	Languages         []*lang.Lang
	Faces             []*pdf.Face
	DefaultPageWidth  bag.ScaledPoint
	DefaultPageHeight bag.ScaledPoint
	Pages             []*Page
	CurrentPage       *Page
	pdf               *pdf.PDF
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

// OutputAt places the nodelist at the position.
func (d *Document) OutputAt(x bag.ScaledPoint, y bag.ScaledPoint, vlist *node.VList) {
	if d.CurrentPage == nil {
		d.CurrentPage = &Page{
			document: d,
		}
		d.Pages = append(d.Pages, d.CurrentPage)
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
	if err = d.pdf.Finish(); err != nil {
		return err
	}
	return nil
}
