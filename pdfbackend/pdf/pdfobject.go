package pdf

import (
	"bytes"
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
func (obj *Object) Save() error {
	if obj.comment != "" {
		if err := obj.pdfwriter.Println("% " + obj.comment); err != nil {
			return err
		}
	}
	obj.pdfwriter.startObject(obj.ObjectNumber)
	n, err := obj.Data.WriteTo(obj.pdfwriter.outfile)
	if err != nil {
		return err
	}
	obj.pdfwriter.pos += n
	obj.pdfwriter.endObject()
	return nil
}

// Dict writes the dict d to a PDF object
func (obj *Object) Dict(d Dict) (*Object, error) {
	_, err := obj.Data.WriteString(hashToString(d, 0))
	if err != nil {
		return nil, err
	}
	return obj, nil
}
