package truetype

import (
	"fmt"

	"github.com/speedata/boxesandglue/backend/bag"
	bf "github.com/speedata/boxesandglue/backend/font"
)

// TrueType holds information about a TrueType or OpenType font at a specific size
type TrueType struct {
	fontobject *bf.Font
}

// LoadFont loads a truetype font
func LoadFont(fn string, size bag.ScaledPoint) (*TrueType, error) {
	f, err := bf.LoadFont(fn, size)
	if err != nil {
		return nil, err
	}
	tt := &TrueType{
		fontobject: f,
	}
	return tt, nil
}

// CMap returns a PDF string to be used as a CMap object
func (tt *TrueType) CMap() {
	glyphs := `Hello äö`
	face := tt.fontobject.Face
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
