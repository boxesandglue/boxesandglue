package pdf

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"strings"

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
type Dict map[Name]any

func (d Dict) String() string {
	return HashToString(d, 0)
}

// Array is a list of anything
type Array []any

func (ary Array) String() string {
	return ArrayToString(ary)
}

// Name represents a PDF name such as Adobe Green. The String() method prepends
// a / (slash) to the name if not present.
type Name string

func (n Name) String() string {
	a := strings.NewReplacer(" ", "#20")
	r, _ := strings.CutPrefix(string(n), "/")
	return "/" + a.Replace(string(r))
}

// Pages is the parent page structure
type Pages struct {
	Pages   []*Page
	dictnum Objectnumber
}

// An Annotation is a PDF element that is additional to the text, such as a
// hyperlink or a note.
type Annotation struct {
	Subtype    Name
	Action     string
	Dictionary Dict
	Rect       [4]float64 // x1, y1, x2, y2
}

// Separation represents a spot color
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
	Dictnum       Objectnumber // The "/Page" object
	Annotations   []Annotation
	Faces         []*Face
	Images        []*Imagefile
	Width         float64
	Height        float64
	Dict          Dict // Additional dictionary entries such as "/Trimbox"
	contentStream *Object
}

// Outline represents PDF bookmarks. To create outlines, you need to assign
// previously created Dest items to the outline. When Open is true, the PDF
// viewer shows the child outlines.
type Outline struct {
	Children     []*Outline
	Title        string
	Open         bool
	Dest         string
	objectNumber Objectnumber
}

// Logger logs font loading and writing.
type Logger interface {
	Infof(string, ...any)
}

// PDF is the central point of writing a PDF file.
type PDF struct {
	Catalog           Dict
	InfoDict          Dict
	DefaultPageWidth  float64
	DefaultPageHeight float64
	Colorspaces       []*Separation
	NumDestinations   map[int]*NumDest
	NameDestinations  map[string]*NameDest
	Outlines          []*Outline
	Major             uint // Major version. Should be 1.
	Minor             uint // Minor version. Just for information purposes. No checks are done.
	Logger            Logger
	outfile           io.Writer
	nextobject        Objectnumber
	objectlocations   map[Objectnumber]int64
	pages             *Pages
	lastEOL           int64
	pos               int64
}

// NewPDFWriter creates a PDF file for writing to file
func NewPDFWriter(file io.Writer) *PDF {
	pw := PDF{
		Major:            1,
		Minor:            7,
		NumDestinations:  make(map[int]*NumDest),
		NameDestinations: make(map[string]*NameDest),
	}
	pw.outfile = file
	pw.nextobject = 1
	pw.objectlocations = make(map[Objectnumber]int64)
	pw.pages = &Pages{}
	pw.Printf("%%PDF-%d.%d", pw.Major, pw.Minor)
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

// Printf writes the formatted string to the PDF file.
func (pw *PDF) Printf(format string, a ...any) error {
	n, err := fmt.Fprintf(pw.outfile, format, a...)
	pw.pos += int64(n)
	return err
}

// AddPage adds a page to the PDF file. The content stream must be complete.
func (pw *PDF) AddPage(content *Object, dictnum Objectnumber) *Page {
	pg := &Page{}
	pg.contentStream = content
	pg.Dictnum = dictnum
	pw.pages.Pages = append(pw.pages.Pages, pg)
	return pg
}

// NextObject returns the next free object number
func (pw *PDF) NextObject() Objectnumber {
	pw.nextobject++
	return pw.nextobject - 1
}

func (pw *PDF) writeInfoDict() (*Object, error) {
	if pw.Major < 2 {
		info := pw.NewObject()
		info.Dictionary = pw.InfoDict
		info.Save()
		return info, nil
	}
	return nil, nil
}

func (pw *PDF) writeDocumentCatalogAndPages() (Objectnumber, error) {
	var err error
	usedFaces := make(map[*Face]bool)
	usedImages := make(map[*Imagefile]bool)
	// Write all page streams:
	for _, page := range pw.pages.Pages {
		for _, img := range page.Images {
			usedImages[img] = true
		}
		if err = page.contentStream.Save(); err != nil {
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

	if len(pw.pages.Pages) == 0 {
		return 0, fmt.Errorf("no pages in document")
	}
	for _, page := range pw.pages.Pages {
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
			resHash["Font"] = strings.Join(res, " ")
		}
		if len(pw.Colorspaces) > 0 {
			colorspace := Dict{}

			for _, cs := range pw.Colorspaces {
				colorspace[Name(cs.ID)] = cs.Obj.String()
			}
			resHash["ColorSpace"] = colorspace
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
			resHash["XObject"] = sb.String()
		}

		pageHash := Dict{
			"Type":     "/Page",
			"Contents": page.contentStream.ObjectNumber.Ref(),
			"Parent":   pagesObj.ObjectNumber.Ref(),
			"MediaBox": fmt.Sprintf("[0 0 %s %s]", FloatToPoint(page.Width), FloatToPoint(page.Height)),
		}

		if len(resHash) > 0 {
			pageHash["Resources"] = resHash
		}

		annotationObjectNumbers := make([]string, len(page.Annotations))
		for i, annot := range page.Annotations {
			annotObj := pw.NewObject()
			annotDict := Dict{
				"Type":    "/Annot",
				"Subtype": annot.Subtype.String(),
				"A":       annot.Action,
				"Rect":    fmt.Sprintf("[%s %s %s %s]", FloatToPoint(annot.Rect[0]), FloatToPoint(annot.Rect[1]), FloatToPoint(annot.Rect[2]), FloatToPoint(annot.Rect[3])),
			}
			for k, v := range annot.Dictionary {
				annotDict[k] = v
			}

			annotObj.Dict(annotDict)
			if err := annotObj.Save(); err != nil {
				return 0, err
			}
			annotationObjectNumbers[i] = annotObj.ObjectNumber.Ref()
		}
		if len(annotationObjectNumbers) > 0 {
			pageHash["Annots"] = "[" + strings.Join(annotationObjectNumbers, " ") + "]"
		}
		for k, v := range page.Dict {
			pageHash[k] = v
		}
		obj.Dict(pageHash)
		obj.Save()
	}

	// The pages object
	kids := make([]string, len(pw.pages.Pages))
	for i, v := range pw.pages.Pages {
		kids[i] = v.Dictnum.Ref()
	}

	pagesObj.comment = "The pages object"
	pw.pages.dictnum = pagesObj.ObjectNumber
	pagesObj.Dict(Dict{
		"Type":     "/Pages",
		"Kids":     "[ " + strings.Join(kids, " ") + " ]",
		"Count":    fmt.Sprint(len(pw.pages.Pages)),
		"MediaBox": fmt.Sprintf("[0 0 %s %s]", FloatToPoint(pw.DefaultPageWidth), FloatToPoint(pw.DefaultPageHeight)),
	})
	pagesObj.Save()

	// outlines
	var outlinesOjbNum Objectnumber

	if pw.Outlines != nil {
		outlinesOjb := pw.NewObject()
		first, last, count, err := pw.writeOutline(outlinesOjb, pw.Outlines)
		if err != nil {
			return 0, err
		}

		outlinesOjb.Dictionary = Dict{
			"Type":  "/Outlines",
			"First": first.Ref(),
			"Last":  last.Ref(),
			"Count": fmt.Sprintf("%d", count),
		}
		outlinesOjbNum = outlinesOjb.ObjectNumber

		if err = outlinesOjb.Save(); err != nil {
			return 0, err
		}
	}

	catalog := pw.NewObject()
	catalog.comment = "Catalog"
	dictCatalog := Dict{
		"Type":  "/Catalog",
		"Pages": pw.pages.dictnum.Ref(),
	}
	if pw.Outlines != nil {
		dictCatalog["/Outlines"] = outlinesOjbNum.Ref()
	}

	var destNameTree *Object
	if len(pw.NameDestinations) != 0 {
		type name struct {
			onum Objectnumber
			name String
		}
		destnames := []name{}
		for _, nd := range pw.NameDestinations {
			nd.objectnumber, err = pw.writeDestObj(nd.PageObjectnumber, nd.X, nd.Y)
			if err != nil {
				return 0, err
			}
			destnames = append(destnames, name{name: nd.Name, onum: nd.objectnumber})
		}

		destNameTree = pw.NewObject()
		var limitsAry, namesAry Array
		limitsAry = append(limitsAry, destnames[0].name)
		limitsAry = append(limitsAry, destnames[len(destnames)-1].name)
		for _, n := range destnames {
			namesAry = append(namesAry, String(n.name))
			namesAry = append(namesAry, n.onum.Ref())
		}

		destNameTree.Dict(Dict{
			"Limits": limitsAry.String(),
			"Names":  namesAry.String(),
		})
		destNameTree.Save()
	}

	if destNameTree != nil {
		d := Dict{
			"Dests": destNameTree.ObjectNumber.Ref(),
		}
		nameDict := pw.NewObject()
		nameDict.Dict(d)
		if err = nameDict.Save(); err != nil {
			return 0, err
		}
		dictCatalog["Names"] = nameDict.ObjectNumber.Ref()
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

func (pw *PDF) writeDestObj(page Objectnumber, x, y float64) (Objectnumber, error) {
	obj := pw.NewObject()
	dest := fmt.Sprintf("[%s /XYZ %g %g null]", page.Ref(), x, y)
	obj.Dict(Dict{
		"D": dest,
	})

	if err := obj.Save(); err != nil {
		return 0, err
	}
	return obj.ObjectNumber, nil

}

func (pw *PDF) writeOutline(parentObj *Object, outlines []*Outline) (first Objectnumber, last Objectnumber, c int, err error) {
	for _, outline := range outlines {
		outline.objectNumber = pw.NextObject()
	}

	c = 0
	for i, outline := range outlines {
		c++
		outlineObj := pw.NewObjectWithNumber(outline.objectNumber)
		outlineDict := Dict{}
		outlineDict["Parent"] = parentObj.ObjectNumber.Ref()
		outlineDict["Title"] = StringToPDF(outline.Title)
		outlineDict["Dest"] = outline.Dest

		if i < len(outlines)-1 {
			outlineDict["Next"] = outlines[i+1].objectNumber.Ref()
		} else {
			last = outline.objectNumber
		}
		if i > 0 {
			outlineDict["Prev"] = outlines[i-1].objectNumber.Ref()
		} else {
			first = outline.objectNumber
		}

		if len(outline.Children) > 0 {
			var cldFirst, cldLast Objectnumber
			var count int
			cldFirst, cldLast, count, err = pw.writeOutline(outlineObj, outline.Children)
			if err != nil {
				return
			}
			outlineDict["First"] = cldFirst.Ref()
			outlineDict["Last"] = cldLast.Ref()
			if outline.Open {
				outlineDict["Count"] = fmt.Sprintf("%d", count)
			} else {
				outlineDict["Count"] = "-1"
			}
			c += count
		}
		outlineObj.Dictionary = outlineDict
		outlineObj.Save()
	}
	return
}

// Finish writes the trailer and xref section but does not close the file.
func (pw *PDF) Finish() error {
	dc, err := pw.writeDocumentCatalogAndPages()
	if err != nil {
		return err
	}

	infodict, err := pw.writeInfoDict()
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
		"Size": fmt.Sprint(int(pw.nextobject)),
		"Root": dc.Ref(),
		"ID":   fmt.Sprintf("[<%s> <%s>]", sum, sum),
	}
	if infodict != nil {
		trailer["Info"] = infodict.ObjectNumber.Ref()
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
		b.WriteString(fmt.Sprintf("%s%s %v\n", strings.Repeat("  ", level+1), k, v))
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
