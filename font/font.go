package font

import (
	"fmt"
	"unicode"

	"github.com/speedata/boxesandglue/bag"
)

// Font is the main structure of a font instance
type Font struct {
	GlueSize bag.ScaledPoint
	size     bag.ScaledPoint
}

// LoadFont loads a font from the harddrive
func LoadFont(filename string, size bag.ScaledPoint) (*Font, error) {
	fmt.Printf("Load font %s size %sf\n", filename, size.String())
	return &Font{size: size, GlueSize: size}, nil
}

// A Codepoint contains size information about the glyphs as a result of Shape
type Codepoint struct {
	Glyph      int
	Advance    bag.ScaledPoint
	Components string
	Hyphenate  bool
}

// Shape transforms the text into a slice of codepoints.
func (f *Font) Shape(text string) []Codepoint {
	codepoints := make([]Codepoint, 0, len(text))
	for _, r := range text {
		if unicode.IsSpace(r) {
			codepoints = append(codepoints, Codepoint{
				Glyph:      32,
				Advance:    f.size,
				Components: " ",
			})
		} else {
			codepoints = append(codepoints, Codepoint{
				Glyph:      int(r),
				Advance:    f.size,
				Hyphenate:  unicode.IsLetter(r),
				Components: string(r),
			})
		}
	}
	return codepoints
}
