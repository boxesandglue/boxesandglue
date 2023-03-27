package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

var pdfStringReplacer = strings.NewReplacer(`(`, `\(`, `)`, `\)`, `\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`, "\b", `\b`, "\t", `\t`)

// NumDest represents a simple PDF destination. The origin of X and Y are in the
// top left corner and expressed in DTP points.
type NumDest struct {
	PageObjectnumber Objectnumber
	Num              int
	X                float64
	Y                float64
	objectnumber     Objectnumber
}

// NameDest represents a named PDF destination. The origin of X and Y are in the
// top left corner and expressed in DTP points.
type NameDest struct {
	PageObjectnumber Objectnumber
	Name             String
	X                float64
	Y                float64
	objectnumber     Objectnumber
}

// String is a string that gets automatically converted to (...) or
// hexadecimal form when placed in the PDF.
type String string

// StringToPDF returns an escaped string suitable to be used as a PDF object.
func StringToPDF(str string) string {
	isASCII := true
	for _, g := range str {
		if g > 127 {
			isASCII = false
			break
		}
	}
	var out strings.Builder
	if isASCII {
		out.WriteRune('(')
		out.WriteString(pdfStringReplacer.Replace(str))
		out.WriteRune(')')
		return out.String()
	}
	out.WriteString("<feff")
	for _, i := range utf16.Encode([]rune(str)) {
		out.WriteString(fmt.Sprintf("%04x", i))
	}
	out.WriteRune('>')
	return out.String()
}

func (s String) String() string {
	return StringToPDF(string(s))
}

// ArrayToString converts the objects in ary to a string including the opening and closing bracket
func ArrayToString(ary []any) string {
	ret := []string{"["}
	for _, elt := range ary {
		switch t := elt.(type) {
		case string:
			ret = append(ret, t)
		case Array:
			ret = append(ret, ArrayToString(t))
		case int:
			ret = append(ret, fmt.Sprintf("%d", t))
		case float64:
			ret = append(ret, strconv.FormatFloat(t, 'f', -1, 64))
		case Dict:
			ret = append(ret, fmt.Sprintf("%s", t))
		case String:
			ret = append(ret, fmt.Sprintf("%s", t))
		default:
			ret = append(ret, fmt.Sprintf("%s", t))
		}
	}
	ret = append(ret, "]")
	return strings.Join(ret, " ")
}

// FloatToPoint returns a string suitable as a PDF size value.
func FloatToPoint(in float64) string {
	const precisionFactor = 100.0
	rounded := math.Round(precisionFactor*in) / precisionFactor
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}

// Object has information about a specific PDF object
type Object struct {
	ObjectNumber Objectnumber
	Data         *bytes.Buffer
	Dictionary   Dict
	Array        []any
	Raw          bool // Data holds everything between object number and endobj
	pdfwriter    *PDF
	compress     bool // for streams
	comment      string
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

// SetCompression turns on stream compression if compresslevel > 0
func (obj *Object) SetCompression(compresslevel uint) {
	obj.compress = compresslevel > 0
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
	if hasData {
		if obj.Dictionary == nil {
			obj.Dictionary = Dict{}
		}
		obj.Dictionary["Length"] = fmt.Sprintf("%d", obj.Data.Len())

		if obj.compress {
			obj.Dictionary["Filter"] = "/FlateDecode"
			var b bytes.Buffer
			zw, err := zlib.NewWriterLevel(&b, zlib.BestSpeed)
			if err != nil {
				return err
			}
			zw.Write(obj.Data.Bytes())
			zw.Close()
			obj.Dictionary["Length"] = fmt.Sprintf("%d", b.Len())
			obj.Dictionary["Length1"] = fmt.Sprintf("%d", obj.Data.Len())
			obj.Data = &b
		} else {
			obj.Dictionary["Length"] = fmt.Sprintf("%d", obj.Data.Len())
		}
	}

	obj.pdfwriter.startObject(obj.ObjectNumber)
	if len(obj.Dictionary) > 0 {
		n, err := fmt.Fprint(obj.pdfwriter.outfile, HashToString(obj.Dictionary, 0))
		if err != nil {
			return err
		}
		obj.pdfwriter.pos += int64(n)
	} else if len(obj.Array) > 0 {
		n, err := fmt.Fprint(obj.pdfwriter.outfile, ArrayToString(obj.Array))
		if err != nil {
			return err
		}
		obj.pdfwriter.pos += int64(n)
	}
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
