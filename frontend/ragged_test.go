package frontend

import (
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

const raggedText = "lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua"

// raggedParagraph sets raggedText in a measure 1pt narrower than its first
// eight words and returns the paragraph.
func raggedParagraph(t *testing.T, align HorizontalAlignment) *node.VList {
	t.Helper()
	fe, ff := lineModelDocument(t)
	text := func(s string) *Text {
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("12pt")
		te.Settings[SettingFontExpansion] = 0.05
		te.Items = append(te.Items, s)
		return te
	}
	head, _, err := fe.Mknodes(text(strings.Join(strings.Fields(raggedText)[:8], " ")))
	if err != nil {
		t.Fatal(err)
	}
	natural := node.Hpack(head).Width
	te := text(raggedText)
	te.Settings[SettingHAlign] = align
	vl, _, err := fe.FormatParagraph(te, natural-bag.MustSP("1pt"))
	if err != nil {
		t.Fatal(err)
	}
	return vl
}

// firstLine is the paragraph's first line.
func firstLine(t *testing.T, vl *node.VList) *node.HList {
	t.Helper()
	for n := vl.List; n != nil; n = n.Next() {
		if hl, ok := n.(*node.HList); ok {
			return hl
		}
	}
	t.Fatal("no line")
	return nil
}

// wordsOn counts the words on a line, the runs of glyphs between its glue.
func wordsOn(hl *node.HList) int {
	words, inWord := 0, false
	for n := hl.List; n != nil; n = n.Next() {
		switch n.(type) {
		case *node.Glyph:
			if !inWord {
				words++
			}
			inWord = true
		case *node.Glue:
			inWord = false
		}
	}
	return words
}

// A ragged line is set at its natural width (plain TeX's \raggedright, CSS
// Text 3 §6.1): its spaces do not shrink and its glyphs are not condensed to
// keep a word the measure has no room for.
func TestARaggedLineDoesNotShrink(t *testing.T) {
	for _, tc := range []struct {
		name  string
		align HorizontalAlignment
	}{
		{"left", HAlignLeft},
		{"right", HAlignRight},
		{"center", HAlignCenter},
	} {
		if got := wordsOn(firstLine(t, raggedParagraph(t, tc.align))); got != 7 {
			t.Errorf("%s: %d words on the first line, want 7", tc.name, got)
		}
	}
}

// A justified line still shrinks its spaces and condenses its glyphs to keep
// the eighth word.
func TestAJustifiedLineStillShrinks(t *testing.T) {
	hl := firstLine(t, raggedParagraph(t, HAlignJustified))
	if got := wordsOn(hl); got != 8 {
		t.Errorf("%d words on the first line, want 8", got)
	}
	if hl.GlueSet >= 0 {
		t.Errorf("glue set %v, want the line shrunk", hl.GlueSet)
	}
}
