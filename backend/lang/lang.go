package lang

import (
	"io"
	"os"
	"slices"
	"unicode"

	"github.com/speedata/hyphenation"
)

// Lang represents a language for for hyphenation
type Lang struct {
	lang *hyphenation.Lang
	Name string
	// Lefthyphenmin and Righthyphenmin are the characters a word keeps at
	// least before and after a hyphen, Wordhyphenmin the characters it needs
	// to be hyphenated at all. Nonspacing marks and punctuation do not count
	// (CSS Text 4, hyphenate-limit-chars). The defaults are TeX's: 2, 3 and 0.
	Lefthyphenmin  int
	Righthyphenmin int
	Wordhyphenmin  int
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
	// The patterns keep their own minimums at zero, so they can be shared.
	hyphenpoints := l.lang.Hyphenate(word)
	if len(hyphenpoints) == 0 {
		return nil
	}
	// counted[i] is the number of characters in the first i runes.
	counted := make([]int, 1, len(word)+1)
	chars := 0
	for _, r := range word {
		if !unicode.Is(unicode.Mn, r) && !unicode.IsPunct(r) {
			chars++
		}
		counted = append(counted, chars)
	}
	if chars < l.Wordhyphenmin {
		return nil
	}
	hyphenpoints = slices.DeleteFunc(hyphenpoints, func(p int) bool {
		return counted[p] < l.Lefthyphenmin || chars-counted[p] < l.Righthyphenmin
	})
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
