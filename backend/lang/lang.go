package lang

import (
	"io"
	"os"

	"github.com/speedata/hyphenation"
)

// Lang represents a language for for hyphenation
type Lang struct {
	Lefthyphenmin  int
	Righthyphenmin int
	Name           string
	lang           *hyphenation.Lang
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

// Hyphenate returns a slice of hyphenation points
func (l *Lang) Hyphenate(word string) []int {
	l.lang.Leftmin = l.Lefthyphenmin
	l.lang.Rightmin = l.Righthyphenmin

	hyphenpoints := l.lang.Hyphenate(word)
	// The slice hyphenpoints contains the valid break points
	// after a character.
	// We need the number of characters to move forward,
	// so we change the slice
	if len(hyphenpoints) > 0 {
		for i := len(hyphenpoints) - 1; i > 0; i-- {
			hyphenpoints[i] = hyphenpoints[i] - hyphenpoints[i-1]
		}
	}
	return hyphenpoints
}

func (l *Lang) String() string {
	return l.Name
}
