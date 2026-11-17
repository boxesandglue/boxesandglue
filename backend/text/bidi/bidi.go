// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run gen.go gen_trieval.go gen_ranges.go

// Package bidi contains functionality for bidirectional text support.
//
// See https://www.unicode.org/reports/tr9.
//
// NOTE: UNDER CONSTRUCTION. This API may change in backwards incompatible ways
// and without notice.
package bidi

// TODO
// - Transformer for reordering?
// - Transformer (validator, really) for Bidi Rule.

import (
	"bytes"
	"unicode/utf8"
)

// The resolved embedding levels are part of the API: an Ordering holds runs of
// one level each (Run.Level), the level of every rune (Ordering.Levels) and the
// paragraph embedding level (Ordering.ParagraphLevel). The direction of a run
// alone is not enough to reorder a line, because rule L2 reverses by level:
// a number inside a right-to-left run in a left-to-right paragraph has level
// 2, the left-to-right text around it level 0.
//
// This package is a fork of golang.org/x/text/unicode/bidi. It differs in the
// bracket pairs of rule N0 (a closing bracket is matched by its opening
// counterpart, canonical equivalents are matched), the explicit default
// direction LeftToRight and the exposed levels. conformance_test.go runs the
// Unicode test files BidiTest.txt and BidiCharacterTest.txt against it.

// A Direction indicates the overall flow of text.
type Direction int

const (
	// Neutral means that text contains no left-to-right and right-to-left
	// characters and that no default direction has been set.
	Neutral Direction = iota

	// LeftToRight indicates the text contains no right-to-left characters and
	// that either there are some left-to-right characters or the option
	// DefaultDirection(LeftToRight) was passed.
	LeftToRight

	// RightToLeft indicates the text contains no left-to-right characters and
	// that either there are some right-to-left characters or the option
	// DefaultDirection(RightToLeft) was passed.
	RightToLeft

	// Mixed indicates text contains both left-to-right and right-to-left
	// characters.
	Mixed
)

type options struct {
	defaultDirection Direction
}

// An Option is an option for Bidi processing.
type Option func(*options)

// ICU allows the user to define embedding levels. This may be used, for example,
// to use hierarchical structure of markup languages to define embeddings.
// The following option may be a way to expose this functionality in this API.
// // LevelFunc sets a function that associates nesting levels with the given text.
// // The levels function will be called with monotonically increasing values for p.
// func LevelFunc(levels func(p int) int) Option {
// 	panic("unimplemented")
// }

// DefaultDirection sets the default direction for a Paragraph. The direction is
// overridden if the text contains directional characters.
func DefaultDirection(d Direction) Option {
	return func(opts *options) {
		opts.defaultDirection = d
	}
}

// A Paragraph holds a single Paragraph for Bidi processing.
type Paragraph struct {
	p          []byte
	o          Ordering
	opts       []Option
	types      []Class
	pairTypes  []bracketType
	pairValues []rune
	runes      []rune
	options    options
}

// Initialize p.runes, p.pairTypes, p.pairValues and p.types from the input previously
// set by p.SetBytes() or p.SetString(). Also limit the input up to (and including) a paragraph
// separator (bidi class B): the separator is the last character of its paragraph and gets
// the paragraph level (rule L1).
//
// The function p.Order() needs these values to be set, so this preparation could be postponed.
// But since the SetBytes and SetStrings functions return the length of the input up to the paragraph
// separator, the whole input needs to be processed anyway and should not be done twice.
//
// The function has the same return values as SetBytes() / SetString(). The
// length counts the bytes of the input, an invalid UTF-8 byte as one byte.
func (p *Paragraph) prepareInput() (n int, err error) {
	// clear slices from previous SetString or SetBytes
	p.runes = nil
	p.pairTypes = nil
	p.pairValues = nil
	p.types = nil

	for n < len(p.p) {
		r, size := utf8.DecodeRune(p.p[n:])
		n += size
		props, _ := LookupRune(r)
		cls := props.Class()
		p.runes = append(p.runes, r)
		p.types = append(p.types, cls)
		switch {
		case !props.IsBracket():
			p.pairTypes = append(p.pairTypes, bpNone)
			p.pairValues = append(p.pairValues, 0)
		case props.IsOpeningBracket():
			p.pairTypes = append(p.pairTypes, bpOpen)
			p.pairValues = append(p.pairValues, canonicalBracket(r))
		default:
			p.pairTypes = append(p.pairTypes, bpClose)
			p.pairValues = append(p.pairValues, canonicalBracket(props.reverseBracket(r)))
		}
		if cls == B {
			break
		}
	}
	return n, nil
}

// canonicalBracket returns the opening bracket r stands for when brackets are
// paired. BD16 compares brackets under canonical equivalence, and the angle
// brackets U+2329 and U+232A decompose to U+3008 and U+3009, so U+2329 also
// pairs with U+3009 and U+3008 with U+232A. These two are the only bracket
// characters with a canonical decomposition.
func canonicalBracket(r rune) rune {
	switch r {
	case '〈':
		return '〈'
	case '〉':
		return '〉'
	}
	return r
}

// SetBytes configures p for the given paragraph text. It replaces text
// previously set by SetBytes or SetString. If b contains a paragraph separator
// it will only process the first paragraph and report the number of bytes
// consumed from b including this separator. Error may be non-nil if options are
// given.
func (p *Paragraph) SetBytes(b []byte, opts ...Option) (n int, err error) {
	p.p = b
	p.opts = opts
	return p.prepareInput()
}

// SetString configures s for the given paragraph text. It replaces text
// previously set by SetBytes or SetString. If s contains a paragraph separator
// it will only process the first paragraph and report the number of bytes
// consumed from s including this separator. Error may be non-nil if options are
// given.
func (p *Paragraph) SetString(s string, opts ...Option) (n int, err error) {
	p.p = []byte(s)
	p.opts = opts
	return p.prepareInput()
}

// FirstStrong reports the direction of the first strong character of the
// paragraph outside isolates (rule P2), Neutral if there is none. Unlike
// ParagraphLevel it tells a paragraph without strong characters from a left
// to right one, so a caller can look at the next paragraph instead.
func (p *Paragraph) FirstStrong() Direction {
	depth := 0
	for _, t := range p.types {
		switch {
		case t == LRI || t == RLI || t == FSI:
			depth++
		case t == PDI:
			if depth > 0 {
				depth--
			}
		case depth > 0:
		case t == L:
			return LeftToRight
		case t == R || t == AL:
			return RightToLeft
		}
	}
	return Neutral
}

// IsLeftToRight reports whether the principle direction of rendering for this
// paragraphs is left-to-right. If this returns false, the principle direction
// of rendering is right-to-left.
func (p *Paragraph) IsLeftToRight() bool {
	return p.Direction() == LeftToRight
}

// Direction returns the direction of the text of this paragraph.
//
// The direction may be LeftToRight, RightToLeft, Mixed, or Neutral.
func (p *Paragraph) Direction() Direction {
	return p.o.Direction()
}

// TODO: what happens if the position is > len(input)? This should return an error.

// RunAt reports the Run at the given position of the input text.
//
// This method can be used for computing line breaks on paragraphs.
func (p *Paragraph) RunAt(pos int) Run {
	c := 0
	runNumber := len(p.o.runes) - 1
	for i, r := range p.o.runes {
		c += len(r)
		if pos < c {
			runNumber = i
			break
		}
	}
	return p.o.Run(runNumber)
}

// directionOf returns the direction of the runes at level lvl: even levels
// are left to right, odd levels right to left.
func directionOf(lvl level) Direction {
	if lvl%2 == 0 {
		return LeftToRight
	}
	return RightToLeft
}

// calculateOrdering splits runes into runs of one embedding level each.
// paragraphLevel is the resolved paragraph embedding level.
func calculateOrdering(levels []level, runes []rune, paragraphLevel level) Ordering {
	o := Ordering{levels: levels, paragraphLevel: paragraphLevel}
	if len(levels) == 0 {
		return o
	}
	start := 0
	for i := 1; i <= len(levels); i++ {
		if i < len(levels) && levels[i] == levels[start] {
			continue
		}
		o.runes = append(o.runes, runes[start:i])
		o.directions = append(o.directions, directionOf(levels[start]))
		o.runLevels = append(o.runLevels, levels[start])
		o.startpos = append(o.startpos, start)
		start = i
	}
	return o
}

// Order computes the visual ordering of all the runs in a Paragraph.
func (p *Paragraph) Order() (Ordering, error) {
	if len(p.types) == 0 {
		return Ordering{}, nil
	}

	// The options belong to the last SetString or SetBytes, not to an earlier
	// text of the same Paragraph.
	p.options = options{}
	for _, fn := range p.opts {
		fn(&p.options)
	}
	lvl := level(-1)
	switch p.options.defaultDirection {
	case RightToLeft:
		lvl = 1
	case LeftToRight:
		lvl = 0
	}
	para, err := newParagraph(p.types, p.pairTypes, p.pairValues, lvl)
	if err != nil {
		return Ordering{}, err
	}

	levels := para.getLevels([]int{len(p.types)})

	p.o = calculateOrdering(levels, p.runes, para.embeddingLevel)
	return p.o, nil
}

// Line computes the visual ordering of runs for a single line starting and
// ending at the given positions in the original text.
func (p *Paragraph) Line(start, end int) (Ordering, error) {
	lineTypes := p.types[start:end]
	para, err := newParagraph(lineTypes, p.pairTypes[start:end], p.pairValues[start:end], -1)
	if err != nil {
		return Ordering{}, err
	}
	levels := para.getLevels([]int{len(lineTypes)})
	o := calculateOrdering(levels, p.runes[start:end], para.embeddingLevel)
	return o, nil
}

// An Ordering holds the runs of a Paragraph in logical order, one run per
// stretch of runes with the same embedding level. Calling SetBytes or
// SetString on the originating Paragraph invalidates an Ordering. The methods
// of an Ordering should only be called by one goroutine at a time.
type Ordering struct {
	runes          [][]rune
	directions     []Direction
	runLevels      []level
	startpos       []int
	levels         []level
	paragraphLevel level
}

// Direction reports the direction of the first run. It is not the paragraph
// direction: a right-to-left paragraph that starts with a number has a
// left-to-right first run. Use ParagraphLevel for the paragraph direction.
func (o *Ordering) Direction() Direction {
	return o.directions[0]
}

// ParagraphLevel returns the paragraph embedding level: 0 for a left-to-right
// paragraph, 1 for a right-to-left one. Without a default direction it is
// resolved from the first strong character (rules P2 and P3).
func (o *Ordering) ParagraphLevel() uint8 {
	return uint8(o.paragraphLevel)
}

// Levels returns the resolved embedding level of every rune of the paragraph
// in logical order, after rule L1 with the whole paragraph taken as one line.
// Characters removed by rule X9 (explicit formatting characters, BN) get the
// level of the character before them, or the paragraph level at the start.
func (o *Ordering) Levels() []uint8 {
	ret := make([]uint8, len(o.levels))
	for i, l := range o.levels {
		ret[i] = uint8(l)
	}
	return ret
}

// NumRuns returns the number of runs.
func (o *Ordering) NumRuns() int {
	return len(o.runes)
}

// Run returns the ith run within the ordering.
func (o *Ordering) Run(i int) Run {
	r := Run{
		runes:     o.runes[i],
		direction: o.directions[i],
		level:     o.runLevels[i],
		startpos:  o.startpos[i],
	}
	return r
}

// TODO: perhaps with options.
// // Reorder creates a reader that reads the runes in visual order per character.
// // Modifiers remain after the runes they modify.
// func (l *Runs) Reorder() io.Reader {
// 	panic("unimplemented")
// }

// A Run is a continuous sequence of characters of a single embedding level.
// Two neighboring runs may have the same direction, a number at level 2 next
// to left-to-right text at level 0 say.
type Run struct {
	runes     []rune
	direction Direction
	level     level
	startpos  int
}

// String returns the text of the run in its original order.
func (r *Run) String() string {
	return string(r.runes)
}

// Bytes returns the text of the run in its original order.
func (r *Run) Bytes() []byte {
	return []byte(r.String())
}

// TODO: methods for
// - Display order
// - headers and footers
// - bracket replacement.

// Direction reports the direction of the run.
func (r *Run) Direction() Direction {
	return r.direction
}

// Level reports the embedding level of the run. Even levels are left to
// right, odd levels right to left.
func (r *Run) Level() uint8 {
	return uint8(r.level)
}

// Pos returns the position of the Run within the text passed to SetBytes or SetString of the
// originating Paragraph value.
func (r *Run) Pos() (start, end int) {
	return r.startpos, r.startpos + len(r.runes) - 1
}

// AppendReverse reverses the order of characters of in, appends them to out,
// and returns the result. Modifiers will still follow the runes they modify.
// Brackets are replaced with their counterparts.
func AppendReverse(out, in []byte) []byte {
	ret := make([]byte, len(in)+len(out))
	copy(ret, out)
	inRunes := bytes.Runes(in)

	for i, r := range inRunes {
		prop, _ := LookupRune(r)
		if prop.IsBracket() {
			inRunes[i] = prop.reverseBracket(r)
		}
	}

	for i, j := 0, len(inRunes)-1; i < j; i, j = i+1, j-1 {
		inRunes[i], inRunes[j] = inRunes[j], inRunes[i]
	}
	copy(ret[len(out):], string(inRunes))

	return ret
}

// ReverseString reverses the order of characters in s and returns a new string.
// Modifiers will still follow the runes they modify. Brackets are replaced with
// their counterparts.
func ReverseString(s string) string {
	input := []rune(s)
	li := len(input)
	ret := make([]rune, li)
	for i, r := range input {
		prop, _ := LookupRune(r)
		if prop.IsBracket() {
			ret[li-i-1] = prop.reverseBracket(r)
		} else {
			ret[li-i-1] = r
		}
	}
	return string(ret)
}
