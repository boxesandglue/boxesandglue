package font

import (
	"unicode"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/textshape/ot"
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
	f := face.OTFace()
	// Scale raw metrics to PDF units (1000 units per em)
	scale := 1000.0 / float64(f.Upem())
	ascend := float64(f.Ascender()) * scale
	descend := float64(-f.Descender()) * scale
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
	hyphenchar := fnt.Shape("-", nil, nil)
	if len(hyphenchar) == 1 {
		fnt.Hyphenchar = hyphenchar[0]
	}
	spacechar := fnt.Shape(" ", nil, nil)
	if len(spacechar) == 1 {
		fnt.SpaceChar = spacechar[0]
	}
	return fnt
}

// Shape transforms the text into a slice of code points.
// The variations parameter maps axis tags (e.g., "wght") to values.
func (f *Font) Shape(text string, features []ot.Feature, variations map[string]float64) []Atom {
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
	buf := ot.NewBuffer()
	buf.AddString(text)
	buf.Flags = ot.BufferFlagRemoveDefaultIgnorables
	face := f.Face.OTFace()

	// Apply variation settings to the shaper
	if len(variations) > 0 {
		bag.Logger.Debug("font.Shape applying variations", "variations", variations)
	}
	for tag, value := range variations {
		f.Face.Shaper.SetVariation(ot.MakeTag(tag[0], tag[1], tag[2], tag[3]), float32(value))
	}

	buf.GuessSegmentProperties()
	f.Face.Shaper.Shape(buf, features)
	glyphs := make([]Atom, 0, len(buf.Info))
	space := f.Face.Codepoint(' ')
	lenBufInfo := len(buf.Info)
	for i, r := range buf.Info {
		char := runes[r.Cluster]
		adv := buf.Pos[i].XAdvance
		advanceCalculated := int32(adv) * int32(f.Mag)
		advanceWant := float32(face.HorizontalAdvance(r.GlyphID)) * float32(f.Mag)

		if unicode.IsSpace(char) {
			glyphs = append(glyphs, Atom{
				IsSpace:    true,
				Advance:    bag.ScaledPoint(advanceWant),
				Components: string(char),
				Codepoint:  int(r.GlyphID),
			})
		} else {
			var bdelta bag.ScaledPoint

			// only add kern if the next item is not a space
			if i < len(buf.Info)-1 {
				if int(buf.Info[i+1].GlyphID) != space {
					bdelta = bag.ScaledPoint(float32(advanceCalculated) - advanceWant)
				}
			}
			g := Atom{
				Advance:   bag.ScaledPoint(advanceWant),
				Height:    f.Size - f.Depth,
				Depth:     f.Depth,
				Hyphenate: unicode.IsLetter(char),
				Codepoint: int(r.GlyphID),
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
