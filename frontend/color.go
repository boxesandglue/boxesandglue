package frontend

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
)

// DefineColor associates a color with a name for later use. A spot color is
// registered with the document so that the PDF gets its Separation color
// space; its Basecolor (the name of the ink) defaults to the given name.
func (d *Document) DefineColor(name string, col *color.Color) {
	if d.usedcolors == nil {
		d.usedcolors = make(map[string]*color.Color)
	}
	if col.Space == color.ColorSpotcolor && col.SpotcolorID == 0 {
		if col.Basecolor == "" {
			col.Basecolor = name
		}
		if d.usedSpotcolors == nil {
			d.usedSpotcolors = make(map[*color.Color]bool)
		}
		d.usedSpotcolors[col] = true
		col.SpotcolorID = len(d.usedSpotcolors)
	}
	if col.Overprint {
		d.Doc.Overprint = true
		warnOverprint(name, col)
	}
	d.useTransparency(col)
	d.usedcolors[name] = col
}

// useTransparency prepares the pages for the transparent color col. In a format
// that forbids transparency the color turns opaque: the format is what the
// reader of the file relies on.
func (d *Document) useTransparency(col *color.Color) {
	if col == nil || col.Transparency == 0 || d.Doc == nil {
		return
	}
	if d.Doc.Format.AllowsTransparency() {
		d.Doc.AddColorStates(col)
		return
	}
	if !d.warnedTransparency {
		d.warnedTransparency = true
		bag.Logger.Warn("The PDF format does not allow transparency, painting transparent colors opaque")
	}
	col.Transparency = 0
}

// warnOverprint reports overprinting colors that cannot do what they ask for.
func warnOverprint(name string, col *color.Color) {
	switch col.Space {
	case color.ColorCMYK:
		if col.C == 0 && col.M == 0 && col.Y == 0 && col.K == 0 {
			bag.Logger.Warn("Color is white and overprints, it leaves no mark", "color", name)
		}
	case color.ColorSpotcolor:
	default:
		bag.Logger.Warn("Only CMYK and spot colors overprint, this color knocks out", "color", name)
	}
}

// parseRGBComponent accepts a number from 0 to 255 or a percentage and returns
// the number. The value is clamped as in CSS Color 4 section 5.1.
func parseRGBComponent(s string) (float64, bool) {
	var f float64
	var err error
	if bare, ok := strings.CutSuffix(s, "%"); ok {
		f, err = strconv.ParseFloat(bare, 64)
		f = f * 255 / 100
	} else {
		f, err = strconv.ParseFloat(s, 64)
	}
	if err != nil {
		return 0, false
	}
	return max(0, min(f, 255)), true
}

// colorArguments splits the arguments of a color function into its n
// components and the alpha value. The legacy form separates all of them with
// commas, the modern form uses blanks and a slash in front of the alpha value:
// rgb(0,128,0,0.5) and rgb(0 128 0 / 50%). Without an alpha value the color is
// opaque.
func colorArguments(s string, n int) (components []string, alpha float64, ok bool) {
	i := strings.IndexByte(s, '(')
	j := strings.LastIndexByte(s, ')')
	if i < 0 || j <= i {
		return nil, 0, false
	}
	inner := strings.ReplaceAll(s[i+1:j], "/", " ")
	parts := strings.Fields(strings.ReplaceAll(inner, ",", " "))
	switch len(parts) {
	case n:
		return parts, 1, true
	case n + 1:
		if alpha, ok = parseCMYKComponent(parts[n]); ok {
			return parts[:n], max(0, min(alpha, 1)), true
		}
	}
	return nil, 0, false
}

// parseCMYKComponent accepts CSS-style CMYK components — a percentage
// ("36%" → 0.36) or a unit-less number ("0.5" → 0.5). Returns the 0..1
// value and ok=false on malformed input. CSS Color 5 §6.1 specifies
// numbers in 0..1; the legacy cmyk()-function (Prince / Antenna House)
// uses percentages — both cases drop into the same parser.
func parseCMYKComponent(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if bare, ok := strings.CutSuffix(s, "%"); ok {
		f, err := strconv.ParseFloat(bare, 64)
		if err != nil {
			return 0, false
		}
		return f / 100.0, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// GetColor returns a color. The string can be a predefined color name or an
// HTML / CSS color definition such as #FAF, rgb(128,128,128), cmyk(0%,0%,0%,100%)
// or device-cmyk(0 0 0 1). All of them take an alpha value: #FAF8, #FFAAFF80,
// rgba(128,128,128,0.5), rgb(128 128 128 / 50%), device-cmyk(0 0 0 1 / 50%).
func (d *Document) GetColor(s string) *color.Color {
	if col, ok := d.usedcolors[s]; ok {
		return col
	}
	if col, ok := csscolors[s]; ok {
		if col.Space == color.ColorSpotcolor {
			d.usedSpotcolors[col] = true
			col.SpotcolorID = len(d.usedSpotcolors)
		}
		d.usedcolors[s] = col
		return col
	}
	s = strings.TrimSpace(s)
	var r, g, b, a int
	col := &color.Color{}
	if strings.HasPrefix(s, "#") {
		col.Space = color.ColorRGB
		a = 255
		switch len(s) {
		case 7:
			fmt.Sscanf(s, "#%2x%2x%2x", &r, &g, &b)
		case 9:
			fmt.Sscanf(s, "#%2x%2x%2x%2x", &r, &g, &b, &a)
		case 4, 5:
			// CSS Color 3 §4.2.1: 3-digit hex (#RGB) expands by
			// duplicating each digit (#abc → #aabbcc), not by zero-
			// padding. Equivalent: shift the nibble into the high
			// half so 0xd becomes 0xdd. CSS Color 4 §5.2 adds the
			// alpha digit of #RGBA.
			a = 15
			fmt.Sscanf(s, "#%1x%1x%1x%1x", &r, &g, &b, &a)
			r = r<<4 | r
			g = g<<4 | g
			b = b<<4 | b
			a = a<<4 | a
		default:
			// FIXME: proper error handling
			// logger.Error("GetColor HTML not implemented", "size", len(s))
		}
		col.R = math.Round(100.0*float64(r)/float64(255)) / 100.0
		col.G = math.Round(100.0*float64(g)/float64(255)) / 100.0
		col.B = math.Round(100.0*float64(b)/float64(255)) / 100.0
		col.Transparency = 1 - math.Round(100.0*float64(a)/float64(255))/100.0
		d.useTransparency(col)
		return col
	} else if strings.HasPrefix(s, "cmyk") || strings.HasPrefix(s, "device-cmyk") {
		parts, alpha, ok := colorArguments(s, 4)
		if !ok {
			return nil
		}
		cv, ok := parseCMYKComponent(parts[0])
		if !ok {
			return nil
		}
		mv, ok := parseCMYKComponent(parts[1])
		if !ok {
			return nil
		}
		yv, ok := parseCMYKComponent(parts[2])
		if !ok {
			return nil
		}
		kv, ok := parseCMYKComponent(parts[3])
		if !ok {
			return nil
		}
		col.Space = color.ColorCMYK
		col.C, col.M, col.Y, col.K = cv, mv, yv, kv
		col.Transparency = 1 - alpha
		d.useTransparency(col)
		return col
	} else if strings.HasPrefix(s, "rgb") {
		parts, alpha, ok := colorArguments(s, 3)
		if !ok {
			return nil
		}
		rv, ok := parseRGBComponent(parts[0])
		if !ok {
			return nil
		}
		gv, ok := parseRGBComponent(parts[1])
		if !ok {
			return nil
		}
		bv, ok := parseRGBComponent(parts[2])
		if !ok {
			return nil
		}
		col.Space = color.ColorRGB
		col.R = math.Round(100.0*rv/255) / 100.0
		col.G = math.Round(100.0*gv/255) / 100.0
		col.B = math.Round(100.0*bv/255) / 100.0
		col.Transparency = 1 - alpha
		d.useTransparency(col)
		return col
	}
	return nil
}
