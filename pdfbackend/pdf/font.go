package pdf

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/pdfbackend/fonts/truetype"
	"github.com/speedata/boxesandglue/pdfbackend/fonts/type1"
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

const (
	// FType1 represents a afm/pfm based font
	FType1 int = iota
	// FTrueType is a TrueType based OpenType font
	FTrueType
)

// FontSubsetter holds fonts that can be subsetted
type FontSubsetter interface {
	Subset([]rune) (string, error)
}

// Face holds information about a font file
type Face struct {
	fonttype     int
	usedChar     map[rune]bool
	InternalName string
	fontobject   *Object
	pw           *PDF
	formatobject FontSubsetter
}

// NewFace creates a new face object. A face is a represenation of a font file.
func (pw *PDF) NewFace(filename string) (*Face, error) {
	face := &Face{
		pw:           pw,
		usedChar:     make(map[rune]bool),
		InternalName: newInternalFontName(),
	}

	switch strings.ToLower(filepath.Ext(filename)) {
	case ".ttf":
		face.fonttype = FTrueType
		tt, err := truetype.LoadFace(filename)
		if err != nil {
			return nil, err
		}
		face.formatobject = tt
	case ".pfb":
		face.fonttype = FType1
		t1, err := type1.LoadFont(filename, "")
		if err != nil {
			return nil, err
		}
		face.formatobject = t1
	}

	return face, nil
}

// NewFont returns a font for the given size. One ScaledPoint is 1/0xffff DTP point
func (f *Face) NewFont(size bag.ScaledPoint) *Font {
	fnt := &Font{
		pw:   f.pw,
		face: f,
	}
	f.fontobject = fnt.pw.NewObject()
	f.pw.fonts = append(f.pw.fonts, fnt)
	return fnt
}

// RegisterChars marks the codepoints as used on the page. For font subsetting.
func (f *Face) RegisterChars(codepoint string) {
	// RegisterChars tells the PDF file which fonts are used on a page and which characters are included.
	// The string r must include every used char in this font in any order at least once.
	for _, v := range codepoint {
		f.usedChar[v] = true
	}
}

// Font is any kind of font for the PDF file (currently only type1 is supported)
type Font struct {
	pw       *PDF
	face     *Face
	FontFile objectnumber
	filename string
	data     []byte
}

// InternalName returns the font face internal name
func (f *Font) InternalName() string {
	return f.face.InternalName
}

func newInternalFontName() string {
	return fmt.Sprintf("/F%d", <-ids)
}

// Used for subsetting the fonts
type charSubset []rune

func (p charSubset) Len() int           { return len(p) }
func (p charSubset) Less(i, j int) bool { return p[i] < p[j] }
func (p charSubset) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// finish writes the font file to the PDF. The font should be sub-setted, therefore we know the requirements only the end of the PDF file.
func (f *Face) finish() error {
	bag.LogWithFields(bag.Fields{"id": f.InternalName}).Trace("Finish font face")
	switch fnt := f.formatobject.(type) {
	case *truetype.TrueType:
		fmt.Println(fnt)
	case *type1.Type1:
		subset := make(charSubset, len(f.usedChar))
		i := 0
		for g := range f.usedChar {
			subset[i] = g
			i++
		}
		sort.Sort(subset)
		charset, err := fnt.Subset(subset)
		if err != nil {
			return err
		}
		st := NewStream(bytes.Join(fnt.Segments, nil))
		st.dict = Dict{
			"/Length1": fmt.Sprintf("%d", len(fnt.Segments[0])),
			"/Length2": fmt.Sprintf("%d", len(fnt.Segments[1])),
			"/Length3": fmt.Sprintf("%d", len(fnt.Segments[2])),
		}
		// pw = PDFWriter
		pw := f.pw
		fontfileObjectNumber := pw.writeStream(st)

		fontdescriptor := pw.NewObject()
		fontdescriptor.Dict(Dict{
			"/Type":        "/FontDescriptor",
			"/FontName":    "/" + fnt.SubsetID + "+" + fnt.FontName,
			"/Flags":       "4",
			"/FontBBox":    fmt.Sprintf("[ %d %d %d %d ]", fnt.FontBBox[0], fnt.FontBBox[1], fnt.FontBBox[2], fnt.FontBBox[3]),
			"/ItalicAngle": fmt.Sprintf("%d", fnt.ItalicAngle),
			"/Ascent":      fmt.Sprintf("%d", fnt.Ascender),
			"/Descent":     fmt.Sprintf("%d", fnt.Descender),
			"/CapHeight":   fmt.Sprintf("%d", fnt.CapHeight),
			"/XHeight":     fmt.Sprintf("%d", fnt.XHeight),
			"/StemV":       fmt.Sprintf("%d", 0),
			"/FontFile":    fontfileObjectNumber.ref(),
			"/CharSet":     fmt.Sprintf("(%s)", charset),
		})
		fontdescriptor.Save()

		fontObj := f.fontobject

		widths := []string{"["}
		for i := subset[0]; i <= subset[len(subset)-1]; i++ {
			widths = append(widths, fmt.Sprintf("%d", fnt.CharsCodepoint[i].Wx))
		}
		widths = append(widths, "]")
		wd := strings.Join(widths, " ")
		fdict := Dict{
			"/Type":           "/Font",
			"/Subtype":        "/Type1",
			"/BaseFont":       "/" + fnt.SubsetID + "+" + fnt.FontName,
			"/FirstChar":      fmt.Sprintf("%d", subset[0]),
			"/LastChar":       fmt.Sprintf("%d", subset[len(subset)-1]),
			"/Widths":         wd,
			"/FontDescriptor": fontdescriptor.ObjectNumber.ref(),
		}
		fontObj.Dict(fdict)
		fontObj.Save()

	}
	return nil
}
