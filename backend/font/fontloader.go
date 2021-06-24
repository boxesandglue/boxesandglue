package font

import (
	"bytes"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/speedata/boxesandglue/backend/bag"
)

const (
	ppem    = 1024
	hinting = font.HintingFull
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

// Font is the main structure of a font instance
type Font struct {
	Space        bag.ScaledPoint
	SpaceStretch bag.ScaledPoint
	SpaceShrink  bag.ScaledPoint
	Size         bag.ScaledPoint
	Face         *Face
	mag          int
}

// Face represents a font structure with no specific size.
// To get the dimensions of a font, you need to create a Font object with a given size
type Face struct {
	FaceID       int
	UnitsPerEM   int32
	Height       bag.ScaledPoint
	filename     string
	sfntobj      *sfnt.Font
	sfntbuffer   sfnt.Buffer
	ToRune       map[sfnt.GlyphIndex]rune
	ToGlyphIndex map[rune]sfnt.GlyphIndex
}

var loadedFaces map[string]*Face

func init() {
	loadedFaces = make(map[string]*Face)
}

func fillFaceObject(id string, sf *sfnt.Font) (*Face, error) {
	face := Face{
		FaceID:       <-ids,
		UnitsPerEM:   int32(sf.UnitsPerEm()),
		Height:       0,
		filename:     id,
		sfntobj:      sf,
		sfntbuffer:   sfnt.Buffer{},
		ToRune:       make(map[sfnt.GlyphIndex]rune),
		ToGlyphIndex: make(map[rune]sfnt.GlyphIndex),
	}

	loadedFaces[id] = &face
	return &face, nil

}

// NewFaceFromData returns a Face object which is a representation of a font file.
// The first parameter (id) should be the file name of the font, but can be any string.
// This is to prevent duplicate font loading.
func NewFaceFromData(id string, data []byte) (*Face, error) {
	r := bytes.NewReader(data)
	sf, err := sfnt.ParseReaderAt(r)
	if err != nil {
		return nil, err
	}
	return fillFaceObject(id, sf)
}

func getFace(filename string) (*Face, error) {
	if f, ok := loadedFaces[filename]; ok {
		bag.LogTrace("Found face")
		return f, nil
	}
	bag.LogTrace("Create new face")
	r, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	sf, err := sfnt.ParseReaderAt(r)
	if err != nil {
		return nil, err
	}
	err = r.Close()
	if err != nil {
		return nil, err
	}

	return fillFaceObject(filename, sf)
}

// LoadFont loads a font from the harddrive
func LoadFont(filename string, size bag.ScaledPoint) (*Font, error) {
	bag.LogWithFields(bag.Fields{
		"font": filename,
		"size": size,
	}).Trace("Load font")
	// fmt.Printf("Load font %s size %sf\n", filename, size.String())
	face, err := getFace(filename)
	if err != nil {
		return nil, err
	}

	mag := int(size) / int(face.UnitsPerEM)
	return &Font{
		Space:        size,
		SpaceStretch: size / 3,
		SpaceShrink:  size / 10,
		Size:         size,
		Face:         face,
		mag:          mag,
	}, nil
}

// GetIndex returns the glyph index of the rune r
func (f *Face) GetIndex(r rune) (sfnt.GlyphIndex, error) {
	if idx, ok := f.ToGlyphIndex[r]; ok {
		return idx, nil
	}

	idx, err := f.sfntobj.GlyphIndex(&f.sfntbuffer, r)
	if err != nil {
		return 0, err
	}
	f.ToRune[idx] = r
	f.ToGlyphIndex[r] = idx
	return idx, nil
}

// AdvanceX returns the advance in horiontal direction
func (f *Font) AdvanceX(r rune) (bag.ScaledPoint, error) {
	idx, err := f.Face.GetIndex(r)
	if err != nil {
		return 0, err
	}

	adv, err := f.Face.sfntobj.GlyphAdvance(&f.Face.sfntbuffer, idx, fixed.Int26_6(f.Face.UnitsPerEM*10), hinting)
	if err != nil {
		return 0, err
	}

	return bag.ScaledPoint(int(adv)) / 10 * bag.ScaledPoint(f.mag), nil
}
