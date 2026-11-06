package frontend

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/document"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

func TestSimple(t *testing.T) {
	f, _ := initDocument(io.Discard)
	f.DefineColor("mycolor", &color.Color{Space: color.ColorCMYK, C: 1, M: 0, Y: 0, K: 1})
	testdata := []struct {
		colorname  string
		result     string
		foreground bool
	}{
		{"red", "1 0 0 RG", true},
		{"mycolor", "1 0 0 1 k", false},
	}
	for _, tc := range testdata {
		col := f.GetColor(tc.colorname)
		if tc.foreground {
			if got, want := col.PDFStringStroking(), tc.result; got != want {
				t.Errorf("col.PDFStringFG() = %s, want %s", got, want)
			}
		} else {
			if got, want := col.PDFStringNonStroking(), tc.result; got != want {
				t.Errorf("col.PDFStringBG() = %s, want %s", got, want)
			}
		}
	}
}

func TestParseColors(t *testing.T) {
	f := Document{}

	testdata := []struct {
		colorvalue string
		expected   string
	}{
		{"rgba(0,255,0)", "rgba(0,255,0,0)"},
		{"rgb(0,255,0)", "rgba(0,255,0,0)"},
		{"rgb(0,255,0,1.0)", "rgba(0,255,0,1)"},
		{"rgb(0,255,0,1)", "rgba(0,255,0,1)"},
	}
	for _, tc := range testdata {
		col := f.GetColor(tc.colorvalue)
		if got := col.String(); got != tc.expected {
			t.Errorf("col.String() = %q, want %q", got, tc.expected)
		}
	}
}

func TestParseCMYKColors(t *testing.T) {
	f := Document{}

	testdata := []struct {
		colorvalue string
		// PDFStringNonStroking output: "C M Y K k"
		expected string
	}{
		// percentage form — what the legacy cmyk()-function uses
		{"cmyk(100%, 0%, 0%, 0%)", "1 0 0 0 k"},
		{"cmyk(0%, 9%, 7%, 30%)", "0 0.09 0.07 0.3 k"},
		// what stringValue() produces from the CSS scanner round-trip
		{"cmyk( 100% , 36% , 0% , 0% )", "1 0.36 0 0 k"},
		// device-cmyk(): CSS Color 5 form, whitespace + 0..1 numbers
		{"device-cmyk(1 0 0 0)", "1 0 0 0 k"},
		{"device-cmyk(1 0.35 0 0)", "1 0.35 0 0 k"},
		{"device-cmyk(0 0.09 0.07 0.3)", "0 0.09 0.07 0.3 k"},
		// device-cmyk() also accepts percentages
		{"device-cmyk(100% 0% 0% 0%)", "1 0 0 0 k"},
	}
	for _, tc := range testdata {
		col := f.GetColor(tc.colorvalue)
		if col == nil {
			t.Errorf("GetColor(%q) = nil, want CMYK color", tc.colorvalue)
			continue
		}
		if got := col.PDFStringNonStroking(); got != tc.expected {
			t.Errorf("GetColor(%q).PDFStringNonStroking() = %q, want %q", tc.colorvalue, got, tc.expected)
		}
	}
}

func TestParseCMYKMalformed(t *testing.T) {
	f := Document{}
	bad := []string{
		"cmyk()",
		"cmyk(1, 2)",
		"cmyk(a, b, c, d)",
		"device-cmyk(1 0 0)",
	}
	for _, s := range bad {
		if col := f.GetColor(s); col != nil {
			t.Errorf("GetColor(%q) = %+v, want nil", s, col)
		}
	}
}

// A spot color defined by the user must end up as a Separation color space in
// the PDF. Only the predefined Pantone entries used to be registered, a
// DefineColor'd spot color wrote "/CS0 cs" without any matching resource.
func TestDefineSpotColor(t *testing.T) {
	f, _ := initDocument(io.Discard)
	spot := &color.Color{Space: color.ColorSpotcolor, Basecolor: "PANTONE 300 C", C: 1, M: 0.44}
	f.DefineColor("brand", spot)
	if spot.SpotcolorID == 0 {
		t.Fatal("spot color not registered with the document")
	}
	if !f.usedSpotcolors[spot] {
		t.Error("spot color missing from usedSpotcolors")
	}
	if got, want := f.GetColor("brand").PDFStringNonStroking(), "/CS1 cs 1 scn "; got != want {
		t.Errorf("PDFStringNonStroking() = %q, want %q", got, want)
	}
	// defining the same color under a second name must not register twice
	f.DefineColor("alias", spot)
	if got := len(f.usedSpotcolors); got != 1 {
		t.Errorf("len(usedSpotcolors) = %d, want 1", got)
	}
	// the ink name falls back to the color name
	plain := &color.Color{Space: color.ColorSpotcolor}
	f.DefineColor("gold", plain)
	if plain.Basecolor != "gold" {
		t.Errorf("Basecolor = %q, want gold", plain.Basecolor)
	}
}

// captureWarnings collects what the logger reports while f runs.
func captureWarnings(t *testing.T, f func()) string {
	t.Helper()
	old := bag.Logger
	defer bag.SetLogger(old)
	var buf bytes.Buffer
	bag.SetLogger(slog.New(slog.NewTextHandler(&buf, nil)))
	f()
	return buf.String()
}

// Overprint only has a meaning on the plates: white does nothing at all and
// RGB and gray knock out as before. Both are mistakes worth a warning.
func TestDefineColorOverprintWarnings(t *testing.T) {
	testdata := []struct {
		name string
		col  color.Color
		want string
	}{
		{"black", color.Color{Space: color.ColorCMYK, K: 1, Overprint: true}, ""},
		{"spot", color.Color{Space: color.ColorSpotcolor, Basecolor: "x", Overprint: true}, ""},
		{"knockout", color.Color{Space: color.ColorRGB}, ""},
		{"white", color.Color{Space: color.ColorCMYK, Overprint: true}, "leaves no mark"},
		{"rgb", color.Color{Space: color.ColorRGB, R: 1, Overprint: true}, "knocks out"},
		{"gray", color.Color{Space: color.ColorGray, Overprint: true}, "knocks out"},
	}
	for _, tc := range testdata {
		f, _ := initDocument(io.Discard)
		got := captureWarnings(t, func() { f.DefineColor(tc.name, &tc.col) })
		if tc.want == "" && got != "" {
			t.Errorf("%s: unexpected warning %q", tc.name, got)
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: warning %q does not contain %q", tc.name, got, tc.want)
		}
		if f.Doc.Overprint != tc.col.Overprint {
			t.Errorf("%s: Doc.Overprint = %v", tc.name, f.Doc.Overprint)
		}
	}
}

// The graphics state outlives the color operators, so a text run has to switch
// overprint off at its end. A color without overprint must write what it
// always wrote.
func TestColorEndOfTextRun(t *testing.T) {
	f, err := initDocument(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := f.NewFontFamily("test")
	if err := ff.AddMember(
		&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"},
		FontWeight400, FontStyleNormal,
	); err != nil {
		t.Fatal(err)
	}
	f.DefineColor("ink", &color.Color{Space: color.ColorCMYK, K: 1, Overprint: true})
	f.DefineColor("plain", &color.Color{Space: color.ColorCMYK, K: 1})
	on := "/" + string(pdf.OverprintOn.ResourceName()) + " gs 0 0 0 1 k "
	off := "/" + string(pdf.OverprintOff.ResourceName()) + " gs " + document.ColorReset
	testdata := []struct{ name, first, last string }{
		{"ink", on, off},
		{"plain", "0 0 0 1 k ", document.ColorReset},
	}
	for _, tc := range testdata {
		head, err := f.BuildNodelistFromString(TypesettingSettings{
			SettingFontFamily: ff,
			SettingSize:       bag.MustSP("10pt"),
			SettingColor:      tc.name,
		}, "text")
		if err != nil {
			t.Fatal(err)
		}
		var code []string
		for n := head; n != nil; n = n.Next() {
			if s, ok := n.(*node.StartStop); ok && s.ShipoutCallback != nil {
				code = append(code, s.ShipoutCallback(s))
			}
		}
		if len(code) != 2 || code[0] != tc.first || code[1] != tc.last {
			t.Errorf("%s: got %q, want %q and %q", tc.name, code, tc.first, tc.last)
		}
	}
}

// Every page gets the graphics states, the operators in the page stream
// refer to them by name. Documents without overprint stay as they were.
func TestOverprintResources(t *testing.T) {
	for _, overprint := range []bool{false, true} {
		var buf bytes.Buffer
		f, err := initDocument(&buf)
		if err != nil {
			t.Fatal(err)
		}
		f.Doc.CompressLevel = 0
		f.DefineColor("ink", &color.Color{Space: color.ColorCMYK, K: 1, Overprint: overprint})
		r := node.NewRule()
		r.Pre = f.GetColor("ink").PDFStringNonStroking()
		r.Width, r.Height = bag.MustSP("1pt"), bag.MustSP("1pt")
		p := f.Doc.NewPage()
		p.OutputAt(0, bag.MustSP("10pt"), node.Vpack(r))
		p.Shipout()
		if err := f.Doc.Finish(); err != nil {
			t.Fatal(err)
		}
		if got := bytes.Contains(buf.Bytes(), []byte("/ExtGState")); got != overprint {
			t.Errorf("overprint %v: /ExtGState in the PDF: %v", overprint, got)
		}
		if got := bytes.Contains(buf.Bytes(), []byte("/OPM 1")); got != overprint {
			t.Errorf("overprint %v: /OPM 1 in the PDF: %v", overprint, got)
		}
	}
}
