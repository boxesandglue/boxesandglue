package pdf

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"fmt"
	"io"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/gofpdi"
)

// Objectnumber represents a PDF object number
type Objectnumber int

// Ref returns a reference to the object number
func (o Objectnumber) Ref() string {
	return fmt.Sprintf("%d 0 R", o)
}

// String returns a reference to the object number
func (o Objectnumber) String() string {
	return fmt.Sprintf("%d 0 R", o)
}

// Dict is a string - string dictionary
type Dict map[string]string

func (d Dict) String() string {
	return HashToString(d, 0)
}

// Array is a list of anything
type Array []interface{}

func (ary Array) String() string {
	return ArrayToString(ary)
}

// Name represents a PDF name such as Adobe Green. The String() method prepends
// a / (slash) to the name.
type Name string

func (n Name) String() string {
	a := strings.NewReplacer(" ", "#20")
	return "/" + a.Replace(string(n))
}

// Pages is the parent page structure
type Pages struct {
	pages   []*Page
	dictnum Objectnumber
}

// An Annotation is a PDF element that is additional to the text, such as a
// hyperlink or a note.
type Annotation struct {
	Subtype string
	Border  []int
	Rect    [4]bag.ScaledPoint
	URI     string
}

// Separation repressents a spot color
type Separation struct {
	Obj        Objectnumber
	ID         string
	Name       string
	ICCProfile Objectnumber
	C          float64
	M          float64
	Y          float64
	K          float64
}

// Page contains information about a single page.
type Page struct {
	Obj         *Object
	Dictnum     Objectnumber
	Annotations []Annotation
	Faces       []*Face
	Images      []*Imagefile
	stream      *Stream
	Width       bag.ScaledPoint
	Height      bag.ScaledPoint
	Dict        Dict
}

// PDF is the central point of writing a PDF file.
type PDF struct {
	outfile           io.Writer
	nextobject        Objectnumber
	Catalog           Dict
	DefaultPageWidth  bag.ScaledPoint
	DefaultPageHeight bag.ScaledPoint
	Colorspaces       []*Separation
	objectlocations   map[Objectnumber]int64
	pages             *Pages
	lastEOL           int64
	pos               int64
}

// NewPDFWriter creates a PDF file for writing to file
func NewPDFWriter(file io.Writer) *PDF {
	pw := PDF{}
	pw.outfile = file
	pw.nextobject = 1
	pw.objectlocations = make(map[Objectnumber]int64)
	pw.pages = &Pages{}
	pw.Println("%PDF-1.7")
	return &pw
}

// Print writes the string to the PDF file
func (pw *PDF) Print(s string) error {
	n, err := fmt.Fprint(pw.outfile, s)
	pw.pos += int64(n)
	return err
}

// Println writes the string to the PDF file and adds a newline.
func (pw *PDF) Println(s string) error {
	n, err := fmt.Fprintln(pw.outfile, s)
	pw.pos += int64(n)
	return err
}

// Printf writes the string to the PDF file and adds a newline.
func (pw *PDF) Printf(format string, a ...interface{}) error {
	n, err := fmt.Fprintf(pw.outfile, format, a...)
	pw.pos += int64(n)
	return err
}

// AddPage adds a page to the PDF file. The stream must be complete.
func (pw *PDF) AddPage(pagestream *Stream) *Page {
	pg := &Page{}
	pg.Obj = pw.NewObject()
	pg.stream = pagestream
	pg.Dictnum = pw.NextObject()
	pw.pages.pages = append(pw.pages.pages, pg)
	return pg
}

// NextObject returns the next free object number
func (pw *PDF) NextObject() Objectnumber {
	pw.nextobject++
	return pw.nextobject - 1
}

// writeStream writes a new stream object with the data and extra dictionary
// entries. writeStream returns the object number of the stream.
func (pw *PDF) writeStream(st *Stream, obj *Object) (*Object, error) {
	if obj == nil {
		obj = pw.NewObject()
	}
	if st.compress {
		st.dict["/Filter"] = "/FlateDecode"
		var b bytes.Buffer
		zw := zlib.NewWriter(&b)
		zw.Write(st.data)
		zw.Close()
		st.dict["/Length"] = fmt.Sprintf("%d", b.Len())
		st.dict["/Length1"] = fmt.Sprintf("%d", len(st.data))
		st.data = b.Bytes()
	} else {
		st.dict["/Length"] = fmt.Sprintf("%d", len(st.data))
	}

	obj.Dict(st.dict)
	var err error

	if _, err = obj.Data.Write(st.data); err != nil {
		return nil, err
	}

	obj.Save()
	return obj, nil
}

func (pw *PDF) writeDocumentCatalogAndPages() (Objectnumber, error) {
	var err error
	usedFaces := make(map[*Face]bool)
	usedImages := make(map[*Imagefile]bool)
	// Write all page streams:
	for _, page := range pw.pages.pages {
		for _, img := range page.Images {
			usedImages[img] = true
		}
		page.Obj, err = pw.writeStream(page.stream, page.Obj)
		if err != nil {
			return 0, err
		}
	}

	// Page streams are finished. Now the /Page dictionaries with
	// references to the streams and the parent
	// Pages objects have to be placed in the file

	//  We need to know in advance where the parent object is written (/Pages)
	pagesObj := pw.NewObject()
	// write out all images to the PDF
	for img := range usedImages {
		img.finish()
	}

	if len(pw.pages.pages) == 0 {
		return 0, fmt.Errorf("no pages in document")
	}
	for _, page := range pw.pages.pages {
		obj := pw.NewObjectWithNumber(page.Dictnum)

		var res []string
		if len(page.Faces) > 0 {
			res = append(res, "<< ")
			for _, face := range page.Faces {
				res = append(res, fmt.Sprintf("%s %s", face.InternalName(), face.fontobject.ObjectNumber.Ref()))
			}
			res = append(res, " >>")
		}

		resHash := Dict{}
		if len(page.Faces) > 0 {
			for _, face := range page.Faces {
				usedFaces[face] = true
			}
			resHash["/Font"] = strings.Join(res, " ")
		}
		if len(pw.Colorspaces) > 0 {
			colorspace := Dict{}

			for _, cs := range pw.Colorspaces {
				colorspace[cs.ID] = cs.Obj.String()
			}
			resHash["/ColorSpace"] = HashToString(colorspace, 0)
		}
		if len(page.Images) > 0 {
			var sb strings.Builder
			sb.WriteString("<<")
			for _, img := range page.Images {
				sb.WriteRune(' ')
				sb.WriteString(img.InternalName())
				sb.WriteRune(' ')
				sb.WriteString(img.imageobject.ObjectNumber.Ref())
			}
			sb.WriteString(">>")
			resHash["/XObject"] = sb.String()
		}

		pageHash := Dict{
			"/Type":     "/Page",
			"/Contents": page.Obj.ObjectNumber.Ref(),
			"/Parent":   pagesObj.ObjectNumber.Ref(),
			"/MediaBox": fmt.Sprintf("[0 0 %s %s]", page.Width, page.Height),
		}
		// do we really need the procset?
		// resHash["/ProcSet"] = "[ /PDF /Text /ImageB /ImageC /ImageI ]"
		// resHash["/ExtGState"] = "<< >> "

		if len(resHash) > 0 {
			pageHash["/Resources"] = HashToString(resHash, 1)
		}

		annotationObjectNumbers := make([]string, len(page.Annotations))
		for i, annot := range page.Annotations {
			annotObj := pw.NewObject()
			actionDict := Dict{
				"/Type": "/Action",
				"/S":    "/URI",
				"/URI":  "(" + annot.URI + ")",
			}
			annotDict := Dict{
				"/Type":    "/Annot",
				"/Subtype": "/Link",
				"/A":       HashToString(actionDict, 1),
				"/Rect":    fmt.Sprintf("[%s %s %s %s]", annot.Rect[0], annot.Rect[1], annot.Rect[2], annot.Rect[3]),
			}
			annotObj.Dict(annotDict)
			if err := annotObj.Save(); err != nil {
				return 0, err
			}
			annotationObjectNumbers[i] = annotObj.ObjectNumber.Ref()
		}
		if len(annotationObjectNumbers) > 0 {
			pageHash["/Annots"] = "[" + strings.Join(annotationObjectNumbers, " ") + "]"
		}
		for k, v := range page.Dict {
			pageHash[k] = v
		}
		obj.Dict(pageHash)
		obj.Save()
	}

	// The pages object
	kids := make([]string, len(pw.pages.pages))
	for i, v := range pw.pages.pages {
		kids[i] = v.Dictnum.Ref()
	}

	pagesObj.comment = "The pages object"
	pw.pages.dictnum = pagesObj.ObjectNumber
	pagesObj.Dict(Dict{
		"/Type":     "/Pages",
		"/Kids":     "[ " + strings.Join(kids, " ") + " ]",
		"/Count":    fmt.Sprint(len(pw.pages.pages)),
		"/MediaBox": fmt.Sprintf("[0 0 %s %s]", pw.DefaultPageWidth.String(), pw.DefaultPageHeight.String()),
	})
	pagesObj.Save()

	catalog := pw.NewObject()
	catalog.comment = "Catalog"
	dictCatalog := Dict{
		"/Type":  "/Catalog",
		"/Pages": pw.pages.dictnum.Ref(),
	}
	for k, v := range pw.Catalog {
		dictCatalog[k] = v
	}
	catalog.Dict(dictCatalog)
	catalog.Save()

	// write out all font descriptors and files into the PDF
	for fnt := range usedFaces {
		fnt.finish()
	}

	return catalog.ObjectNumber, nil
}

// Finish writes the trailer and xref section but does not close the file.
func (pw *PDF) Finish() error {
	dc, err := pw.writeDocumentCatalogAndPages()
	if err != nil {
		return err
	}
	// XRef section
	type chunk struct {
		startOnum Objectnumber
		positions []int64
	}
	objectChunks := []chunk{}
	var curchunk *chunk
	for i := Objectnumber(1); i <= pw.nextobject; i++ {
		if loc, ok := pw.objectlocations[i]; ok {
			if curchunk == nil {
				curchunk = &chunk{
					startOnum: i,
				}
			}
			curchunk.positions = append(curchunk.positions, loc)
		} else {
			objectChunks = append(objectChunks, *curchunk)
			curchunk = nil
		}
	}
	var str strings.Builder

	for _, chunk := range objectChunks {
		if chunk.startOnum == 1 {
			fmt.Fprintf(&str, "0 %d\n", len(chunk.positions)+1)
			fmt.Fprintln(&str, "0000000000 65535 f ")
		} else {
			fmt.Fprintf(&str, "%d %d\n", chunk.startOnum, len(chunk.positions))
		}
		for _, pos := range chunk.positions {
			fmt.Fprintf(&str, "%010d 00000 n \n", pos)

		}
	}

	xrefpos := pw.pos
	pw.Println("xref")
	pw.Print(str.String())
	sum := fmt.Sprintf("%X", md5.Sum([]byte(str.String())))

	trailer := Dict{
		"/Size": fmt.Sprint(int(pw.nextobject)),
		"/Root": dc.Ref(),
		"/ID":   fmt.Sprintf("[<%s> <%s>]", sum, sum),
	}
	if err = pw.Println("trailer"); err != nil {
		return err
	}

	pw.outHash(trailer)

	if err = pw.Printf("\nstartxref\n%d\n%%%%EOF\n", xrefpos); err != nil {
		return err
	}

	return nil
}

// Size returns the current size of the PDF file.
func (pw *PDF) Size() int64 {
	return pw.pos
}

// HashToString converts a PDF dictionary to a string including the paired angle
// brackets (<< ... >>).
func HashToString(h Dict, level int) string {
	var b bytes.Buffer
	b.WriteString(strings.Repeat("  ", level))
	b.WriteString("<<\n")
	for k, v := range h {
		b.WriteString(fmt.Sprintf("%s%s %s\n", strings.Repeat("  ", level+1), k, v))
	}
	b.WriteString(strings.Repeat("  ", level))
	b.WriteString(">>")
	return b.String()
}

func (pw *PDF) outHash(h Dict) {
	pw.Printf(HashToString(h, 0))
}

// Write an end of line (EOL) marker to the file if it is not on a EOL already.
func (pw *PDF) eol() {
	if pw.pos != pw.lastEOL {
		pw.Println("")
		pw.lastEOL = pw.pos
	}
}

// Write a start object marker with the next free object.
func (pw *PDF) startObject(onum Objectnumber) error {
	var position int64
	position = pw.pos + 1
	pw.objectlocations[onum] = position
	pw.Printf("\n%d 0 obj\n", onum)
	return nil
}

// Write a simple "endobj" to the PDF file. Return the object number.
func (pw *PDF) endObject() Objectnumber {
	onum := pw.nextobject
	pw.eol()
	pw.Println("endobj")
	return onum
}

// ImportImage writes an Image to the PDF
func (pw *PDF) ImportImage(imp *gofpdi.Importer, pagenumber int) (Objectnumber, string) {
	firstObj := pw.NewObject()
	imp.SetNextObjectID(int(firstObj.ObjectNumber))
	if pagenumber == 0 {
		pagenumber = 1
	}
	for i, str := range imp.GetImportedObjects() {
		bb := bytes.NewBuffer(str)
		if i == 1 {
			firstObj.Data = bb
			firstObj.Save()
		} else {
			obj := pw.NewObject()
			obj.Data = bb
			obj.Save()
		}
	}
	return firstObj.ObjectNumber, "/Im1"
}
