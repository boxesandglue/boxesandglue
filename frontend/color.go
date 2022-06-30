package frontend

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/color"
)

// DefineColor associates a color with a name for later use.
func (d *Document) DefineColor(name string, col *color.Color) {
	d.usedcolors[name] = col
}

var rgbmatcher = regexp.MustCompile(`rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,?\s*([0-9.-]*)?\s*\)`)

// GetColor returns a color. The string can be a predefined color name or an
// HTML / CSS color definition such as #FAF or rgb(0.5.,0.5,0.5).
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
	var r, g, b int
	var alpha float64
	var err error
	col := &color.Color{}
	if strings.HasPrefix(s, "#") {
		col.Space = color.ColorRGB
		switch len(s) {
		case 7:
			fmt.Sscanf(s, "#%2x%2x%2x", &r, &g, &b)
		case 4:
			fmt.Sscanf(s, "#%1x%1x%1x", &r, &g, &b)
		default:
			bag.Logger.DPanicf("GetColor HTML not implemented size %s", len(s))
		}
		col.R = math.Round(100.0*float64(r)/float64(255)) / 100.0
		col.G = math.Round(100.0*float64(g)/float64(255)) / 100.0
		col.B = math.Round(100.0*float64(b)/float64(255)) / 100.0
		return col
	} else if strings.HasPrefix(s, "rgb") {
		col.Space = color.ColorRGB
		colorvalues := rgbmatcher.FindAllStringSubmatch(s, -1)

		if len(colorvalues) == 1 {
			if len(colorvalues[0]) > 3 {
				// TODO: percentage
				if r, err = strconv.Atoi(colorvalues[0][1]); err != nil {
					return nil
				}
				if g, err = strconv.Atoi(colorvalues[0][2]); err != nil {
					return nil
				}
				if b, err = strconv.Atoi(colorvalues[0][3]); err != nil {
					return nil
				}
				col.R = math.Round(100.0*float64(r)/float64(255)) / 100.0
				col.G = math.Round(100.0*float64(g)/float64(255)) / 100.0
				col.B = math.Round(100.0*float64(b)/float64(255)) / 100.0
				if alpha, err = strconv.ParseFloat(colorvalues[0][4], 64); err == nil {
					col.A = alpha
				}
				return col
			}
		}
	}
	return nil
}
