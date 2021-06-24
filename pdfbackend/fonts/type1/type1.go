// Package type1 reads and writes fonts in Type1 format.
package type1

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Char holds the information about one of the max. 255 characters in a font.
type Char struct {
	Name          string
	Codepoint     rune
	OrigCodepoint int
	Wx            int
	BBox          []int
	Kernx         map[rune]int
	Charstring    []byte
}

// Type1 holds all information about a Type 1 font
type Type1 struct {
	FontName           string
	FullName           string
	FamilyName         string
	Weight             string
	ItalicAngle        int
	IsFixedPitch       bool
	UnderlinePosition  int
	UnderlineThickness int
	Version            string
	EncodingScheme     string
	FontBBox           []int
	CapHeight          int
	XHeight            int
	Descender          int
	Ascender           int
	NumChars           int
	CharsName          map[string]Char
	CharsCodepoint     map[rune]Char
	Segments           [][]byte
	SubsetID           string
}

// Return a string of length 6 based on the characters in runelist.
// All returned characters are in the range A-Z.
func getCharTag(runelist []rune) string {
	sum := md5.Sum([]byte(string(runelist)))
	ret := make([]rune, 6)
	for i := 0; i < 6; i++ {
		ret[i] = rune(sum[2*i]+sum[2*i+1])/26 + 'A'
	}
	return string(ret)
}

// Subset reduces the first and second segment so that it only contains the necessary characters.
// Name must be a string length 6 such as "ABCDEF" and should be unique.
// Returns a string containing all the names of the used characters, for example /one/a/b/V/exclam,
// and the error if something went wrong.
func (t *Type1) Subset(runelist []rune) (string, error) {
	var err error
	t.SubsetID = getCharTag(runelist)
	decoded := decodeEexec(t.Segments[1])
	r := bytes.NewReader(decoded)

	indexCharstrings := bytes.Index(decoded, []byte("/CharStrings"))
	bb := new(bytes.Buffer)
	_, err = bb.Write(decoded[:indexCharstrings])
	if err != nil {
		return "", err
	}
	_, err = r.Seek(int64(indexCharstrings+len("/CharStrings")+1), os.SEEK_SET)
	if err != nil {
		return "", err
	}

	err = forwardTo(r, '/')
	if err != nil {
		return "", err
	}

	for {
		charname, codes, err := readCharstring(r)
		if err == io.EOF {
			break
		}

		c := t.CharsName[charname]
		c.Charstring = codes

		t.CharsName[charname] = c
		t.CharsCodepoint[c.Codepoint] = c
	}

	fmt.Fprintf(bb, "/CharStrings %d dict dup begin\n", len(runelist)+1)
	for _, r := range runelist {
		c := t.CharsCodepoint[r]
		fmt.Fprintf(bb, "/%s %d RD ", c.Name, len(c.Charstring))
		bb.Write(c.Charstring)
		fmt.Fprintln(bb, " ND")
	}
	cs := t.CharsName[".notdef"].Charstring
	fmt.Fprintf(bb, "/.notdef %d RD ", len(cs))
	bb.Write(cs)
	fmt.Fprintln(bb, " ND")

	_, err = io.Copy(bb, r)
	if err != nil {
		return "", err
	}
	t.Segments[1] = encodeEexec([]byte{1, 1, 1, 1}, bb.Bytes())

	// segment 1 done, now segment 0
	segments0 := t.Segments[0]

	// Is there a requirement for line ending? I don't know, so I try out two common ways
	var lines []string
	{
		lineenda := regexp.MustCompile("\r\n?")
		lineendb := regexp.MustCompile("\r?\n")
		a := lineenda.Split(string(segments0), -1)
		b := lineendb.Split(string(segments0), -1)
		if len(a) > len(b) {
			lines = a
		} else {
			lines = b
		}
	}

	fontnameRegexp := regexp.MustCompile("/FontName /(.*) def")
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "/FontName") {
			lines[i] = fontnameRegexp.ReplaceAllString(lines[i], "/FontName /"+t.SubsetID+"+$1 def")
		}
	}
	segments0 = []byte(strings.Join(lines, "\n"))

	indexEncoding := bytes.Index(segments0, []byte("/Encoding"))
	indexReadonlyDef := bytes.Index(segments0[indexEncoding:], []byte("readonly def"))
	r = bytes.NewReader(segments0)
	r.Seek(int64(indexEncoding+len("/Encoding")+1), os.SEEK_SET)
	_, err = readInt(r)
	if err != nil {
		return "", err
	}
	assertString(r, "array")
	bb = new(bytes.Buffer)
	bb.Write(segments0[:indexEncoding])
	fmt.Fprintf(bb, "/Encoding 256 array\n")
	fmt.Fprintln(bb, "0 1 255 {1 index exch /.notdef put } for")

	ret := make([]string, len(runelist))
	for _, r := range runelist {
		fmt.Fprintf(bb, "dup %d /%s put\n", t.CharsCodepoint[r].OrigCodepoint, t.CharsCodepoint[r].Name)
		ret = append(ret, "/", t.CharsCodepoint[r].Name)
	}
	bb.Write(segments0[indexEncoding+indexReadonlyDef:])
	t.Segments[0] = bb.Bytes()
	return strings.Join(ret, ""), nil
}

// Read the charstring entry, but don't decrypt. If no more strings are available,
// return an io.EOF.
// The reader is expected to be a the beginning of an entry
// (at the forward slash of the name).
func readCharstring(r *bytes.Reader) (charname string, buf []byte, err error) {
	var b byte
	b, err = r.ReadByte()
	if err != nil {
		return
	}
	if b != '/' {
		err = io.EOF
		r.UnreadByte()
		return
	}
	charname, err = readName(r)
	if err != nil {
		return
	}
	lengthEncoded, err := readInt(r)
	if err != nil {
		return
	}
	if err = assertString(r, "RD"); err != nil {
		return
	}
	buf = make([]byte, lengthEncoded)
	_, err = r.Read(buf)
	if err != nil {
		return
	}
	skipWhitespace(r)
	if err = assertString(r, "ND"); err != nil {
		return
	}
	return
}

// Decodes the encoded bytes.
func decodeEexec(encoded []byte) []byte {
	var r, c1, c2 uint16
	var cipher byte

	r = 55665
	c1 = 52845
	c2 = 22719

	decoded := make([]byte, len(encoded))

	for i, j := range encoded {
		cipher = j
		decoded[i] = byte(uint16(cipher) ^ (r >> 8))
		r = (uint16(cipher)+r)*c1 + c2
	}
	return decoded[4:]
}

// Encode the bytes in decoded with the given prefix.
func encodeEexec(prefix, decoded []byte) []byte {
	var tmp1, tmp2 []byte
	r := uint16(55665)

	r, tmp1 = encodeBytes(r, prefix)
	_, tmp2 = encodeBytes(r, decoded)
	encoded := append(tmp1, tmp2...)
	return encoded
}

// Encode the bytes with the given start value of r and return
// the new value of r and the encoded bytes. This is necessary to concatenate
// eexec encoded bytes.
func encodeBytes(r uint16, decoded []byte) (uint16, []byte) {
	var c1, c2, cipher uint16
	c1 = 52845
	c2 = 22719
	encoded := make([]byte, len(decoded))
	for i, plain := range decoded {
		cipher = uint16(plain) ^ (r >> 8)
		r = (uint16(cipher)+r)*c1 + c2
		encoded[i] = byte(cipher)
	}
	return r, encoded
}

// Write the PFB to the given io.Writer
func (t *Type1) Write(w io.Writer) error {
	var err error
	for i := 0; i < 3; i++ {
		err = t.writeSegment(w, i)
		if err != nil {
			return err
		}
	}
	postfix := []byte{128, 3}
	_, err = w.Write(postfix)
	return err
}

// WriteFile writes the PFB file to given file name.
func (t *Type1) WriteFile(filename string) error {
	w, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer w.Close()
	return t.Write(w)
}

// Write the given segment. The second segment (index 1) is binary.
func (t *Type1) writeSegment(w io.Writer, segment int) error {
	l := len(t.Segments[segment])
	var asciiBinary byte
	if segment == 1 {
		asciiBinary = 2
	} else {
		asciiBinary = 1
	}
	prefix := []byte{128, asciiBinary, byte(l & 0xFF), byte(l >> 8 & 0xFF), byte(l >> 16 & 0xFF), byte(l >> 24 & 0xFF)}
	_, err := w.Write(prefix)
	if err != nil {
		return err
	}
	_, err = w.Write(t.Segments[segment])
	return err
}

// LoadFont opens a Type1 PostScript font.
// The PFB file must be given, the afm file is optional.
// If it is empty, it is deduced from the given PFB file.
func LoadFont(pfbfilename string, afmfilename string) (*Type1, error) {
	pfbfilenameWithoutExt := trimSuffix(pfbfilename)
	possiblePFBFilenames := []string{pfbfilename, pfbfilenameWithoutExt + ".PFB", pfbfilenameWithoutExt + ".pfb"}

	possibleAFMFilenames := []string{}
	if afmfilename == "" {
		// Let's try to find a suitable AFM file
		possibleAFMFilenames = append(possibleAFMFilenames, pfbfilenameWithoutExt+".AFM")
		possibleAFMFilenames = append(possibleAFMFilenames, pfbfilenameWithoutExt+".afm")
	} else {
		possibleAFMFilenames = append(possibleAFMFilenames, afmfilename)
	}

	pfb, err := tryOpen(possiblePFBFilenames)
	if err != nil {
		return nil, err
	}
	defer pfb.Close()
	t := &Type1{}
	t.ParsePFB(pfb)

	afm, err := tryOpen(possibleAFMFilenames)
	if err == nil {
		t.ParseAFM(afm)
	}
	afm.Close()
	return t, nil
}

// Try to open one of the given files. If a file is available,
// the function returns an open file, which must be Close()d.
// If no file is present, it returns nil and an error.
func tryOpen(filenames []string) (*os.File, error) {
	for _, v := range filenames {
		f, err := os.Open(v)
		if err == nil {
			return f, nil
		}
	}
	return nil, fmt.Errorf("cannot find file %q", filenames)
}
