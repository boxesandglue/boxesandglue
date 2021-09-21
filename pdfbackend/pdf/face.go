package pdf

import (
	"bytes"
	"fmt"
	"os"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/gootf/opentype"
)

var (
	ids chan int
)

func genIntegerSequence(ids chan int) {
	i := int(0)
	for {
		ids <- i
		i++
	}
}

func init() {
	ids = make(chan int)
	go genIntegerSequence(ids)
}

// newInternalFontName returns a font name for the PDF such as /F1
func newInternalFontName() string {
	return fmt.Sprintf("/F%d", <-ids)
}

// Face represents a font structure with no specific size.
// To get the dimensions of a font, you need to create a Font object with a given size
type Face struct {
	FaceID       int
	Font         *opentype.Font
	UnitsPerEM   int32
	filename     string
	ToRune       map[int]rune
	ToGlyphIndex map[rune]int
	usedChar     map[int]bool
	FontFile     objectnumber
	fontobject   *Object
	pw           *PDF
}

// RegisterChars marks the codepoints as used on the page. For font subsetting.
func (face *Face) RegisterChars(codepoints []int) {
	// RegisterChars tells the PDF file which fonts are used on a page and which characters are included.
	// The slice must include every used char in this font in any order at least once.
	face.usedChar[0] = true
	for _, v := range codepoints {
		face.usedChar[v] = true
	}
}

// RegisterChar marks the codepoint as used on the page. For font subsetting.
func (face *Face) RegisterChar(codepoint int) {
	face.usedChar[0] = true
	face.usedChar[codepoint] = true
}

func fillFaceObject(id string, fnt *opentype.Font) (*Face, error) {
	err := fnt.ReadTables()
	if err != nil {
		return nil, err
	}
	face := Face{
		FaceID:       <-ids,
		UnitsPerEM:   int32(fnt.UnitsPerEM),
		Font:         fnt,
		filename:     id,
		ToRune:       make(map[int]rune),
		ToGlyphIndex: make(map[rune]int),
		usedChar:     make(map[int]bool),
	}

	return &face, nil
}

// NewFaceFromData returns a Face object which is a representation of a font file.
// The first parameter (id) should be the file name of the font, but can be any string.
// This is to prevent duplicate font loading.
func NewFaceFromData(id string, data []byte) (*Face, error) {
	r := bytes.NewReader(data)
	fnt, err := opentype.Open(r, 0)
	if err != nil {
		return nil, err
	}
	return fillFaceObject(id, fnt)
}

func getFace(filename string) (*Face, error) {
	bag.LogTrace("Create new face")
	r, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	fnt, err := opentype.Open(r, 0)
	if err != nil {
		return nil, err
	}

	return fillFaceObject(filename, fnt)
}

// LoadFace loads a font from the disc. The index specifies the sub font to be
// loaded.
func LoadFace(pw *PDF, filename string, idx int) (*Face, error) {
	bag.LogTrace("Create new face")
	r, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	fnt, err := opentype.Open(r, 0)
	if err != nil {
		return nil, err
	}
	f, err := fillFaceObject(filename, fnt)
	if err != nil {
		return nil, err
	}
	f.pw = pw
	f.fontobject = pw.NewObject()
	return f, nil
}

// GetIndex returns the glyph index of the rune r
func (face *Face) GetIndex(r rune) (int, error) {
	if idx, ok := face.ToGlyphIndex[r]; ok {
		return idx, nil
	}

	idx, err := face.Font.GetIndex(r)
	if err != nil {
		return 0, err
	}
	face.ToRune[idx] = r
	face.ToGlyphIndex[r] = idx
	return idx, nil
}

// InternalName returns a PDF usable name such as /F1
func (face *Face) InternalName() string {
	return fmt.Sprintf("/F%d", face.FaceID)
}

// Codepoints returns the internal code points for the runes.
func (face *Face) Codepoints(runes []rune) []int {
	return face.Codepoints(runes)
}

// finish writes the font file to the PDF. The font should be sub-setted,
// therefore we know the requirements only the end of the PDF file.
func (face *Face) finish() error {
	bag.LogTrace("finish face")
	var err error
	bag.LogWithFields(bag.Fields{"id": face.InternalName}).Trace("Finish font face")
	fnt := face.Font
	subset := make([]int, len(face.usedChar))
	i := 0
	for g := range face.usedChar {
		subset[i] = g
		i++
	}

	if err = fnt.Subset(subset); err != nil {
		return err
	}

	var w bytes.Buffer
	if err = fnt.WriteSubset(&w); err != nil {
		return err
	}

	b := w.Bytes()

	fontstream := NewStream(b)
	fontstream.SetCompression()
	if fnt.IsCFF {
		fontstream.dict["/Subtype"] = "/CIDFontType0C"
	}
	fontstreamOnum := face.pw.writeStream(fontstream)

	fontDescriptor := Dict{
		"/Type":        "/FontDescriptor",
		"/FontName":    fnt.PDFName(),
		"/FontBBox":    fnt.BoundingBox(),
		"/Ascent":      fmt.Sprintf("%d", fnt.Ascender()),
		"/Descent":     fmt.Sprintf("%d", fnt.Descender()),
		"/CapHeight":   fmt.Sprintf("%d", fnt.CapHeight()),
		"/Flags":       fmt.Sprintf("%d", fnt.Flags()),
		"/ItalicAngle": fmt.Sprintf("%d", fnt.ItalicAngle()),
		"/StemV":       fmt.Sprintf("%d", fnt.StemV()),
		"/XHeight":     fmt.Sprintf("%d", fnt.XHeight()),
	}
	if fnt.IsCFF {
		fontDescriptor["/FontFile3"] = fontstreamOnum.ref()
	} else {
		fontDescriptor["/FontFile2"] = fontstreamOnum.ref()
	}

	fontDescriptorObj := face.pw.NewObject()
	fontDescriptorObj.Dict(fontDescriptor).Save()

	cmap := fnt.CMap()
	cmapStream := NewStream([]byte(cmap))
	cmapOnum := face.pw.writeStream(cmapStream)

	cidFontType2 := Dict{
		"/BaseFont":       fnt.PDFName(),
		"/CIDSystemInfo":  `<< /Ordering (Identity) /Registry (Adobe) /Supplement 0 >>`,
		"/FontDescriptor": fontDescriptorObj.ObjectNumber.ref(),
		"/Subtype":        "/CIDFontType2",
		"/Type":           "/Font",
		"/W":              fnt.Widths(),
	}

	if fnt.IsCFF {
		cidFontType2["/Subtype"] = "/CIDFontType0"
	} else {
		cidFontType2["/Subtype"] = "/CIDFontType2"
	}
	cidFontType2Obj := face.pw.NewObject()
	cidFontType2Obj.Dict(cidFontType2).Save()

	fontObj := face.fontobject
	fontObj.Dict(Dict{
		"/BaseFont":        fnt.PDFName(),
		"/DescendantFonts": fmt.Sprintf("[%s]", cidFontType2Obj.ObjectNumber.ref()),
		"/Encoding":        "/Identity-H",
		"/Subtype":         "/Type0",
		"/ToUnicode":       cmapOnum.ref(),
		"/Type":            "/Font",
	})
	fontObj.Save()
	return nil
}
