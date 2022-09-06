package pdf

import (
	"bytes"
	"fmt"
	"os"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/textlayout/fonts"
	"github.com/speedata/textlayout/fonts/truetype"
	"github.com/speedata/textlayout/harfbuzz"
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
	Font         *harfbuzz.Font
	UnitsPerEM   int32
	filename     string
	ToRune       map[fonts.GID]rune
	ToGlyphIndex map[rune]fonts.GID
	usedChar     map[int]bool
	cmap         fonts.Cmap
	FontFile     Objectnumber // TODO used?
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

func fillFaceObject(id string, fnt harfbuzz.Face) (*Face, error) {
	// err := fnt.ReadTables()
	// if err != nil {
	// 	return nil, err
	// }
	cm, _ := fnt.Cmap()
	face := Face{
		FaceID:       <-ids,
		UnitsPerEM:   int32(fnt.Upem()),
		Font:         harfbuzz.NewFont(fnt),
		filename:     id,
		ToRune:       make(map[fonts.GID]rune),
		ToGlyphIndex: make(map[rune]fonts.GID),
		usedChar:     make(map[int]bool),
		cmap:         cm,
	}

	return &face, nil
}

// NewFaceFromData returns a Face object which is a representation of a font file.
// The first parameter (id) should be the file name of the font, but can be any string.
// This is to prevent duplicate font loading.
func NewFaceFromData(id string, data []byte) (*Face, error) {
	r := bytes.NewReader(data)
	bag.Logger.Info("Read font")
	fnt, err := truetype.Load(r)
	if err != nil {
		return nil, err
	}
	firstface := fnt[0]
	return fillFaceObject(id, firstface)
}

// LoadFace loads a font from the disc. The index specifies the sub font to be
// loaded.
func LoadFace(pw *PDF, filename string, idx int) (*Face, error) {
	r, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	bag.Logger.Infof("Load font %s", filename)
	fnt, err := truetype.Load(r)
	if err != nil {
		return nil, err
	}
	firstface := fnt[0]

	f, err := fillFaceObject(filename, firstface)
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
		return int(idx), nil
	}
	cm := face.Codepoints([]rune{r})
	idx := fonts.GID(cm[0])
	face.ToRune[idx] = r
	face.ToGlyphIndex[r] = idx
	return int(idx), nil
}

// InternalName returns a PDF usable name such as /F1
func (face *Face) InternalName() string {
	return fmt.Sprintf("/F%d", face.FaceID)
}

// Codepoint tries to find the code point for r. If none found, 0 is returned.
func (face *Face) Codepoint(r rune) fonts.GID {
	if gid, ok := face.cmap.Lookup(r); ok {
		return gid
	}
	return 0
}

// Codepoints returns the internal code points for the runes.
func (face *Face) Codepoints(runes []rune) []int {
	ret := []int{}
	for _, r := range runes {
		if gid, ok := face.cmap.Lookup(r); ok {
			ret = append(ret, int(gid))
		}
	}
	return ret
}

// finish writes the font file to the PDF. The font should be subsetted,
// therefore we know the requirements only the end of the PDF file.
func (face *Face) finish() error {
	var err error
	bag.Logger.Infof("Write font %s to PDF", face.filename)
	fnt := face.Font.Face()
	subset := make([]fonts.GID, len(face.usedChar))
	i := 0
	for g := range face.usedChar {
		subset[i] = fonts.GID(g)
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
	fontstream.SetCompression(9)

	var isCFF bool
	if otf, ok := fnt.(*truetype.Font); ok {
		isCFF = otf.Type == truetype.TypeOpenType
	}

	if isCFF {
		fontstream.dict["/Subtype"] = "/CIDFontType0C"
	}
	fontstreamObj, err := face.pw.writeStream(fontstream, nil)
	if err != nil {
		return err
	}
	fontstreamOnum := fontstreamObj.ObjectNumber
	fontDescriptor := Dict{
		"/Type":        "/FontDescriptor",
		"/FontName":    fnt.NamePDF(),
		"/FontBBox":    fnt.BoundingBoxPDF(),
		"/Ascent":      fmt.Sprintf("%d", fnt.AscenderPDF()),
		"/Descent":     fmt.Sprintf("%d", fnt.DescenderPDF()),
		"/CapHeight":   fmt.Sprintf("%d", fnt.CapHeightPDF()),
		"/Flags":       fmt.Sprintf("%d", fnt.FlagsPDF()),
		"/ItalicAngle": fmt.Sprintf("%d", fnt.ItalicAnglePDF()),
		"/StemV":       fmt.Sprintf("%d", fnt.StemVPDF()),
		"/XHeight":     fmt.Sprintf("%d", fnt.XHeightPDF()),
	}
	if isCFF {
		fontDescriptor["/FontFile3"] = fontstreamOnum.Ref()
	} else {
		fontDescriptor["/FontFile2"] = fontstreamOnum.Ref()
	}

	fontDescriptorObj := face.pw.NewObject()
	fdd := fontDescriptorObj.Dict(fontDescriptor)
	fdd.Save()

	cmap := fnt.CMapPDF()
	cmapStream := NewStream([]byte(cmap))
	cmapObj, err := face.pw.writeStream(cmapStream, nil)
	if err != nil {
		return err
	}

	cidFontType2 := Dict{
		"/BaseFont":       fnt.NamePDF(),
		"/CIDSystemInfo":  `<< /Ordering (Identity) /Registry (Adobe) /Supplement 0 >>`,
		"/FontDescriptor": fontDescriptorObj.ObjectNumber.Ref(),
		"/Subtype":        "/CIDFontType2",
		"/Type":           "/Font",
		"/W":              fnt.WidthsPDF(),
		"/CIDToGIDMap":    "/Identity",
	}

	if isCFF {
		cidFontType2["/Subtype"] = "/CIDFontType0"
	} else {
		cidFontType2["/Subtype"] = "/CIDFontType2"
	}
	cidFontType2Obj := face.pw.NewObject()
	d := cidFontType2Obj.Dict(cidFontType2)
	d.Save()

	fontObj := face.fontobject
	fontObj.Dict(Dict{
		"/BaseFont":        fnt.NamePDF(),
		"/DescendantFonts": fmt.Sprintf("[%s]", cidFontType2Obj.ObjectNumber.Ref()),
		"/Encoding":        "/Identity-H",
		"/Subtype":         "/Type0",
		"/ToUnicode":       cmapObj.ObjectNumber.Ref(),
		"/Type":            "/Font",
	})
	fontObj.Save()
	return nil
}
