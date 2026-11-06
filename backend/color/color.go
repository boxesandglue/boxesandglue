package color

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	pdf "github.com/boxesandglue/baseline-pdf"
)

// formatColorComponent renders a 0..1 color component for PDF content
// streams. 4 decimal places give a resolution of 0.0001 — finer than the
// 8-bit input (1/255 ≈ 0.0039) — while keeping content streams compact
// (`0.5294` instead of `0.5294117647058824`). Trailing zeros and an empty
// fractional part are trimmed (`0.5000` → `0.5`, `1.0000` → `1`).
func formatColorComponent(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	if strings.ContainsRune(s, '.') {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// Color holds color values for the document. All intensities are from 0 to 1.
// Basecolor is a spot color name such as Pantone 119 for example.
type Color struct {
	Basecolor   string
	Space       Space
	SpotcolorID int
	C           float64
	M           float64
	Y           float64
	K           float64
	R           float64
	G           float64
	B           float64
	// Transparency is 0 for an opaque color, the zero value, and 1 for an
	// invisible one. Like a spot color, a transparent color has to go through
	// Document.DefineColor or Document.GetColor, which set up the resources
	// the page needs and refuse transparency in formats that forbid it.
	Transparency float64
	// Overprint prints the color on top of what is below instead of knocking
	// it out. It has an effect on CMYK and spot colors. Like a spot color, the
	// color has to go through Document.DefineColor, which sets up the
	// resources the page needs.
	Overprint bool
}

func (col Color) String() string {
	switch col.Space {
	case ColorNone:
		return "-"
	case ColorRGB:
		alphaStr := strconv.FormatFloat(col.Alpha(), 'f', -1, 64)
		return fmt.Sprintf("rgba(%d,%d,%d,%s)", int(col.R*255), int(col.G*255), int(col.B*255), alphaStr)
	case ColorSpotcolor:
		return col.Basecolor
	}
	return ""
}

// Alpha returns the opacity of the color, 1 for an opaque one. It is rounded
// to the precision of the color components, so that the alpha of a color does
// not depend on the way it was computed and equal colors share one graphics
// state.
func (col Color) Alpha() float64 {
	return math.Round((1-col.Transparency)*10000) / 10000
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
	var values string
	switch col.Space {
	case ColorRGB:
		values = fmt.Sprintf("%s %s %s %s", formatColorComponent(col.R), formatColorComponent(col.G), formatColorComponent(col.B), col.getPDFColorSuffix(stroking))
	case ColorCMYK:
		values = fmt.Sprintf("%s %s %s %s %s", formatColorComponent(col.C), formatColorComponent(col.M), formatColorComponent(col.Y), formatColorComponent(col.K), col.getPDFColorSuffix(stroking))
	case ColorGray:
		values = fmt.Sprintf("%s %s", formatColorComponent(col.G), col.getPDFColorSuffix(stroking))
	case ColorSpotcolor:
		values = fmt.Sprintf("/CS%d %s 1 scn ", col.SpotcolorID, col.getPDFColorSuffix(stroking))
	default:
		return ""
	}
	if col.Overprint {
		values = gsOperator(pdf.OverprintOn) + values
	}
	if col.Transparency != 0 {
		values = gsOperator(alphaState(col.Alpha(), stroking)) + values
	}
	return values
}

// alphaEnd is the graphics state that ends a transparent color. It sets both
// alphas, as a run does not know which of them its color has set.
var alphaEnd = func() pdf.ExtGState {
	opaque := 1.0
	return pdf.ExtGState{FillAlpha: &opaque, StrokeAlpha: &opaque}
}()

// alphaState returns the graphics state for alpha. The alpha for filling and
// the one for stroking are separate states, like the two colors: setting one
// must not touch the other.
func alphaState(alpha float64, stroking bool) pdf.ExtGState {
	if stroking {
		return pdf.ExtGState{StrokeAlpha: &alpha}
	}
	return pdf.ExtGState{FillAlpha: &alpha}
}

// ExtGStates returns the graphics states the operators of a transparent color
// refer to. A page has to list them in its resources.
func (col *Color) ExtGStates() []pdf.ExtGState {
	if col == nil || col.Transparency == 0 {
		return nil
	}
	return []pdf.ExtGState{alphaState(col.Alpha(), false), alphaState(col.Alpha(), true), alphaEnd}
}

// gsOperator returns the PDF instruction that activates gs.
func gsOperator(gs pdf.ExtGState) string {
	return "/" + string(gs.ResourceName()) + " gs "
}

// PDFStringEnd returns the PDF instructions a color needs at its end, beyond
// switching to another color. The graphics state of an overprinting or a
// transparent color outlives the color operators unless it is reset.
func (col *Color) PDFStringEnd() string {
	if col == nil {
		return ""
	}
	var end string
	if col.Overprint {
		end = gsOperator(pdf.OverprintOff)
	}
	if col.Transparency != 0 {
		end += gsOperator(alphaEnd)
	}
	return end
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
