package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/color"
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
