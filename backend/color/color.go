package color

import (
	"fmt"
	"strconv"
)

// Color holds color values for the document. All intensities are from 0 to 1.
// Basecolor is a spot color name such as Pantone 119 for example.
type Color struct {
	Space       Space
	Basecolor   string
	SpotcolorID int
	C           float64
	M           float64
	Y           float64
	K           float64
	R           float64
	G           float64
	B           float64
	A           float64
}

func (col Color) String() string {
	switch col.Space {
	case ColorNone:
		return "-"
	case ColorRGB:
		alphaStr := strconv.FormatFloat(col.A, 'f', -1, 64)
		return fmt.Sprintf("rgba(%d,%d,%d,%s)", int(col.R*255), int(col.G*255), int(col.B*255), alphaStr)
	case ColorSpotcolor:
		return col.Basecolor
	}
	return ""
}

// Space represents the color space of a defined color.
type Space int

const (
	// ColorNone represents an undefined color space
	ColorNone Space = iota
	// ColorRGB represents a color in RGB color space
	ColorRGB
	// ColorCMYK represents a color in CMYK color space
	ColorCMYK
	// ColorGray represents a gray space color.
	ColorGray
	// ColorSpotcolor represents a spot color.
	ColorSpotcolor
)

func (col *Color) getPDFColorSuffix(stroking bool) string {
	if stroking {
		// keep the order
		return []string{"", "RG", "K", "G", "CS"}[col.Space]
	}
	return []string{"", "rg", "k", "g", "cs"}[col.Space]
}

// getPDFColorValues returns the PDF string for this color. If color is in color
// space none or not in the spaces RGG, CMYK, gray or spotcolor, it will return
// the empty string.
func (col *Color) getPDFColorValues(stroking bool) string {
	if col == nil {
		return ""
	}
	switch col.Space {
	case ColorRGB:
		return fmt.Sprintf("%s %s %s %s", strconv.FormatFloat(col.R, 'f', -1, 64), strconv.FormatFloat(col.G, 'f', -1, 64), strconv.FormatFloat(col.B, 'f', -1, 64), col.getPDFColorSuffix(stroking))
	case ColorCMYK:
		return fmt.Sprintf("%s %s %s %s %s", strconv.FormatFloat(col.C, 'f', -1, 64), strconv.FormatFloat(col.M, 'f', -1, 64), strconv.FormatFloat(col.Y, 'f', -1, 64), strconv.FormatFloat(col.K, 'f', -1, 64), col.getPDFColorSuffix(stroking))
	case ColorGray:
		return fmt.Sprintf("%s %s", strconv.FormatFloat(col.G, 'f', -1, 64), col.getPDFColorSuffix(stroking))
	case ColorSpotcolor:
		return fmt.Sprintf("/CS%d %s 1 scn ", col.SpotcolorID, col.getPDFColorSuffix(stroking))
	default:
		return ""
	}
}

// PDFStringStroking returns the PDF instructions to switch to the color for
// stroking colors.
func (col *Color) PDFStringStroking() string {
	return col.getPDFColorValues(true)
}

// PDFStringNonStroking returns the PDF instructions to switch to the color for
// non-stroking (filling) colors.
func (col *Color) PDFStringNonStroking() string {
	return col.getPDFColorValues(false)
}
