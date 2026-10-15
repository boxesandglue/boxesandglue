package lang

import (
	"io"
	"os"
	"slices"
	"unicode/utf8"

	"github.com/speedata/hyphenation"
)

// Lang represents a language for for hyphenation
type Lang struct {
	lang           *hyphenation.Lang
	Name           string
	Lefthyphenmin  int
	Righthyphenmin int
}

// LoadPatternFile loads the hyphenation patterns with the given file name
func LoadPatternFile(fn string) (*Lang, error) {
	r, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	hl, err := hyphenation.New(r)
	if err != nil {
		return nil, err
	}
	if err = r.Close(); err != nil {
		return nil, err
	}

	l := &Lang{lang: hl, Lefthyphenmin: 2, Righthyphenmin: 3}

	return l, nil
}

// NewFromReader returns a Lang object from the reader r which points to
// hyphenation patterns.
func NewFromReader(r io.Reader) (*Lang, error) {
	hl, err := hyphenation.New(r)
	if err != nil {
		return nil, err
	}
	l := &Lang{lang: hl, Lefthyphenmin: 2, Righthyphenmin: 3}

	return l, nil
}

// Copy returns a Lang of its own over the same patterns, which Hyphenate
// never changes, with a word cache of its own, so copies may hyphenate at once
// with different minimums and a copy's cache goes away with it.
func (l *Lang) Copy() *Lang {
	c := *l
	c.lang = l.lang.Clone()
	return &c
}

// Hyphenate returns a slice of hyphenation points
func (l *Lang) Hyphenate(word string) []int {
	// The patterns keep their own minimums at zero, so they can be shared;
	// these are the bounds they would apply, a point counting runes from the
	// word's start.
	first := 1 + max(l.Lefthyphenmin, 0)
	end := utf8.RuneCountInString(word) + 1 - max(l.Righthyphenmin, 0)
	hyphenpoints := slices.DeleteFunc(l.lang.Hyphenate(word), func(p int) bool { return p < first || p >= end })
	// The slice hyphenpoints contains the valid break points
	// after a character.
	// We need the number of characters to move forward,
	// so we change the slice
	if len(hyphenpoints) > 0 {
		for i := len(hyphenpoints) - 1; i > 0; i-- {
			hyphenpoints[i] -= hyphenpoints[i-1]
		}
	}
	return hyphenpoints
}

func (l *Lang) String() string {
	return l.Name
}
