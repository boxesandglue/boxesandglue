package pdf

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

type objectnumber int

func (o objectnumber) ref() string {
	return fmt.Sprintf("%d 0 R", o)
}

// Dict is a string - string dictionary
type Dict map[string]string

// PDF is the central point of writing a PDF file.
type PDF struct {
	outfile         io.WriteSeeker
	nextobject      objectnumber
	objectlocations map[objectnumber]int64
	pages           *Pages
	lastEOL         int64
	fonts           []*Font
}

// NewPDFWriter creates a PDF file for writing to file
func NewPDFWriter(file io.WriteSeeker) *PDF {
	pw := PDF{}
	pw.outfile = file
	pw.nextobject = 1
	pw.objectlocations = make(map[objectnumber]int64)
	pw.pages = &Pages{}
	pw.out("%PDF-1.7")
	return &pw
}

// Pages is the parent page structure
type Pages struct {
	pages   []*Page
	dictnum objectnumber
}

// Page contains information about a single page
type Page struct {
	onum    objectnumber
	dictnum objectnumber
	Fonts   []*Font
	stream  *Stream
}

// RegisterChars tells the PDF file which fonts are used on a page and which characters are included.
// The string r must include every used char in this font in any order at least once.
func (pg *Page) RegisterChars(fnt *Font, r string) {
	for _, v := range r {
		fnt.usedChar[v] = true
	}
}

// AddPage adds a page to the PDF file. The stream must be complete.
func (pw *PDF) AddPage(pagestream *Stream) *Page {
	pg := &Page{}
	pg.stream = pagestream
	pw.pages.pages = append(pw.pages.pages, pg)
	return pg
}

// Get next free object number
func (pw *PDF) nextObject() objectnumber {
	pw.nextobject++
	return pw.nextobject - 1
}

func (pw *PDF) writeStream(st *Stream) objectnumber {
	obj := pw.NewObject()
	st.dict["/Length"] = fmt.Sprintf("%d", len(st.data))
	obj.Dict(st.dict)
	obj.Data.WriteString("\nstream\n")
	obj.Data.Write(st.data)
	obj.Data.WriteString("\nendstream\n")
	obj.Save()
	return obj.ObjectNumber
}

func (pw *PDF) writeDocumentCatalog() (objectnumber, error) {
	// Write all page streams:
	for _, page := range pw.pages.pages {
		page.onum = pw.writeStream(page.stream)
	}

	// Page streams are finished. Now the /Page dictionaries with
	// references to the streams and the parent
	// Pages objects have to be placed in the file

	//  We need to know in advance where the parent object is written (/Pages)
	pagesObj := pw.NewObject()

	for _, page := range pw.pages.pages {
		obj := pw.NewObject()
		onum := obj.ObjectNumber
		page.dictnum = onum

		var res []string
		if len(page.Fonts) > 0 {
			res = append(res, "<< ")
			for _, fnt := range page.Fonts {
				res = append(res, fmt.Sprintf("%s %s", fnt.InternalName, fnt.fontobject.ObjectNumber.ref()))
			}
			res = append(res, " >>")
		}

		resHash := Dict{}
		if len(page.Fonts) > 0 {
			resHash["/Font"] = strings.Join(res, " ")
		}
		pageHash := Dict{
			"/Type":     "/Page",
			"/Contents": page.onum.ref(),
			"/Parent":   pagesObj.ObjectNumber.ref(),
		}

		resHash["/ProcSet"] = "[ /PDF /Text ]"
		if len(resHash) > 0 {
			pageHash["/Resources"] = hashToString(resHash, 1)
		}
		obj.Dict(pageHash)
		obj.Save()
	}

	// The pages object
	kids := make([]string, len(pw.pages.pages))
	for i, v := range pw.pages.pages {
		kids[i] = v.dictnum.ref()
	}

	fmt.Fprintln(pw.outfile, "%% The pages object")
	pw.pages.dictnum = pagesObj.ObjectNumber
	pagesObj.Dict(Dict{
		"/Type":  "/Pages",
		"/Kids":  "[ " + strings.Join(kids, " ") + " ]",
		"/Count": fmt.Sprint(len(pw.pages.pages)),
		// "/Resources": "<<  >>",
		"/MediaBox": "[0 0 612 792]",
	})
	pagesObj.Save()

	catalog := pw.NewObject()
	catalog.comment = "Catalog"
	catalog.Dict(Dict{
		"/Type":  "/Catalog",
		"/Pages": pw.pages.dictnum.ref(),
	})
	catalog.Save()

	// write out all font descriptors and files into the PDF
	for _, fnt := range pw.fonts {
		fnt.finish()
	}
	return catalog.ObjectNumber, nil
}

// Finish writes the trailer and xref section but does not close the file
func (pw *PDF) Finish() error {
	fmt.Println("Now finishing the PDF")
	dc, err := pw.writeDocumentCatalog()
	if err != nil {
		return err
	}

	// XRef section
	xrefpos := pw.curpos()
	pw.out("xref")
	pw.outf("0 %d\n", pw.nextobject)
	fmt.Fprintln(pw.outfile, "0000000000 65535 f ")
	for i := objectnumber(1); i < pw.nextobject; i++ {
		if loc, ok := pw.objectlocations[i]; ok {
			fmt.Fprintf(pw.outfile, "%010d 00000 n \n", loc)
		}
	}

	trailer := Dict{
		"/Size": fmt.Sprint(pw.nextobject),
		"/Root": dc.ref(),
		"/ID":   "[<72081BF410BDCCB959F83B2B25A355D7> <72081BF410BDCCB959F83B2B25A355D7>]",
	}
	fmt.Fprintln(pw.outfile, "trailer")
	pw.outHash(trailer)
	fmt.Fprintln(pw.outfile, "startxref")
	fmt.Fprintf(pw.outfile, "%d\n", xrefpos)
	eofmarker := "%%EOF"
	fmt.Fprintln(pw.outfile, eofmarker)
	return nil
}

func hashToString(h Dict, level int) string {
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
	pw.out(hashToString(h, 0))
}

// Write an end of line (EOL) marker to the file if it is not on a EOL already.
func (pw *PDF) eol() {
	if curpos := pw.curpos(); curpos != pw.lastEOL {
		fmt.Fprintln(pw.outfile, "")
		pw.lastEOL = curpos
	}
}

func (pw *PDF) out(str string) {
	fmt.Fprintln(pw.outfile, str)
	pw.lastEOL = pw.curpos()
}

// Write a formatted string to the PDF file
func (pw *PDF) outf(format string, str ...interface{}) {
	fmt.Fprintf(pw.outfile, format, str...)
}

// Return the current position in the PDF file. Panics if something is wrong.
func (pw *PDF) curpos() int64 {
	pos, err := pw.outfile.Seek(0, os.SEEK_CUR)
	if err != nil {
		panic(err)
	}
	return pos
}

// Write a start object marker with the next free object.
func (pw *PDF) startObject(onum objectnumber) error {
	var position int64
	position = pw.curpos() + 1
	pw.objectlocations[onum] = position
	pw.outf("\n%d 0 obj\n", onum)
	return nil
}

// Write a simple "endobj" to the PDF file. Return the object number.
func (pw *PDF) endObject() objectnumber {
	onum := pw.nextobject
	pw.eol()
	pw.out("endobj")
	return onum
}
