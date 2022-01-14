package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
)

// Object has information about a specific PDF object
type Object struct {
	ObjectNumber Objectnumber
	Data         *bytes.Buffer
	Dictionary   Dict
	pdfwriter    *PDF
	compress     bool // for streams
	comment      string
	Raw          bool // Data holds everything between object number and endobj
}

// NewObjectWithNumber create a new PDF object and reserves an object
// number for it.
// The object is not written to the PDF until Save() is called.
func (pw *PDF) NewObjectWithNumber(objnum Objectnumber) *Object {
	obj := &Object{
		Data: &bytes.Buffer{},
	}
	obj.ObjectNumber = objnum
	obj.pdfwriter = pw
	return obj
}

// NewObject create a new PDF object and reserves an object
// number for it.
// The object is not written to the PDF until Save() is called.
func (pw *PDF) NewObject() *Object {
	obj := &Object{
		Data: &bytes.Buffer{},
	}
	obj.ObjectNumber = pw.NextObject()
	obj.pdfwriter = pw
	return obj
}

// SetCompression turns on stream compression
func (obj *Object) SetCompression() {
	obj.compress = true
}

// Save adds the PDF object to the main PDF file.
func (obj *Object) Save() error {
	if obj.comment != "" {
		if err := obj.pdfwriter.Print("\n% " + obj.comment); err != nil {
			return err
		}
	}

	if obj.Raw {
		err := obj.pdfwriter.startObject(obj.ObjectNumber)
		if err != nil {
			return err
		}
		n, err := obj.Data.WriteTo(obj.pdfwriter.outfile)
		if err != nil {
			return err
		}
		obj.pdfwriter.pos += n
		obj.pdfwriter.endObject()
		return nil
	}
	hasData := obj.Data.Len() > 0
	if hasData && len(obj.Dictionary) > 0 {
		obj.Dictionary["/Length"] = fmt.Sprintf("%d", obj.Data.Len())

		if obj.compress {
			obj.Dictionary["/Filter"] = "/FlateDecode"
			var b bytes.Buffer
			zw, err := zlib.NewWriterLevel(&b, zlib.BestSpeed)
			if err != nil {
				return err
			}
			zw.Write(obj.Data.Bytes())
			zw.Close()
			obj.Dictionary["/Length"] = fmt.Sprintf("%d", b.Len())
			obj.Dictionary["/Length1"] = fmt.Sprintf("%d", obj.Data.Len())
			obj.Data = &b
		} else {
			obj.Dictionary["/Length"] = fmt.Sprintf("%d", obj.Data.Len())
		}

	}

	obj.pdfwriter.startObject(obj.ObjectNumber)
	n, err := fmt.Fprint(obj.pdfwriter.outfile, HashToString(obj.Dictionary, 0))
	if err != nil {
		return err
	}
	obj.pdfwriter.pos += int64(n)
	if obj.Data.Len() > 0 {
		n, err := fmt.Fprintln(obj.pdfwriter.outfile, "\nstream")
		if err != nil {
			return err
		}
		obj.pdfwriter.pos += int64(n)
		m, err := obj.Data.WriteTo(obj.pdfwriter.outfile)
		if err != nil {
			return err
		}
		obj.pdfwriter.pos += m
		n, err = fmt.Fprintln(obj.pdfwriter.outfile, "\nendstream")
		if err != nil {
			return err
		}
		obj.pdfwriter.pos += int64(n)
	}
	obj.pdfwriter.endObject()
	return nil
}

// Dict writes the dict d to a PDF object
func (obj *Object) Dict(d Dict) *Object {
	obj.Dictionary = d
	return obj
}
