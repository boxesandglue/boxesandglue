package document

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
)

// Color holds color values for the document. All intensities are from 0 to 1.
type Color struct {
	Space ColorSpace
	C     float64
	M     float64
	Y     float64
	K     float64
	R     float64
	G     float64
	B     float64
	A     float64
}

var (
	csscolors = map[string]*Color{
		"aliceblue":            {Space: ColorRGB, R: 0.941, G: 0.973, B: 1, A: 1},
		"antiquewhite":         {Space: ColorRGB, R: 0.98, G: 0.922, B: 0.843, A: 1},
		"aqua":                 {Space: ColorRGB, R: 0, G: 1, B: 1, A: 1},
		"aquamarine":           {Space: ColorRGB, R: 0.498, G: 1, B: 0.831, A: 1},
		"azure":                {Space: ColorRGB, R: 0.941, G: 1, B: 1, A: 1},
		"beige":                {Space: ColorRGB, R: 0.961, G: 0.961, B: 0.863, A: 1},
		"bisque":               {Space: ColorRGB, R: 1, G: 0.894, B: 0.769, A: 1},
		"black":                {Space: ColorGray, R: 0, G: 0, B: 0, A: 1},
		"blanchedalmond":       {Space: ColorRGB, R: 1, G: 0.894, B: 0.769, A: 1},
		"blue":                 {Space: ColorRGB, R: 0, G: 0, B: 1, A: 1},
		"blueviolet":           {Space: ColorRGB, R: 0.541, G: 0.169, B: 0.886, A: 1},
		"brown":                {Space: ColorRGB, R: 0.647, G: 0.165, B: 0.165, A: 1},
		"burlywood":            {Space: ColorRGB, R: 0.871, G: 0.722, B: 0.529, A: 1},
		"cadetblue":            {Space: ColorRGB, R: 0.373, G: 0.62, B: 0.627, A: 1},
		"chartreuse":           {Space: ColorRGB, R: 0.498, G: 1, B: 0, A: 1},
		"chocolate":            {Space: ColorRGB, R: 0.824, G: 0.412, B: 0.118, A: 1},
		"coral":                {Space: ColorRGB, R: 1, G: 0.498, B: 0.314, A: 1},
		"cornflowerblue":       {Space: ColorRGB, R: 0.392, G: 0.584, B: 0.929, A: 1},
		"cornsilk":             {Space: ColorRGB, R: 1, G: 0.973, B: 0.863, A: 1},
		"crimson":              {Space: ColorRGB, R: 0.863, G: 0.078, B: 0.235, A: 1},
		"darkblue":             {Space: ColorRGB, R: 0, G: 0, B: 0.545, A: 1},
		"darkcyan":             {Space: ColorRGB, R: 0, G: 0.545, B: 0.545, A: 1},
		"darkgoldenrod":        {Space: ColorRGB, R: 0.722, G: 0.525, B: 0.043, A: 1},
		"darkgray":             {Space: ColorRGB, R: 0.663, G: 0.663, B: 0.663, A: 1},
		"darkgreen":            {Space: ColorRGB, R: 0, G: 0.392, B: 0, A: 1},
		"darkgrey":             {Space: ColorRGB, R: 0.663, G: 0.663, B: 0.663, A: 1},
		"darkkhaki":            {Space: ColorRGB, R: 0.741, G: 0.718, B: 0.42, A: 1},
		"darkmagenta":          {Space: ColorRGB, R: 0.545, G: 0, B: 0.545, A: 1},
		"darkolivegreen":       {Space: ColorRGB, R: 0.333, G: 0.42, B: 0.184, A: 1},
		"darkorange":           {Space: ColorRGB, R: 1, G: 0.549, B: 0, A: 1},
		"darkorchid":           {Space: ColorRGB, R: 0.6, G: 0.196, B: 0.8, A: 1},
		"darkred":              {Space: ColorRGB, R: 0.545, G: 0, B: 0, A: 1},
		"darksalmon":           {Space: ColorRGB, R: 0.914, G: 0.588, B: 0.478, A: 1},
		"darkseagreen":         {Space: ColorRGB, R: 0.561, G: 0.737, B: 0.561, A: 1},
		"darkslateblue":        {Space: ColorRGB, R: 0.282, G: 0.239, B: 0.545, A: 1},
		"darkslategray":        {Space: ColorRGB, R: 0.184, G: 0.31, B: 0.31, A: 1},
		"darkslategrey":        {Space: ColorRGB, R: 0.184, G: 0.31, B: 0.31, A: 1},
		"darkturquoise":        {Space: ColorRGB, R: 0, G: 0.808, B: 0.82, A: 1},
		"darkviolet":           {Space: ColorRGB, R: 0.58, G: 0, B: 0.827, A: 1},
		"deeppink":             {Space: ColorRGB, R: 1, G: 0.078, B: 0.576, A: 1},
		"deepskyblue":          {Space: ColorRGB, R: 0, G: 0.749, B: 1, A: 1},
		"dimgray":              {Space: ColorRGB, R: 0.412, G: 0.412, B: 0.412, A: 1},
		"dimgrey":              {Space: ColorRGB, R: 0.412, G: 0.412, B: 0.412, A: 1},
		"dodgerblue":           {Space: ColorRGB, R: 0.118, G: 0.565, B: 1, A: 1},
		"firebrick":            {Space: ColorRGB, R: 0.698, G: 0.133, B: 0.133, A: 1},
		"floralwhite":          {Space: ColorRGB, R: 1, G: 0.98, B: 0.941, A: 1},
		"forestgreen":          {Space: ColorRGB, R: 0.133, G: 0.545, B: 0.133, A: 1},
		"fuchsia":              {Space: ColorRGB, R: 1, G: 0, B: 1, A: 1},
		"gainsboro":            {Space: ColorRGB, R: 0.863, G: 0.863, B: 0.863, A: 1},
		"ghostwhite":           {Space: ColorRGB, R: 0.973, G: 0.973, B: 1, A: 1},
		"gold":                 {Space: ColorRGB, R: 1, G: 0.843, B: 0, A: 1},
		"goldenrod":            {Space: ColorRGB, R: 0.855, G: 0.647, B: 0.125, A: 1},
		"gray":                 {Space: ColorRGB, R: 0.502, G: 0.502, B: 0.502, A: 1},
		"green":                {Space: ColorRGB, R: 0, G: 0.502, B: 0, A: 1},
		"greenyellow":          {Space: ColorRGB, R: 0.678, G: 1, B: 0.184, A: 1},
		"grey":                 {Space: ColorRGB, R: 0.502, G: 0.502, B: 0.502, A: 1},
		"honeydew":             {Space: ColorRGB, R: 0.941, G: 1, B: 0.941, A: 1},
		"hotpink":              {Space: ColorRGB, R: 1, G: 0.412, B: 0.706, A: 1},
		"indianred":            {Space: ColorRGB, R: 0.804, G: 0.361, B: 0.361, A: 1},
		"indigo":               {Space: ColorRGB, R: 0.294, G: 0, B: 0.51, A: 1},
		"ivory":                {Space: ColorRGB, R: 1, G: 1, B: 0.941, A: 1},
		"khaki":                {Space: ColorRGB, R: 0.941, G: 0.902, B: 0.549, A: 1},
		"lavender":             {Space: ColorRGB, R: 0.902, G: 0.902, B: 0.98, A: 1},
		"lavenderblush":        {Space: ColorRGB, R: 1, G: 0.941, B: 0.961, A: 1},
		"lawngreen":            {Space: ColorRGB, R: 0.486, G: 0.988, B: 0, A: 1},
		"lemonchiffon":         {Space: ColorRGB, R: 1, G: 0.98, B: 0.804, A: 1},
		"lightblue":            {Space: ColorRGB, R: 0.678, G: 0.847, B: 0.902, A: 1},
		"lightcoral":           {Space: ColorRGB, R: 0.941, G: 0.502, B: 0.502, A: 1},
		"lightcyan":            {Space: ColorRGB, R: 0.878, G: 1, B: 1, A: 1},
		"lightgoldenrodyellow": {Space: ColorRGB, R: 0.98, G: 0.98, B: 0.824, A: 1},
		"lightgray":            {Space: ColorRGB, R: 0.827, G: 0.827, B: 0.827, A: 1},
		"lightgreen":           {Space: ColorRGB, R: 0.565, G: 0.933, B: 0.565, A: 1},
		"lightgrey":            {Space: ColorRGB, R: 0.827, G: 0.827, B: 0.827, A: 1},
		"lightpink":            {Space: ColorRGB, R: 1, G: 0.714, B: 0.757, A: 1},
		"lightsalmon":          {Space: ColorRGB, R: 1, G: 0.627, B: 0.478, A: 1},
		"lightseagreen":        {Space: ColorRGB, R: 0.125, G: 0.698, B: 0.667, A: 1},
		"lightskyblue":         {Space: ColorRGB, R: 0.529, G: 0.808, B: 0.98, A: 1},
		"lightslategray":       {Space: ColorRGB, R: 0.467, G: 0.533, B: 0.6, A: 1},
		"lightslategrey":       {Space: ColorRGB, R: 0.467, G: 0.533, B: 0.6, A: 1},
		"lightsteelblue":       {Space: ColorRGB, R: 0.69, G: 0.769, B: 0.871, A: 1},
		"lightyellow":          {Space: ColorRGB, R: 1, G: 1, B: 0.878, A: 1},
		"lime":                 {Space: ColorRGB, R: 0, G: 1, B: 0, A: 1},
		"limegreen":            {Space: ColorRGB, R: 0.196, G: 0.804, B: 0.196, A: 1},
		"linen":                {Space: ColorRGB, R: 0.98, G: 0.941, B: 0.902, A: 1},
		"maroon":               {Space: ColorRGB, R: 0.502, G: 0, B: 0, A: 1},
		"mediumaquamarine":     {Space: ColorRGB, R: 0.4, G: 0.804, B: 0.667, A: 1},
		"mediumblue":           {Space: ColorRGB, R: 0, G: 0, B: 0.804, A: 1},
		"mediumorchid":         {Space: ColorRGB, R: 0.729, G: 0.333, B: 0.827, A: 1},
		"mediumpurple":         {Space: ColorRGB, R: 0.576, G: 0.439, B: 0.859, A: 1},
		"mediumseagreen":       {Space: ColorRGB, R: 0.235, G: 0.702, B: 0.443, A: 1},
		"mediumslateblue":      {Space: ColorRGB, R: 0.482, G: 0.408, B: 0.933, A: 1},
		"mediumspringgreen":    {Space: ColorRGB, R: 0, G: 0.98, B: 0.604, A: 1},
		"mediumturquoise":      {Space: ColorRGB, R: 0.282, G: 0.82, B: 0.8, A: 1},
		"mediumvioletred":      {Space: ColorRGB, R: 0.78, G: 0.082, B: 0.522, A: 1},
		"midnightblue":         {Space: ColorRGB, R: 0.098, G: 0.098, B: 0.439, A: 1},
		"mintcream":            {Space: ColorRGB, R: 0.961, G: 1, B: 0.98, A: 1},
		"mistyrose":            {Space: ColorRGB, R: 1, G: 0.894, B: 0.882, A: 1},
		"moccasin":             {Space: ColorRGB, R: 1, G: 0.894, B: 0.71, A: 1},
		"navajowhite":          {Space: ColorRGB, R: 1, G: 0.871, B: 0.678, A: 1},
		"navy":                 {Space: ColorRGB, R: 0, G: 0, B: 0.502, A: 1},
		"oldlace":              {Space: ColorRGB, R: 0.992, G: 0.961, B: 0.902, A: 1},
		"olive":                {Space: ColorRGB, R: 0.502, G: 0.502, B: 0, A: 1},
		"olivedrab":            {Space: ColorRGB, R: 0.42, G: 0.557, B: 0.137, A: 1},
		"orange":               {Space: ColorRGB, R: 1, G: 0.647, B: 0, A: 1},
		"orangered":            {Space: ColorRGB, R: 1, G: 0.271, B: 0, A: 1},
		"orchid":               {Space: ColorRGB, R: 0.855, G: 0.439, B: 0.839, A: 1},
		"palegoldenrod":        {Space: ColorRGB, R: 0.933, G: 0.91, B: 0.667, A: 1},
		"palegreen":            {Space: ColorRGB, R: 0.596, G: 0.984, B: 0.596, A: 1},
		"paleturquoise":        {Space: ColorRGB, R: 0.686, G: 0.933, B: 0.933, A: 1},
		"palevioletred":        {Space: ColorRGB, R: 0.859, G: 0.439, B: 0.576, A: 1},
		"papayawhip":           {Space: ColorRGB, R: 1, G: 0.937, B: 0.835, A: 1},
		"peachpuff":            {Space: ColorRGB, R: 1, G: 0.855, B: 0.725, A: 1},
		"peru":                 {Space: ColorRGB, R: 0.804, G: 0.522, B: 0.247, A: 1},
		"pink":                 {Space: ColorRGB, R: 1, G: 0.753, B: 0.796, A: 1},
		"plum":                 {Space: ColorRGB, R: 0.867, G: 0.627, B: 0.867, A: 1},
		"powderblue":           {Space: ColorRGB, R: 0.69, G: 0.878, B: 0.902, A: 1},
		"purple":               {Space: ColorRGB, R: 0.502, G: 0, B: 0.502, A: 1},
		"rebeccapurple":        {Space: ColorRGB, R: 0.4, G: 0.2, B: 0.6, A: 1},
		"red":                  {Space: ColorRGB, R: 1, G: 0, B: 0, A: 1},
		"rosybrown":            {Space: ColorRGB, R: 0.737, G: 0.561, B: 0.561, A: 1},
		"royalblue":            {Space: ColorRGB, R: 0.255, G: 0.412, B: 0.882, A: 1},
		"saddlebrown":          {Space: ColorRGB, R: 0.545, G: 0.271, B: 0.075, A: 1},
		"salmon":               {Space: ColorRGB, R: 0.98, G: 0.502, B: 0.447, A: 1},
		"sandybrown":           {Space: ColorRGB, R: 0.957, G: 0.643, B: 0.376, A: 1},
		"seagreen":             {Space: ColorRGB, R: 0.18, G: 0.545, B: 0.341, A: 1},
		"seashell":             {Space: ColorRGB, R: 1, G: 0.961, B: 0.933, A: 1},
		"sienna":               {Space: ColorRGB, R: 0.627, G: 0.322, B: 0.176, A: 1},
		"silver":               {Space: ColorRGB, R: 0.753, G: 0.753, B: 0.753, A: 1},
		"skyblue":              {Space: ColorRGB, R: 0.529, G: 0.808, B: 0.922, A: 1},
		"slateblue":            {Space: ColorRGB, R: 0.416, G: 0.353, B: 0.804, A: 1},
		"slategray":            {Space: ColorRGB, R: 0.439, G: 0.502, B: 0.565, A: 1},
		"slategrey":            {Space: ColorRGB, R: 0.439, G: 0.502, B: 0.565, A: 1},
		"snow":                 {Space: ColorRGB, R: 1, G: 0.98, B: 0.98, A: 1},
		"springgreen":          {Space: ColorRGB, R: 0, G: 1, B: 0.498, A: 1},
		"steelblue":            {Space: ColorRGB, R: 0.275, G: 0.51, B: 0.706, A: 1},
		"tan":                  {Space: ColorRGB, R: 0.824, G: 0.706, B: 0.549, A: 1},
		"teal":                 {Space: ColorRGB, R: 0, G: 0.502, B: 0.502, A: 1},
		"thistle":              {Space: ColorRGB, R: 0.847, G: 0.749, B: 0.847, A: 1},
		"tomato":               {Space: ColorRGB, R: 1, G: 0.388, B: 0.278, A: 1},
		"turquoise":            {Space: ColorRGB, R: 0.251, G: 0.878, B: 0.816, A: 1},
		"violet":               {Space: ColorRGB, R: 0.933, G: 0.51, B: 0.933, A: 1},
		"wheat":                {Space: ColorRGB, R: 0.961, G: 0.871, B: 0.702, A: 1},
		"white":                {Space: ColorGray, R: 0, G: 1, B: 0, A: 1},
		"whitesmoke":           {Space: ColorRGB, R: 0.961, G: 0.961, B: 0.961, A: 1},
		"yellow":               {Space: ColorRGB, R: 1, G: 1, B: 0, A: 1},
		"yellowgreen":          {Space: ColorRGB, R: 0.604, G: 0.804, B: 0.196, A: 1},
	}
)

// ColorSpace represents the color space of a defined color.
type ColorSpace int

const (
	// ColorRGB represents a color in RGB color space
	ColorRGB ColorSpace = iota
	// ColorCMYK represents a color in CMYK color space
	ColorCMYK
	// ColorGray represents a gray scale color.
	ColorGray
)

// DefineColor associates a color with a name for later use.
func (d *Document) DefineColor(name string, col *Color) {
	d.colors[name] = col
}

// GetColor returns a color. The string can be a predefined color name or an
// HTML / CSS color definition such as #FAF or rgb(0.5.,0.5,0.5).
func (d *Document) GetColor(s string) *Color {
	if col, ok := d.colors[s]; ok {
		return col
	}
	col := &Color{}
	if strings.HasPrefix(s, "#") {
		col.Space = ColorRGB
		var r, g, b, alpha int
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
		col.A = math.Round(100.0*float64(alpha)/float64(255)) / 100.0
		return col
	}
	return nil
}

func (col *Color) getPDFColorSuffix(fg bool) string {
	if fg {
		return []string{"rg", "k", "g"}[col.Space]
	}
	return []string{"RG", "K", "G"}[col.Space]
}

func (col *Color) getPDFColorValues() string {
	switch col.Space {
	case ColorRGB:
		return fmt.Sprintf("%s %s %s ", strconv.FormatFloat(col.R, 'f', -1, 64), strconv.FormatFloat(col.G, 'f', -1, 64), strconv.FormatFloat(col.B, 'f', -1, 64))
	case ColorCMYK:
		return fmt.Sprintf("%s %s %s %s ", strconv.FormatFloat(col.C, 'f', -1, 64), strconv.FormatFloat(col.M, 'f', -1, 64), strconv.FormatFloat(col.Y, 'f', -1, 64), strconv.FormatFloat(col.K, 'f', -1, 64))
	case ColorGray:
		return fmt.Sprintf("%s ", strconv.FormatFloat(col.G, 'f', -1, 64))
	default:
		bag.Logger.DPanic("PDFStringFG: unknown color space.")
		return ""
	}
}

// PDFStringFG returns the PDF instructions to swith to the color for foreground colors.
func (col *Color) PDFStringFG() string {
	return col.getPDFColorValues() + col.getPDFColorSuffix(true)
}

// PDFStringBG returns the PDF instructions to swith to the color for background colors.
func (col *Color) PDFStringBG() string {
	return col.getPDFColorValues() + col.getPDFColorSuffix(false)
}
