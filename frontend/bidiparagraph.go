package frontend

import (
	"strings"
	"unicode/utf8"

	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/boxesandglue/backend/text/bidi"
	"github.com/boxesandglue/textshape/ot"
)

// The bidi levels of a paragraph are resolved once, over the whole text of
// the paragraph (UAX#9 works on paragraphs, and the level of a number or a
// space depends on the text around it, across the strings of the child
// texts and across font changes). Mknodes collects the text, resolves it and
// hands each string its slice of the levels; the shaper splits a string into
// runs of one level and shapes each run in the direction of its level. The
// line reorder after line breaking (bidiReorderLines) reads the levels off
// the nodes.

// objectReplacement stands for an item of a paragraph that is not text in
// the text the bidi algorithm sees: an image or another box, a leader, the
// list marker. U+FFFC is a neutral (ON), so the item takes the level of the
// text around it, as a word would.
const objectReplacement = '￼'

// collectBidiText writes the text of te to b in the order in which Mknodes
// builds the nodes of te: a string as it is, one objectReplacement for each
// node item, leader and the list marker, and the bidi control characters of
// SettingUnicodeBidi around a child Text (bidiControls) and around the whole
// paragraph (paragraphControls). The control characters exist in this text
// only; Mknodes skips their levels at the same places. top is true for the
// Text Mknodes is called with; the list marker of a child Text is not built
// (Mknodes removes it before it descends). dir is the direction te inherits.
// Items Mknodes does not know contribute nothing.
func collectBidiText(te *Text, b *strings.Builder, top bool, dir Direction) {
	if d, ok := te.Settings[SettingDirection].(Direction); ok {
		dir = d
	}
	var closing string
	if top {
		if prep, ok := te.Settings[SettingPrepend]; ok {
			if _, ok := prep.(node.Node); ok {
				b.WriteRune(objectReplacement)
			}
		}
		var open string
		open, closing = paragraphControls(te)
		b.WriteString(open)
	}
	for _, itm := range te.Items {
		switch t := itm.(type) {
		case string:
			b.WriteString(t)
		case *Text:
			if _, ok := t.Settings[SettingLeader]; ok {
				b.WriteRune(objectReplacement)
				continue
			}
			open, childClosing := bidiControls(t, dir)
			b.WriteString(open)
			collectBidiText(t, b, false, dir)
			b.WriteString(childClosing)
		case node.Node:
			b.WriteRune(objectReplacement)
		}
	}
	b.WriteString(closing)
}

// Bidi control characters of UAX#9.
const (
	bidiLRE = "\u202A"
	bidiRLE = "\u202B"
	bidiPDF = "\u202C"
	bidiLRO = "\u202D"
	bidiRLO = "\u202E"
	bidiLRI = "\u2066"
	bidiRLI = "\u2067"
	bidiFSI = "\u2068"
	bidiPDI = "\u2069"
)

// bidiControls returns the control characters that open and close the
// SettingUnicodeBidi of the child Text te, in the direction of te or else
// the one it inherits (CSS Writing Modes 3 §2.4.2). A child without the
// setting gets none.
func bidiControls(te *Text, inherited Direction) (open, closing string) {
	ub, _ := te.Settings[SettingUnicodeBidi].(UnicodeBidi)
	dir := inherited
	if d, ok := te.Settings[SettingDirection].(Direction); ok {
		dir = d
	}
	rtl := dir == DirectionRTL
	pick := func(ltr, rtlValue string) string {
		if rtl {
			return rtlValue
		}
		return ltr
	}
	switch ub {
	case UnicodeBidiEmbed:
		return pick(bidiLRE, bidiRLE), bidiPDF
	case UnicodeBidiIsolate:
		return pick(bidiLRI, bidiRLI), bidiPDI
	case UnicodeBidiBidiOverride:
		return pick(bidiLRO, bidiRLO), bidiPDF
	case UnicodeBidiIsolateOverride:
		return pick(bidiLRI, bidiRLI) + pick(bidiLRO, bidiRLO), bidiPDF + bidiPDI
	case UnicodeBidiPlaintext:
		return bidiFSI, bidiPDI
	}
	return "", ""
}

// paragraphControls returns the control characters around the text of the
// paragraph te. Only the overrides have an effect on a paragraph: every
// character takes the paragraph direction. Embed and isolate open no level
// on a block, plaintext sets the paragraph direction (paragraphOptions).
func paragraphControls(te *Text) (open, closing string) {
	ub, _ := te.Settings[SettingUnicodeBidi].(UnicodeBidi)
	if ub != UnicodeBidiBidiOverride && ub != UnicodeBidiIsolateOverride {
		return "", ""
	}
	if d, _ := te.Settings[SettingDirection].(Direction); d == DirectionRTL {
		return bidiRLO, bidiPDF
	}
	return bidiLRO, bidiPDF
}

// paragraphBidi holds the resolved levels of the text collectBidiText
// collected, one per rune, while Mknodes walks the items.
type paragraphBidi struct {
	levels []uint8
	// pos is the index of the first rune of the next item.
	pos int
}

// newParagraphBidi resolves the levels of te with the paragraph direction
// dir.
func newParagraphBidi(te *Text, dir Direction) *paragraphBidi {
	var b strings.Builder
	collectBidiText(te, &b, true, dir)
	return &paragraphBidi{levels: resolveLevels(b.String(), dir)}
}

// take returns the levels of the next n runes. Once Mknodes asks for more
// than collectBidiText collected, the walks disagree; pb then gives no
// levels at all, and every string is resolved on its own.
func (pb *paragraphBidi) take(n int) []uint8 {
	if pb == nil || pb.levels == nil {
		return nil
	}
	if pb.pos+n > len(pb.levels) {
		pb.levels = nil
		return nil
	}
	l := pb.levels[pb.pos : pb.pos+n : pb.pos+n]
	pb.pos += n
	return l
}

// done reports whether Mknodes used exactly the levels of the collected
// text.
func (pb *paragraphBidi) done() bool {
	return pb.levels != nil && pb.pos == len(pb.levels)
}

// resolveLevels returns the bidi level of every rune of text with the
// paragraph direction dir. A paragraph separator (a newline, class B) ends
// a UAX#9 paragraph; the next one is resolved with the same direction, so
// every part of the text has the direction of the paragraph it is set in.
// An invalid UTF-8 byte counts as one rune, as in a range loop.
func resolveLevels(text string, dir Direction) []uint8 {
	def := bidi.LeftToRight
	var base uint8
	if dir == DirectionRTL {
		def = bidi.RightToLeft
		base = 1
	}
	levels := make([]uint8, 0, utf8.RuneCountInString(text))
	var p bidi.Paragraph
	for rest := text; rest != ""; {
		n, err := p.SetString(rest, bidi.DefaultDirection(def))
		var o bidi.Ordering
		if err == nil && n > 0 {
			o, err = p.Order()
		}
		if err != nil || n == 0 {
			for range rest {
				levels = append(levels, base)
			}
			break
		}
		levels = append(levels, o.Levels()...)
		rest = rest[n:]
	}
	return levels
}

// detectParagraphDirection returns the direction of the first strong
// character of te outside isolates (UAX#9 rules P2 and P3): DirectionRTL for
// Hebrew or Arabic, DirectionLTR for Latin, and DirectionLTR when te has no
// strong character at all. A part of the text after a newline counts when
// the parts before it have no strong character.
func detectParagraphDirection(te *Text) Direction {
	var b strings.Builder
	collectBidiText(te, &b, true, DirectionLTR)
	var p bidi.Paragraph
	for rest := b.String(); rest != ""; {
		n, err := p.SetString(rest)
		if err != nil || n == 0 {
			break
		}
		switch p.FirstStrong() {
		case bidi.RightToLeft:
			return DirectionRTL
		case bidi.LeftToRight:
			return DirectionLTR
		}
		rest = rest[n:]
	}
	return DirectionLTR
}

// shapeWithBidi shapes str with the bidi levels of its runes and returns the
// atoms in logical order alongside a parallel slice of their levels. levels
// has one entry per rune of str, the slice of the paragraph's levels; when it
// does not fit (BuildNodelistFromString called on its own), str is resolved
// as a paragraph of its own with the direction paragraphDir.
//
// str is cut into runs of one level, and each run is shaped in the direction
// of its level. Soft hyphen and zero width space cut runs as well. The shaper
// is configured to drop default-ignorables from the buffer (font.ShapeDir
// sets BufferFlagRemoveDefaultIgnorables), so U+00AD and U+200B never reach
// the atom loop on their own; a sentinel atom (Components is the character,
// IsSpace=false) takes their place. The atom loop in BuildNodelistFromString
// turns a soft hyphen into a discretionary break (or drops it for `hyphens:
// none`) and a zero width space into a break opportunity without width.
func shapeWithBidi(fnt *font.Font, str string, features []ot.Feature, variations map[string]float64, paragraphDir Direction, levels []uint8) ([]font.Atom, []uint8) {
	if str == "" {
		return nil, nil
	}
	if len(levels) != utf8.RuneCountInString(str) {
		levels = resolveLevels(str, paragraphDir)
	}
	var atoms []font.Atom
	var atomLevels []uint8
	// The run being collected is str[start:i] at level runLevel.
	start := 0
	var runLevel uint8
	flush := func(end int) {
		if end <= start {
			return
		}
		runAtoms := shapeLevelRun(fnt, str[start:end], features, variations, runLevel)
		atoms = append(atoms, runAtoms...)
		for range runAtoms {
			atomLevels = append(atomLevels, runLevel)
		}
	}
	ri := 0
	for i, r := range str {
		lvl := levels[ri]
		ri++
		if r == '­' || r == '​' {
			flush(i)
			atoms = append(atoms, font.Atom{Components: string(r)})
			atomLevels = append(atomLevels, lvl)
			start = i + utf8.RuneLen(r)
			continue
		}
		if i > start && lvl != runLevel {
			flush(i)
			start = i
		}
		if i == start {
			runLevel = lvl
		}
	}
	flush(len(str))
	return atoms, atomLevels
}

// shapeLevelRun shapes s, a run of one bidi level, in the direction of the
// level: left to right for an even level, right to left for an odd one.
// HarfBuzz returns RTL output in reverse-logical (visual) order. The atoms
// are stored in logical order so paragraph-edge whitespace stripping and
// Knuth-Plass line breaking see word boundaries at the natural positions;
// the post-linebreak reorder rebuilds visual order at line time.
func shapeLevelRun(fnt *font.Font, s string, features []ot.Feature, variations map[string]float64, level uint8) []font.Atom {
	dir := ot.DirectionLTR
	if level%2 == 1 {
		dir = ot.DirectionRTL
	}
	atoms := fnt.ShapeDir(s, features, variations, dir)
	if dir == ot.DirectionRTL {
		reverseAtoms(atoms)
	}
	return atoms
}
