package font

import (
	"sync"
	"unicode"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/textshape/ot"
)

var bufPool = sync.Pool{
	New: func() any { return ot.NewBuffer() },
}

// An Atom contains size information about the glyphs as a result of Shape
type Atom struct {
	Components string
	Advance    bag.ScaledPoint
	Height     bag.ScaledPoint
	Depth      bag.ScaledPoint
	XOffset    bag.ScaledPoint // Horizontal offset from GPOS positioning
	YOffset    bag.ScaledPoint // Vertical offset from GPOS positioning (e.g., mark attachment)
	Codepoint  int
	Kernafter  bag.ScaledPoint
	IsSpace    bool
	NoBreak    bool // Space that must not be a breakpoint (e.g. NBSP U+00A0)
	Hyphenate  bool
}

// MissingGlyphFunc is called when a character cannot be found in the font.
// The arguments are the font face and the rune that is missing.
type MissingGlyphFunc func(face *pdf.Face, r rune)

// Font is the main structure of a font instance
type Font struct {
	Face             *pdf.Face
	Hyphenchar       Atom
	SpaceChar        Atom
	Space            bag.ScaledPoint
	SpaceStretch     bag.ScaledPoint
	SpaceShrink      bag.ScaledPoint
	Size             bag.ScaledPoint
	Depth            bag.ScaledPoint
	Mag              int
	MissingGlyphFunc MissingGlyphFunc
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
// Direction is guessed from the script.
func (f *Font) Shape(text string, features []ot.Feature, variations map[string]float64) []Atom {
	return f.ShapeDir(text, features, variations, ot.DirectionInvalid)
}

// ShapeDir is like Shape but takes an explicit text direction.
// Pass ot.DirectionInvalid to fall back to script-based guessing.
// For RTL scripts (Hebrew, Arabic) shaped with DirectionRTL, the returned
// atoms are in visual order (i.e. ready for left-to-right placement).
func (f *Font) ShapeDir(text string, features []ot.Feature, variations map[string]float64, dir ot.Direction) []Atom {
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
	buf := bufPool.Get().(*ot.Buffer)
	buf.Reset()
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

	if dir != ot.DirectionInvalid {
		buf.SetDirection(dir)
	}
	buf.GuessSegmentProperties()
	f.Face.Shaper.Shape(buf, features)
	glyphs := make([]Atom, 0, len(buf.Info))
	space := f.Face.Codepoint(' ')
	lenBufInfo := len(buf.Info)
	// HarfBuzz returns RTL output in reverse-logical (visual) order, so the
	// "next cluster" used to delimit a glyph's source text comes from the
	// previous buffer entry, not the next one.
	rtl := buf.Direction == ot.DirectionRTL
	for i, r := range buf.Info {
		char := runes[r.Cluster]
		adv := buf.Pos[i].XAdvance
		advanceCalculated := int32(adv) * int32(f.Mag)
		advanceWant := float32(face.HorizontalAdvance(r.GlyphID)) * float32(f.Mag)

		if r.GlyphID == 0 && !unicode.IsSpace(char) && f.MissingGlyphFunc != nil {
			f.MissingGlyphFunc(f.Face, char)
		}

		if unicode.IsSpace(char) {
			glyphs = append(glyphs, Atom{
				IsSpace:    true,
				NoBreak:    char == '\u00A0',
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

			// Get GPOS positioning offsets and scale them
			xOffset := bag.ScaledPoint(int32(buf.Pos[i].XOffset) * int32(f.Mag))
			yOffset := bag.ScaledPoint(int32(buf.Pos[i].YOffset) * int32(f.Mag))

			g := Atom{
				Advance:   bag.ScaledPoint(advanceWant),
				Height:    f.Size - f.Depth,
				Depth:     f.Depth,
				XOffset:   xOffset,
				YOffset:   yOffset,
				Hyphenate: unicode.IsLetter(char),
				Codepoint: int(r.GlyphID),
				Kernafter: bdelta,
			}
			endCluster := len(runes)
			if rtl {
				if i > 0 {
					endCluster = int(buf.Info[i-1].Cluster)
				}
			} else {
				if i < lenBufInfo-1 {
					endCluster = int(buf.Info[i+1].Cluster)
				}
			}
			// Defensive: if the shaper produced non-monotonic clusters
			// (e.g. caller-set Direction conflicts with the script's natural
			// direction), fall back to a single-rune source range instead of
			// panicking on an inverted slice.
			if endCluster <= int(r.Cluster) {
				endCluster = int(r.Cluster) + 1
				if endCluster > len(runes) {
					endCluster = len(runes)
				}
			}
			g.Components = string(runes[r.Cluster:endCluster])
			glyphs = append(glyphs, g)
		}
	}
	bufPool.Put(buf)
	return glyphs
}
