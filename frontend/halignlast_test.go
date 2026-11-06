package frontend

import (
	"bytes"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// gapsOf returns the empty room left and right of the glyphs of a line.
func gapsOf(line *node.HList) (left, right bag.ScaledPoint) {
	var x, end bag.ScaledPoint
	seen := false
	for e := line.List; e != nil; e = e.Next() {
		switch v := e.(type) {
		case *node.Glyph:
			if !seen {
				left, seen = x, true
			}
			x += v.Width
			end = x
		case *node.Glue:
			x += v.Width
		case *node.Kern:
			x += v.Kern
		}
	}
	return left, line.Width - end
}

func TestHAlignLast(t *testing.T) {
	var buf bytes.Buffer
	fe, err := NewForWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("text")
	if err = ff.AddMember(&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"}, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	format := func(text string, last HorizontalAlignment, width string) []*node.HList {
		t.Helper()
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Settings[SettingHAlign] = HAlignJustified
		te.Settings[SettingHAlignLast] = last
		te.Items = append(te.Items, text)
		vl, _, err := fe.FormatParagraph(te, bag.MustSP(width))
		if err != nil {
			t.Fatal(err)
		}
		var lines []*node.HList
		for e := vl.List; e != nil; e = e.Next() {
			if hl, ok := e.(*node.HList); ok {
				lines = append(lines, hl)
			}
		}
		return lines
	}
	const text, width = "one two three four five six seven eight nine ten eleven", "100pt"
	wantLines := len(format(text, HAlignDefault, width))
	if wantLines < 2 {
		t.Fatalf("%d lines, want several", wantLines)
	}
	// Stretching rounds every glue down to a scaled point.
	near := func(a, b bag.ScaledPoint) bool { return max(a-b, b-a) <= bag.MustSP("0.01pt") }
	for _, tc := range []struct {
		name string
		last HorizontalAlignment
		ok   func(left, right bag.ScaledPoint) bool
	}{
		{"default", HAlignDefault, func(l, r bag.ScaledPoint) bool { return near(l, 0) && r > 0 }},
		{"left", HAlignLeft, func(l, r bag.ScaledPoint) bool { return near(l, 0) && r > 0 }},
		{"right", HAlignRight, func(l, r bag.ScaledPoint) bool { return l > 0 && near(r, 0) }},
		{"center", HAlignCenter, func(l, r bag.ScaledPoint) bool { return l > 0 && near(l, r) }},
		{"justified", HAlignJustified, func(l, r bag.ScaledPoint) bool { return near(l, 0) && near(r, 0) }},
		{"start", HAlignStart, func(l, r bag.ScaledPoint) bool { return near(l, 0) && r > 0 }},
		{"end", HAlignEnd, func(l, r bag.ScaledPoint) bool { return l > 0 && near(r, 0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := format(text, tc.last, width)
			if len(lines) != wantLines {
				t.Fatalf("%d lines, want %d: the breaks must stay", len(lines), wantLines)
			}
			if left, right := gapsOf(lines[len(lines)-1]); !tc.ok(left, right) {
				t.Errorf("last line: %v left, %v right", left, right)
			}
			if left, right := gapsOf(lines[0]); !near(left, 0) || !near(right, 0) {
				t.Errorf("first line: %v left, %v right", left, right)
			}
		})
	}
	// Nothing to stretch: the end-of-paragraph fill keeps the line at the start.
	if left, right := gapsOf(format("word", HAlignJustified, width)[0]); !near(left, 0) || right == 0 {
		t.Errorf("one word: %v left, %v right", left, right)
	}
}
