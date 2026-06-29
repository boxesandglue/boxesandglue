package node

import (
	"fmt"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// TestRaggedRightFillsLines reproduces the right-aligned narrow-column case
// (xts mailmerge company block). Words separated by fixed inter-word glue,
// LineStartGlue carrying fil stretch (text-align:right). HSize is wide enough
// to fit several words per line. The breaker must NOT break after every word.
func TestRaggedRightFillsLines(t *testing.T) {
	const charWidth = bag.ScaledPoint(6 * bag.Factor) // ~6pt per glyph
	const spaceWidth = bag.ScaledPoint(3 * bag.Factor)

	// Three short words: "Print" "Company" "Office"  (5+7+6 glyphs)
	words := []string{"Print", "Company", "Office"}

	var head, cur Node
	for i, w := range words {
		if i > 0 {
			sp := NewGlue()
			sp.Width = spaceWidth // fixed inter-word glue (no stretch) for ragged-right
			head = InsertAfter(head, cur, sp)
			cur = sp
		}
		head, cur = glyphRun(head, cur, w, charWidth)
	}
	head, _ = AppendLineEndAfter(head, cur)

	settings := NewLinebreakSettings()
	// text-align:right -> leftskip (LineStartGlue) carries fil stretch.
	ls := NewGlue()
	ls.Stretch = 1 * bag.Factor
	ls.StretchOrder = StretchFil
	settings.LineStartGlue = ls
	settings.LineEndGlue = NewGlue() // zero
	// HSize: each word ~ 5*6=30 / 7*6=42 / 6*6=36 pt, spaces 3pt.
	// "Print Company Office" = 30+3+42+3+36 = 114pt. Give 130pt -> fits on ONE line.
	settings.HSize = 130 * bag.Factor
	settings.LineHeight = 12 * bag.Factor

	vlist, _ := Linebreak(head, settings)
	got := countHLists(vlist)
	fmt.Printf("RAGGED: %d lines for %q at HSize=%s\n", got, words, settings.HSize)
	if got != 1 {
		t.Errorf("ragged-right broke %q into %d lines, want 1 (all words fit in HSize)", words, got)
	}
}

// TestHardBreakRightAligned: HardBreak must force a break even when the
// paragraph is right-aligned (LineStartGlue carries fil stretch). Mimics the
// mailmerge company block "Office<br/>61556".
func TestHardBreakRightAligned(t *testing.T) {
	const charWidth = bag.ScaledPoint(6 * bag.Factor)

	var head, cur Node
	head, cur = glyphRun(head, cur, "Office", charWidth)
	hb := NewHardBreak()
	head = InsertAfter(head, cur, hb)
	cur = hb
	head, cur = glyphRun(head, cur, "61556", charWidth)
	head, _ = AppendLineEndAfter(head, cur)

	settings := NewLinebreakSettings()
	ls := NewGlue()
	ls.Stretch = 1 * bag.Factor
	ls.StretchOrder = StretchFil
	settings.LineStartGlue = ls
	settings.LineEndGlue = NewGlue()
	settings.HSize = 130 * bag.Factor // both words easily fit on one line
	settings.LineHeight = 12 * bag.Factor

	vlist, _ := Linebreak(head, settings)
	got := countHLists(vlist)
	fmt.Printf("HB-RIGHT: %d lines (want 2)\n", got)
	if got != 2 {
		t.Errorf("HardBreak under right-align yielded %d lines, want 2", got)
	}
}

// TestRaggedRightMultiLine: a long right-aligned paragraph that needs several
// lines must FILL each line (pack as many words as fit), not break after every
// word. spaceStretch>0 mimics the real inter-word glue from the shaper.
func TestRaggedRightMultiLine(t *testing.T) {
	const charWidth = bag.ScaledPoint(4 * bag.Factor)
	const spaceWidth = bag.ScaledPoint(2 * bag.Factor)
	const spaceStretch = bag.ScaledPoint(1 * bag.Factor)

	// 15 words of 4 glyphs each = 16pt + 2pt space. ~7 fit in 130pt.
	var head, cur Node
	for i := range 15 {
		if i > 0 {
			sp := NewGlue()
			sp.Width = spaceWidth
			sp.Stretch = spaceStretch
			sp.Shrink = spaceStretch / 2
			head = InsertAfter(head, cur, sp)
			cur = sp
		}
		head, cur = glyphRun(head, cur, "wxyz", charWidth)
	}
	head, _ = AppendLineEndAfter(head, cur)

	settings := NewLinebreakSettings()
	ls := NewGlue()
	ls.Stretch = 1 * bag.Factor
	ls.StretchOrder = StretchFil
	settings.LineStartGlue = ls
	settings.LineEndGlue = NewGlue()
	settings.HSize = 130 * bag.Factor
	settings.LineHeight = 12 * bag.Factor

	vlist, _ := Linebreak(head, settings)
	got := countHLists(vlist)
	// 15 words * 18pt = 270pt of content; 130pt lines -> ~3 lines if filled.
	fmt.Printf("RAGGED-MULTI: %d lines (filled would be ~3; one-per-word would be 15)\n", got)
	if got > 5 {
		t.Errorf("ragged-right multi-line broke into %d lines, expected lines to be filled (~3)", got)
	}
}

// TestRaggedRightWithMonsterWord: a right-aligned paragraph with several short
// words followed by one unbreakable word wider than HSize (mimics the mailmerge
// company block where dropped <br/> glued "98104206-711-...jbiddy@printcompany.com"
// into one ~222pt run). The SHORT words before it must still fill normally; the
// monster word overflowing must not make every preceding word break onto its
// own line.
func TestRaggedRightWithMonsterWord(t *testing.T) {
	const charWidth = bag.ScaledPoint(4 * bag.Factor)
	const spaceWidth = bag.ScaledPoint(2 * bag.Factor)
	const spaceStretch = bag.ScaledPoint(1 * bag.Factor)

	mkSpace := func() *Glue {
		g := NewGlue()
		g.Width = spaceWidth
		g.Stretch = spaceStretch
		g.Shrink = spaceStretch / 2
		return g
	}

	var head, cur Node
	// 6 short words "abcd"
	for i := range 6 {
		if i > 0 {
			sp := mkSpace()
			head = InsertAfter(head, cur, sp)
			cur = sp
		}
		head, cur = glyphRun(head, cur, "abcd", charWidth)
	}
	// space, then the monster word: 60 glyphs, no internal glue => ~240pt
	sp := mkSpace()
	head = InsertAfter(head, cur, sp)
	cur = sp
	monster := strings.Repeat("x", 60)
	head, cur = glyphRun(head, cur, monster, charWidth)
	head, _ = AppendLineEndAfter(head, cur)

	settings := NewLinebreakSettings()
	ls := NewGlue()
	ls.Stretch = 1 * bag.Factor
	ls.StretchOrder = StretchFil
	settings.LineStartGlue = ls
	settings.LineEndGlue = NewGlue()
	settings.HSize = 130 * bag.Factor
	settings.LineHeight = 12 * bag.Factor

	vlist, _ := Linebreak(head, settings)
	got := countHLists(vlist)
	// 6 short words (16pt each + 2pt) = ~106pt -> fit on ONE line; monster on
	// its own (overfull) line. Expect ~2 lines. One-per-word would be 7.
	fmt.Printf("RAGGED-MONSTER: %d lines (want ~2; one-per-word bug => 7)\n", got)
	if got > 3 {
		t.Errorf("monster-word case broke into %d lines, want ~2 (short words should fill)", got)
	}
}

// discWord appends glyphs for s with a hyphenation Disc inserted after the
// rune index splitAfter (0-based). Mimics a hyphenatable word.
func discWord(head, cur Node, s string, width bag.ScaledPoint, splitAfter int) (Node, Node) {
	for i, r := range s {
		g := NewGlyph()
		g.Width = width
		g.Components = string(r)
		head = InsertAfter(head, cur, g)
		cur = g
		if i == splitAfter {
			d := NewDisc()
			hyphen := NewGlyph()
			hyphen.Width = width
			hyphen.Components = "-"
			d.Pre = hyphen
			head = InsertAfter(head, cur, d)
			cur = d
		}
	}
	return head, cur
}

// TestRaggedRightWithHyphenation adds hyphenation Disc nodes inside the words.
// This is the real mailmerge company-block configuration.
func TestRaggedRightWithHyphenation(t *testing.T) {
	const charWidth = bag.ScaledPoint(6 * bag.Factor)
	const spaceWidth = bag.ScaledPoint(3 * bag.Factor)

	var head, cur Node
	head, cur = discWord(head, cur, "Print", charWidth, 2)
	sp := NewGlue()
	sp.Width = spaceWidth
	head = InsertAfter(head, cur, sp)
	cur = sp
	head, cur = discWord(head, cur, "Company", charWidth, 2) // Com-pany
	sp2 := NewGlue()
	sp2.Width = spaceWidth
	head = InsertAfter(head, cur, sp2)
	cur = sp2
	head, cur = discWord(head, cur, "Office", charWidth, 2) // Of-fice
	head, _ = AppendLineEndAfter(head, cur)

	settings := NewLinebreakSettings()
	ls := NewGlue()
	ls.Stretch = 1 * bag.Factor
	ls.StretchOrder = StretchFil
	settings.LineStartGlue = ls
	settings.LineEndGlue = NewGlue()
	settings.HSize = 130 * bag.Factor
	settings.LineHeight = 12 * bag.Factor

	vlist, _ := Linebreak(head, settings)
	got := countHLists(vlist)
	fmt.Printf("RAGGED+HYPH: %d lines (want 1)\n", got)
	if got != 1 {
		t.Errorf("ragged-right+hyphenation broke into %d lines, want 1", got)
	}
}
