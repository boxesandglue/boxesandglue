package font

import (
	"fmt"
	"unicode"

	"github.com/speedata/boxesandglue/bag"
)

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
				Advance:    f.Size,
				Components: " ",
			})
		} else {
			adv, err := f.AdvanceX(r)
			if err != nil {
				fmt.Println(err)
			}
			codepoints = append(codepoints, Codepoint{
				Glyph:      int(r),
				Advance:    adv,
				Hyphenate:  unicode.IsLetter(r),
				Components: string(r),
			})
		}
	}
	return codepoints
}
