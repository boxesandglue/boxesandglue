package pdf

import (
	"bytes"
	"fmt"
)

// Object has information about a specific PDF object
type Object struct {
	ObjectNumber objectnumber
	Data         bytes.Buffer
	pdfwriter    *PDF
	comment      string
}

// NewObject create a new PDF object and reserves an object
// number for it.
// The object is not written to the PDF until Save() is called.
func (pw *PDF) NewObject() *Object {
	obj := &Object{}
	obj.ObjectNumber = pw.nextObject()
	obj.pdfwriter = pw
	return obj
}

// Save adds the PDF object to the main PDF file.
func (obj *Object) Save() {
	if obj.comment != "" {
		fmt.Fprintln(obj.pdfwriter.outfile, "% "+obj.comment)
	}
	obj.pdfwriter.startObject(obj.ObjectNumber)
	obj.Data.WriteTo(obj.pdfwriter.outfile)
	obj.pdfwriter.endObject()
}

// Dict writes the dict d to a PDF object
func (obj *Object) Dict(d Dict) {
	obj.Data.WriteString(hashToString(d, 0))
}
