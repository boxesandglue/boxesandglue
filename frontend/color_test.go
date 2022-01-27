package frontend

import (
	"testing"
)

func TestSimple(t *testing.T) {
	f := Frontend{}
	f.DefineColor("mycolor", &Color{Space: ColorCMYK, C: 1, M: 0, Y: 0, K: 1})
	testdata := []struct {
		colorname  string
		result     string
		foreground bool
	}{
		{"red", "1 0 0 rg", true},
		{"mycolor", "1 0 0 1 K", false},
	}
	for _, tc := range testdata {
		col := f.GetColor(tc.colorname)
		if tc.foreground {
			if got, want := col.PDFStringFG(), tc.result; got != want {
				t.Errorf("col.PDFStringFG() = %s, want %s", got, want)
			}
		} else {
			if got, want := col.PDFStringBG(), tc.result; got != want {
				t.Errorf("col.PDFStringBG() = %s, want %s", got, want)
			}
		}
	}
}

func TestParseColors(t *testing.T) {
	f := Frontend{}

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

func TestHTMLColors(t *testing.T) {
	f := Frontend{}

	testdata := []struct {
		colorvalue string
		result     string
	}{
		{"#fff080", "1 0.94 0.5 "},
		{"#000000", "0 0 0 "},
		{"#000", "0 0 0 "},
	}
	for _, tc := range testdata {
		col := f.GetColor(tc.colorvalue)
		if got, want := col.getPDFColorValues(), tc.result; got != want {
			t.Errorf("col.getPDFColorValues = %q, want %q", got, want)
		}
	}
}
