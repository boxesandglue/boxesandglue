package frontend

import (
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

// DefinedColor returns the color that DefineColor knows under name, nil for
// anything else. The colors that GetColor keeps under their CSS name do not
// count: whether a name is defined must not depend on what was used before.
func (d *Document) DefinedColor(name string) *color.Color {
	for _, n := range []string{name, strings.ToLower(strings.TrimSpace(name))} {
		if col, ok := d.usedcolors[n]; ok && col != csscolors[n] {
			return col
		}
	}
	return nil
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

// parseHue accepts an angle with the units of CSS, a number means degrees, and
// returns the degrees from 0 up to 360.
func parseHue(s string) (float64, bool) {
	scale := 1.0
	for _, unit := range []struct {
		suffix string
		scale  float64
	}{{"deg", 1}, {"grad", 0.9}, {"rad", 180 / math.Pi}, {"turn", 360}} {
		if bare, ok := strings.CutSuffix(s, unit.suffix); ok {
			s, scale = bare, unit.scale
			break
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	f = math.Mod(f*scale, 360)
	if f < 0 {
		f += 360
	}
	return f, true
}

// parseHSLPercentage accepts the saturation or the lightness of hsl(), with or
// without a percent sign, and returns the value clamped to 0..1.
func parseHSLPercentage(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
	if err != nil {
		return 0, false
	}
	return max(0, min(f/100, 1)), true
}

// hslToRGB converts a hue in degrees and a saturation and a lightness in 0..1,
// see CSS Color 4 section 7.1.
func hslToRGB(hue, saturation, lightness float64) (r, g, b float64) {
	channel := func(n float64) float64 {
		k := math.Mod(n+hue/30, 12)
		a := saturation * min(lightness, 1-lightness)
		return lightness - a*max(-1, min(k-3, 9-k, 1))
	}
	return channel(0), channel(8), channel(4)
}

// parseHexColor returns the components of a CSS hex color after the #.
func parseHexColor(digits string) (r, g, b, a int, ok bool) {
	if len(digits) == 3 || len(digits) == 4 {
		// CSS Color 3 §4.2.1: 3-digit hex (#RGB) expands by duplicating
		// each digit (#abc → #aabbcc), not by zero-padding. CSS Color 4
		// §5.2 adds the alpha digit of #RGBA.
		var long strings.Builder
		for _, digit := range []byte(digits) {
			long.WriteByte(digit)
			long.WriteByte(digit)
		}
		digits = long.String()
	}
	if len(digits) == 6 {
		digits += "ff"
	}
	if len(digits) != 8 {
		return 0, 0, 0, 0, false
	}
	value, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return 0, 0, 0, 0, false
	}
	return int(value >> 24), int(value >> 16 & 0xff), int(value >> 8 & 0xff), int(value & 0xff), true
}

// colorKeywords are valid values of the color properties of CSS that name no
// color by themselves. The cascade deals with them, or the callers do.
var colorKeywords = map[string]bool{
	"":             true,
	"currentcolor": true,
	"inherit":      true,
	"initial":      true,
	"none":         true,
	"revert":       true,
	"revert-layer": true,
	"unset":        true,
}

// warnColor reports a color that cannot be used, once per value.
func (d *Document) warnColor(s string) {
	if colorKeywords[strings.ToLower(strings.TrimSpace(s))] || d.warnedColors[s] {
		return
	}
	if d.warnedColors == nil {
		d.warnedColors = make(map[string]bool)
	}
	d.warnedColors[s] = true
	bag.Logger.Warn("Cannot parse color, ignoring it", "color", s)
}

// GetColor returns a color, or nil with a warning if it cannot use s. The
// string can be a predefined color name or an HTML / CSS color definition such
// as #FAF, rgb(128,128,128), hsl(120 100% 25%), cmyk(0%,0%,0%,100%) or
// device-cmyk(0 0 0 1). All of them take an alpha value: #FAF8, #FFAAFF80,
// rgba(128,128,128,0.5), rgb(128 128 128 / 50%), hsl(120 100% 25% / 50%),
// device-cmyk(0 0 0 1 / 50%). Names and functions ignore the case, as in CSS,
// but a name given to DefineColor counts as it is.
func (d *Document) GetColor(s string) *color.Color {
	col := d.parseColor(s)
	if col == nil {
		d.warnColor(s)
	}
	return col
}

// parseColor is GetColor without the warning.
func (d *Document) parseColor(s string) *color.Color {
	if col, ok := d.usedcolors[s]; ok {
		return col
	}
	s = strings.ToLower(strings.TrimSpace(s))
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
	col := &color.Color{}
	if digits, ok := strings.CutPrefix(s, "#"); ok {
		r, g, b, a, ok := parseHexColor(digits)
		if !ok {
			return nil
		}
		col.Space = color.ColorRGB
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
	} else if strings.HasPrefix(s, "hsl") {
		parts, alpha, ok := colorArguments(s, 3)
		if !ok {
			return nil
		}
		hue, ok := parseHue(parts[0])
		if !ok {
			return nil
		}
		saturation, ok := parseHSLPercentage(parts[1])
		if !ok {
			return nil
		}
		lightness, ok := parseHSLPercentage(parts[2])
		if !ok {
			return nil
		}
		rv, gv, bv := hslToRGB(hue, saturation, lightness)
		col.Space = color.ColorRGB
		col.R = math.Round(100.0*rv) / 100.0
		col.G = math.Round(100.0*gv) / 100.0
		col.B = math.Round(100.0*bv) / 100.0
		col.Transparency = 1 - alpha
		d.useTransparency(col)
		return col
	}
	return nil
}
