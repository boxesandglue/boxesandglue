package truetype

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"

	"github.com/speedata/boxesandglue/backend/font"
)

// TrueType holds information about a TrueType or OpenType font at a specific size
type TrueType struct {
	face     *font.Face
	SubsetID string
}

// LoadFace loads a truetype font
func LoadFace(filename string) (*TrueType, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	bf, err := font.NewFaceFromData(filename, data)
	if err != nil {
		return nil, err
	}

	tt := &TrueType{
		face: bf,
	}
	return tt, nil
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

// Subset returns a truetype font subset
func (tt *TrueType) Subset(runelist []rune) (string, error) {
	return "abcde", nil
}

// CMap returns a PDF string to be used as a CMap object
func (tt *TrueType) CMap() {
	glyphs := `Hello äö`
	face := tt.face
	for _, r := range glyphs {
		face.GetIndex(r)
	}

	fmt.Println(`/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo << /Registry (Adobe)/Ordering`)
	for k, v := range face.ToRune {
		fmt.Printf("<%04X><%04X><%04X>\n", k, k, v)
	}
}
