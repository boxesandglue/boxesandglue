package document

import (
	"fmt"
	"strconv"

	"github.com/speedata/boxesandglue/backend/bag"
)

// Color holds color values for the document. All intensities are from 0 to 1.
// Basecolor is a spot color name such as Pantone 119 for example.
type Color struct {
	Space       ColorSpace
	Basecolor   string
	Spotcolorid int
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

// ColorSpace represents the color space of a defined color.
type ColorSpace int

const (
	// ColorNone represents an undefined color space
	ColorNone ColorSpace = iota
	// ColorRGB represents a color in RGB color space
	ColorRGB
	// ColorCMYK represents a color in CMYK color space
	ColorCMYK
	// ColorGray represents a gray scale color.
	ColorGray
	// ColorSpotcolor represents a spot color.
	ColorSpotcolor
)

func (col *Color) getPDFColorSuffix(fg bool) string {
	if fg {
		// keep the order
		return []string{"", "rg", "k", "g", "cs"}[col.Space]
	}
	return []string{"", "RG", "K", "G", "CS"}[col.Space]
}

func (col *Color) getPDFColorValues(stroking bool) string {
	if col == nil {
		return ""
	}
	switch col.Space {
	case ColorNone:
		return ""
	case ColorRGB:
		return fmt.Sprintf("%s %s %s %s", strconv.FormatFloat(col.R, 'f', -1, 64), strconv.FormatFloat(col.G, 'f', -1, 64), strconv.FormatFloat(col.B, 'f', -1, 64), col.getPDFColorSuffix(stroking))
	case ColorCMYK:
		return fmt.Sprintf("%s %s %s %s %s", strconv.FormatFloat(col.C, 'f', -1, 64), strconv.FormatFloat(col.M, 'f', -1, 64), strconv.FormatFloat(col.Y, 'f', -1, 64), strconv.FormatFloat(col.K, 'f', -1, 64), col.getPDFColorSuffix(stroking))
	case ColorGray:
		return fmt.Sprintf("%s %s", strconv.FormatFloat(col.G, 'f', -1, 64), col.getPDFColorSuffix(stroking))
	case ColorSpotcolor:
		return fmt.Sprintf("/CS%d %s 1 scn ", col.Spotcolorid, col.getPDFColorSuffix(stroking))
	default:
		bag.Logger.DPanic("PDFStringFG: unknown color space.")
		return ""
	}
}

// PDFStringFG returns the PDF instructions to swith to the color for foreground colors.
func (col *Color) PDFStringFG() string {
	return col.getPDFColorValues(true)
}

// PDFStringBG returns the PDF instructions to swith to the color for background colors.
func (col *Color) PDFStringBG() string {
	return col.getPDFColorValues(false)
}
