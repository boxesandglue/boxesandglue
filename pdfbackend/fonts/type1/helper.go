package type1

import (
	"bytes"
	"fmt"
	"path/filepath"
	"unicode"
)

// Read a positive integer number from r. Skip following white space.
func readInt(r *bytes.Reader) (int, error) {
	var num int
	for {
		s, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		if !unicode.IsDigit(rune(s)) {
			break
		}
		num = num*10 + int(s) - '0'
	}
	skipWhitespace(r)
	return num, nil
}

// Read a string from r and skip the following white space.
// The string must be equal to the given expected. Return an error otherwise.
func assertString(r *bytes.Reader, expected string) error {
	s, err := readString(r)
	if err != nil {
		return err
	}
	if s != expected {
		return fmt.Errorf("%q expected but got %q", expected, s)
	}
	return nil
}

// Read a string from r, skip the following white space.
func readString(r *bytes.Reader) (string, error) {
	buf := []byte{}
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if !unicode.IsLetter(rune(b)) {
			r.UnreadByte()
			break
		}
		buf = append(buf, b)

	}
	skipWhitespace(r)
	return string(buf), nil
}

// Read and return a PostScript name from r.
// A name consists of letters, punctuation and numbers (/.notdef /Euro123).
// Skips whitespace at the end.
func readName(r *bytes.Reader) (string, error) {
	buf := []byte{}
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if !unicode.IsLetter(rune(b)) && !unicode.IsPunct(rune(b)) && !unicode.IsNumber(rune(b)) {
			r.UnreadByte()
			break
		}
		buf = append(buf, b)

	}
	skipWhitespace(r)
	return string(buf), nil
}

// Skip over all white space.
func skipWhitespace(r *bytes.Reader) error {
	for {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		if !unicode.IsSpace(rune(b)) {
			r.UnreadByte()
			break
		}
	}
	return nil
}

// Skip until the byte b is found. The next byte is b (unless an error occurs).
func forwardTo(r *bytes.Reader, b byte) error {
	for {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		if b == '/' {
			r.UnreadByte()
			break
		}
	}
	return nil
}

// Trim the file name extension from the file
func trimSuffix(fn string) string {
	var extension = filepath.Ext(fn)
	return fn[0 : len(fn)-len(extension)]
}
