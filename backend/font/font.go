package font

import (
	"unicode"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/pdfbackend/pdf"
	"github.com/speedata/textlayout/fonts"
	"github.com/speedata/textlayout/harfbuzz"
)

// An Atom contains size information about the glyphs as a result of Shape
type Atom struct {
	Advance    bag.ScaledPoint
	Height     bag.ScaledPoint
	Depth      bag.ScaledPoint
	IsSpace    bool
	Components string
	Codepoint  int
	Hyphenate  bool
	Kernafter  bag.ScaledPoint
}

// Font is the main structure of a font instance
type Font struct {
	Space        bag.ScaledPoint
	SpaceStretch bag.ScaledPoint
	SpaceShrink  bag.ScaledPoint
	Size         bag.ScaledPoint
	Depth        bag.ScaledPoint
	Face         *pdf.Face
	Hyphenchar   Atom
	Mag          int
}

// NewFont creates a new font instance.
func NewFont(face *pdf.Face, size bag.ScaledPoint) *Font {
	f := face.Font.Face()
	factor := 100 / (float64(f.AscenderPDF()) + float64(f.DescenderPDF()*-1))
	mag := int(size) / int(face.UnitsPerEM)
	fnt := &Font{
		Space:        size * 333 / 1000,
		SpaceStretch: size * 167 / 1000,
		SpaceShrink:  size * 111 / 1000,
		Size:         size,
		Face:         face,
		Mag:          mag,
		Depth:        size * bag.ScaledPoint(float64(-1*f.DescenderPDF())*factor) / 100,
	}
	atoms := fnt.Shape("-", []harfbuzz.Feature{})
	if len(atoms) == 1 {
		fnt.Hyphenchar = atoms[0]
	}
	return fnt
}

// Shape transforms the text into a slice of code points.
func (f *Font) Shape(text string, features []harfbuzz.Feature) []Atom {
	buf := harfbuzz.NewBuffer()
	buf.AddRunes([]rune(text), 0, -1)
	buf.Flags = harfbuzz.RemoveDefaultIgnorables

	ha := f.Face.Font.Face().HorizontalAdvance

	buf.GuessSegmentProperties()
	buf.Shape(f.Face.Font, features)
	runes := []rune(text)
	glyphs := make([]Atom, 0, len(buf.Info))
	space := f.Face.Codepoint(' ')
	for i, r := range buf.Info {
		char := runes[r.Cluster]
		if unicode.IsSpace(char) {
			glyphs = append(glyphs, Atom{
				IsSpace:    true,
				Advance:    f.Size,
				Components: " ",
			})
		} else {
			var bdelta bag.ScaledPoint
			adv := buf.Pos[i].XAdvance
			advanceCalculated := adv * int32(f.Mag)
			advanceWant := ha(r.Glyph) * float32(f.Mag)

			// only add kern if the next item is not a space
			if i < len(buf.Info)-1 {
				if buf.Info[i+1].Glyph != space {
					bdelta = bag.ScaledPoint(float32(advanceCalculated) - advanceWant)
				}
			}
			glyphs = append(glyphs, Atom{
				Advance:    bag.ScaledPoint(advanceWant),
				Height:     f.Size - f.Depth,
				Depth:      f.Depth,
				Hyphenate:  unicode.IsLetter(char),
				Components: string(char),
				Codepoint:  int(r.Glyph),
				Kernafter:  bdelta,
			})
		}
	}
	return glyphs
}

// AdvanceX returns the advance in horizontal direction
func (f *Font) AdvanceX(r rune) (bag.ScaledPoint, error) {
	idx, err := f.Face.GetIndex(r)
	if err != nil {
		return 0, err
	}
	adv := f.Face.Font.Face().HorizontalAdvance(fonts.GID(idx))
	if err != nil {
		return 0, err
	}
	wd := bag.ScaledPoint(adv) * bag.ScaledPoint(f.Mag)
	return wd, nil
}
