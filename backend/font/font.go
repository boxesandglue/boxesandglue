package font

import (
	"fmt"
	"unicode"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/pdfbackend/pdf"
)

// An Atom contains size information about the glyphs as a result of Shape
type Atom struct {
	Glyph      int
	Advance    bag.ScaledPoint
	Components string
	Codepoint  int
	Hyphenate  bool
}

// Font is the main structure of a font instance
type Font struct {
	Space        bag.ScaledPoint
	SpaceStretch bag.ScaledPoint
	SpaceShrink  bag.ScaledPoint
	Size         bag.ScaledPoint
	Face         *pdf.Face
	Mag          int
}

// Shape transforms the text into a slice of codepoints.
func (f *Font) Shape(text string) []Atom {
	glyphs := make([]Atom, 0, len(text))
	for _, r := range text {
		if unicode.IsSpace(r) {
			glyphs = append(glyphs, Atom{
				Glyph:      32,
				Advance:    f.Size,
				Components: " ",
			})
		} else {
			adv, err := f.AdvanceX(r)
			if err != nil {
				fmt.Println(err)
			}
			glyphs = append(glyphs, Atom{
				Glyph:      int(r),
				Advance:    adv,
				Hyphenate:  unicode.IsLetter(r),
				Components: string(r),
				Codepoint:  f.Face.ToGlyphIndex[r],
			})
		}
	}
	return glyphs
}

// AdvanceX returns the advance in horiontal direction
func (f *Font) AdvanceX(r rune) (bag.ScaledPoint, error) {
	idx, err := f.Face.GetIndex(r)
	if err != nil {
		return 0, err
	}
	adv, err := f.Face.Font.GlyphAdvance(idx)
	if err != nil {
		return 0, err
	}
	wd := bag.ScaledPoint(adv) * bag.ScaledPoint(f.Mag)
	return wd, nil
}
