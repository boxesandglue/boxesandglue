package csshtml

import (
	"testing"
)

func TestParseBorder(t *testing.T) {
	testCases := []struct {
		input string
		width string
		style string
		color string
	}{
		{"solid black", "1pt", "solid", "black"},
		{"1pt solid black", "1pt", "solid", "black"},
		{"solid black 1pt", "1pt", "solid", "black"},
		{"solid 00pt black", "00pt", "solid", "black"},
		{"solid 1pt black", "1pt", "solid", "black"},
		{"black solid 1pt", "1pt", "solid", "black"},
		{"1pt green", "1pt", "none", "green"},
		{"green", "1pt", "none", "green"},
		{"0.5rem outset pink", "0.5rem", "outset", "pink"},
		{"thin outset pink", "0.5pt", "outset", "pink"},
		{"medium", "1pt", "none", "currentcolor"},
		{"thick dashed", "2pt", "dashed", "currentcolor"},
		{"dotted", "1pt", "dotted", "currentcolor"},
		{"dashed", "1pt", "dashed", "currentcolor"},
		{"dashed rgba(170, 50, 220, .6)", "1pt", "dashed", "rgba(170, 50, 220, .6)"},
		{"rgba(170, 50, 220, .6)", "1pt", "none", "rgba(170, 50, 220, .6)"},
	}
	for _, tC := range testCases {
		t.Run(tC.input, func(t *testing.T) {
			wd, sty, col := parseBorderAttribute(tC.input)
			if wd != tC.width || sty != tC.style || col != tC.color {
				t.Errorf(`parseBorderAttribute(%s) got "%s %s %s" want "%s %s %s"`, tC.input, wd, sty, col, tC.width, tC.style, tC.color)
			}
		})
	}
}
