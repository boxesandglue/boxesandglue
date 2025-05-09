package document

import (
	_ "embed" // embed is used to embed the default color profile
	"io"
	"os"
)

// ColorProfile represents a color profile
type ColorProfile struct {
	Identifier string
	Registry   string
	Info       string
	Condition  string
	Colors     int
	data       []byte
}

func (cp *ColorProfile) String() string {
	return "color profile:" + cp.Identifier
}

//go:embed ISOcoated_v2_eci.icc
var b []byte

// LoadColorprofile loads an icc based color profile from the URL.
func (d *PDFDocument) LoadColorprofile(filename string) (*ColorProfile, error) {
	r, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	cp := &ColorProfile{data: data}
	d.ColorProfile = cp
	return cp, nil
}

// LoadDefaultColorprofile load the default ISOcoated_v2_eci.icc color profile
func (d *PDFDocument) LoadDefaultColorprofile() (*ColorProfile, error) {
	cp := &ColorProfile{
		Identifier: "FOGRA39",
		Registry:   "http://www.color.org",
		Info:       "Coated FOGRA39 (ISO 12647-2:2004)",
		Condition:  "Offset printing, according to ISO 12647-2:2004/Amd 1, OFCOM, paper type 1 or 2 = coated art, 115 g/m2, tone value increase curves A (CMY) and B (K)",
		Colors:     4,
		data:       b,
	}
	d.ColorProfile = cp
	return cp, nil
}
