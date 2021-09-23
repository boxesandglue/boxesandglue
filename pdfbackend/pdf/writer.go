package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"strings"
)

type objectnumber int

func (o objectnumber) ref() string {
	return fmt.Sprintf("%d 0 R", o)
}

// Dict is a string - string dictionary
type Dict map[string]string

// Pages is the parent page structure
type Pages struct {
	pages   []*Page
	dictnum objectnumber
}

// Page contains information about a single page
type Page struct {
	onum    objectnumber
	dictnum objectnumber
	Faces   []*Face
	stream  *Stream
}

// PDF is the central point of writing a PDF file.
type PDF struct {
	outfile         io.Writer
	nextobject      objectnumber
	objectlocations map[objectnumber]int64
	pages           *Pages
	lastEOL         int64
	Faces           []*Face
	pos             int64
}

// NewPDFWriter creates a PDF file for writing to file
func NewPDFWriter(file io.Writer) *PDF {
	pw := PDF{}
	pw.outfile = file
	pw.nextobject = 1
	pw.objectlocations = make(map[objectnumber]int64)
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
	pg.stream = pagestream
	pw.pages.pages = append(pw.pages.pages, pg)
	return pg
}

// Get next free object number
func (pw *PDF) nextObject() objectnumber {
	pw.nextobject++
	return pw.nextobject - 1
}

// writeStream writes a new stream object with the data and extra dictionary entries. writeStream returns the object number of the stream.
func (pw *PDF) writeStream(st *Stream) (objectnumber, error) {
	obj := pw.NewObject()
	if st.compress {
		st.dict["/Filter"] = "/FlateDecode"
	}
	if st.compress {
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

	if _, err := obj.Data.WriteString("\nstream\n"); err != nil {
		return 0, err
	}

	if _, err = obj.Data.Write(st.data); err != nil {
		return 0, err
	}

	if _, err = obj.Data.WriteString("\nendstream"); err != nil {
		return 0, err
	}
	obj.Save()
	return obj.ObjectNumber, nil
}

func (pw *PDF) writeDocumentCatalog() (objectnumber, error) {
	var err error
	// Write all page streams:
	for _, page := range pw.pages.pages {
		page.onum, err = pw.writeStream(page.stream)
		if err != nil {
			return 0, err
		}
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
		if len(page.Faces) > 0 {
			res = append(res, "<< ")
			for _, face := range page.Faces {
				res = append(res, fmt.Sprintf("%s %s", face.InternalName(), face.fontobject.ObjectNumber.ref()))
			}
			res = append(res, " >>")
		}

		resHash := Dict{}
		if len(page.Faces) > 0 {
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

	if err := pw.Println("%% The pages object"); err != nil {
		return 0, err
	}

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
	for _, fnt := range pw.Faces {
		fnt.finish()
	}
	return catalog.ObjectNumber, nil
}

// Finish writes the trailer and xref section but does not close the file
func (pw *PDF) Finish() error {
	dc, err := pw.writeDocumentCatalog()
	if err != nil {
		return err
	}

	// XRef section
	xrefpos := pw.pos
	pw.Println("xref")
	pw.Printf("0 %d\n", pw.nextobject)
	n, err := fmt.Fprintln(pw.outfile, "0000000000 65535 f ")
	if err != nil {
		return err
	}
	pw.pos += int64(n)

	for i := objectnumber(1); i < pw.nextobject; i++ {
		if loc, ok := pw.objectlocations[i]; ok {
			err := pw.Printf("%010d 00000 n \n", loc)
			if err != nil {
				return err
			}
		}
	}

	trailer := Dict{
		"/Size": fmt.Sprint(pw.nextobject),
		"/Root": dc.ref(),
		"/ID":   "[<72081BF410BDCCB959F83B2B25A355D7> <72081BF410BDCCB959F83B2B25A355D7>]",
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
	pw.Printf(hashToString(h, 0))
}

// Write an end of line (EOL) marker to the file if it is not on a EOL already.
func (pw *PDF) eol() {
	if pw.pos != pw.lastEOL {
		pw.Println("")
		pw.lastEOL = pw.pos
	}
}

// Write a start object marker with the next free object.
func (pw *PDF) startObject(onum objectnumber) error {
	var position int64
	position = pw.pos + 1
	pw.objectlocations[onum] = position
	pw.Printf("\n%d 0 obj\n", onum)
	return nil
}

// Write a simple "endobj" to the PDF file. Return the object number.
func (pw *PDF) endObject() objectnumber {
	onum := pw.nextobject
	pw.eol()
	pw.Println("endobj")
	return onum
}
