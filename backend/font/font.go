package font

import (
	"unicode"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/textlayout/harfbuzz"
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
	SpaceChar    Atom
	Mag          int
}

// NewFont creates a new font instance.
func NewFont(face *pdf.Face, size bag.ScaledPoint) *Font {
	f := face.HarfbuzzFont.Face()
	ascend := float64(f.AscenderPDF())
	descend := float64(-1 * f.DescenderPDF())
	factor := size.ToPT() / (ascend + descend)
	fnt := &Font{
		Space:        size * 333 / 1000,
		SpaceStretch: size * 167 / 1000,
		SpaceShrink:  size * 111 / 1000,
		Size:         size,
		Face:         face,
		Mag:          int(size) / int(face.UnitsPerEM),
		Depth:        bag.ScaledPointFromFloat(factor * descend),
	}
	hyphenchar := fnt.Shape("-", []harfbuzz.Feature{})
	if len(hyphenchar) == 1 {
		fnt.Hyphenchar = hyphenchar[0]
	}
	spacechar := fnt.Shape(" ", []harfbuzz.Feature{})
	if len(spacechar) == 1 {
		fnt.SpaceChar = spacechar[0]
	}
	return fnt
}

// Shape transforms the text into a slice of code points.
func (f *Font) Shape(text string, features []harfbuzz.Feature) []Atom {
	// empty paragraphs have ZERO WIDTH SPACE as a marker
	if text == "\u200B" {
		return []Atom{
			{
				IsSpace:    true,
				Advance:    bag.ScaledPoint(0),
				Components: text,
				Codepoint:  f.SpaceChar.Codepoint,
			},
		}
	}
	runes := []rune(text)
	buf := harfbuzz.NewBuffer()
	buf.AddRunes(runes, 0, -1)
	buf.Flags = harfbuzz.RemoveDefaultIgnorables
	ha := f.Face.HarfbuzzFont.Face().HorizontalAdvance

	buf.GuessSegmentProperties()
	buf.Shape(f.Face.HarfbuzzFont, features)
	glyphs := make([]Atom, 0, len(buf.Info))
	space := f.Face.Codepoint(' ')
	lenBufInfo := len(buf.Info)
	for i, r := range buf.Info {
		char := runes[r.Cluster]
		adv := buf.Pos[i].XAdvance
		advanceCalculated := adv * int32(f.Mag)
		advanceWant := ha(r.Glyph) * float32(f.Mag)

		if unicode.IsSpace(char) {
			glyphs = append(glyphs, Atom{
				IsSpace:    true,
				Advance:    bag.ScaledPoint(advanceWant),
				Components: string(char),
				Codepoint:  int(r.Glyph),
			})
		} else {
			var bdelta bag.ScaledPoint

			// only add kern if the next item is not a space
			if i < len(buf.Info)-1 {
				if int(buf.Info[i+1].Glyph) != space {
					bdelta = bag.ScaledPoint(float32(advanceCalculated) - advanceWant)
				}
			}
			g := Atom{
				Advance:   bag.ScaledPoint(advanceWant),
				Height:    f.Size - f.Depth,
				Depth:     f.Depth,
				Hyphenate: unicode.IsLetter(char),
				Codepoint: int(r.Glyph),
				Kernafter: bdelta,
			}
			if i == lenBufInfo-1 {
				// last element
				g.Components = string(runes[r.Cluster:])
			} else {
				g.Components = string(runes[r.Cluster:buf.Info[i+1].Cluster])
			}
			glyphs = append(glyphs, g)
		}
	}
	return glyphs
}
