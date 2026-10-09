package node

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
)

// LineModel places each line of a paragraph in its line box and decides the
// glue between the lines. When LinebreakSettings.LineModel is set, Linebreak
// asks it for every line it builds, in place of HalfLeading and the lineskip
// glue, so a caller can set lines by rules of its own (a word processor's
// line spacing, say) without changing the line breaker. A model is the
// caller's own value and carries whatever it needs, such as the paragraph's
// font size.
//
// A nil LineModel keeps the built-in behaviour: HalfLeading, or lineskip glue
// padding each line to LineHeight.
type LineModel interface {
	// LineBox returns the height above the baseline and the depth below it
	// of line, a line Linebreak has just packed to HSize. line.Height and
	// line.Depth are its natural extent; each glyph on it carries its font
	// (with the size and the face's vertical metrics) and its LineShift.
	// A model whose lines end elsewhere than LinebreakSettings.Font's text
	// edges may record the line's trims as its LineTrimStart and
	// LineTrimEnd attributes; Linebreak keeps them, and takes one off again
	// when it is 0.
	LineBox(line *HList, settings *LinebreakSettings) (height, depth bag.ScaledPoint)
	// Leading returns the glue Linebreak puts next to line, whose line box
	// is already set: above each line but the first, and below the last
	// line unless OmitLastLeading is set. A nil glue puts nothing there.
	Leading(line *HList, settings *LinebreakSettings) *Glue
}

// A LineModel that also implements BackgroundAreaModel decides the box an
// inline background covers on its lines. The frontend asks it once per
// background, with the font of the Text that carries the background, when
// that Text's line model (frontend.SettingLineModel) implements it. The band
// it returns is painted for the whole background, on every line it spans,
// and is moved by that Text's vertical offset (vertical-align, LineShift) as
// the font's box would be. The band wins over frontend.SettingBackgroundArea,
// which htmlbag sets to frontend.BackgroundAreaAscentDescent by default.
// When the font of that Text is not known, the model is not asked and each
// glyph's font gives the box, as without a model.
type BackgroundAreaModel interface {
	// BackgroundArea returns the height above and the depth below the
	// baseline of the band an inline background in font f covers.
	BackgroundArea(f *font.Font) (height, depth bag.ScaledPoint)
}
