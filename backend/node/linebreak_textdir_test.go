package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

func edgeGlues(hl *HList) (leftskip, lineend *Glue) {
	if g, ok := hl.List.(*Glue); ok {
		leftskip = g
	}
	if g, ok := Tail(hl.List).(*Glue); ok {
		lineend = g
	}
	return leftskip, lineend
}

// Linebreak stamps the paragraph direction on the paragraph box and on
// every line, so later passes know which edge the line end is.
func TestLinebreakCarriesTheTextDirection(t *testing.T) {
	for _, dir := range []TextDirection{TextDirLTR, TextDirRTL} {
		var head, cur Node
		head, cur = glyphRun(head, cur, "ABCD", 10*bag.Factor)
		AppendLineEndAfter(head, cur)
		settings := NewLinebreakSettings()
		settings.HSize = 200 * bag.Factor
		settings.LineHeight = 12 * bag.Factor
		settings.TextDirection = dir

		vlist, _ := Linebreak(head, settings)
		if vlist.TextDir != dir {
			t.Errorf("paragraph box: TextDir %s, want %s", vlist.TextDir, dir)
		}
		for n := vlist.List; n != nil; n = n.Next() {
			if hl, ok := n.(*HList); ok && hl.TextDir != dir {
				t.Errorf("line: TextDir %s, want %s", hl.TextDir, dir)
			}
		}
	}
}

// A line that ends in a forced break is not justified: its slack goes to
// the line end, which is the right edge of a left to right line and the
// left edge of a right to left one.
func TestHardBreakSlackGoesToTheLineEnd(t *testing.T) {
	build := func(dir TextDirection) *HList {
		var head, cur Node
		head, cur = glyphRun(head, cur, "AB", 10*bag.Factor)
		hb := NewHardBreak()
		head = InsertAfter(head, cur, hb)
		cur = hb
		head, cur = glyphRun(head, cur, "CD", 10*bag.Factor)
		AppendLineEndAfter(head, cur)
		settings := NewLinebreakSettings()
		settings.HSize = 200 * bag.Factor
		settings.LineHeight = 12 * bag.Factor
		settings.TextDirection = dir
		vlist, _ := Linebreak(head, settings)
		return vlist.List.(*HList)
	}
	leftskip, lineend := edgeGlues(build(TextDirLTR))
	if leftskip == nil || lineend == nil {
		t.Fatal("ltr: line without edge glues")
	}
	if leftskip.StretchOrder >= StretchFil || lineend.StretchOrder < StretchFil {
		t.Errorf("ltr: slack on the left (leftskip order %d, lineend order %d), want it on the right", leftskip.StretchOrder, lineend.StretchOrder)
	}
	leftskip, lineend = edgeGlues(build(TextDirRTL))
	if leftskip == nil || lineend == nil {
		t.Fatal("rtl: line without edge glues")
	}
	if leftskip.StretchOrder < StretchFil || lineend.StretchOrder >= StretchFil {
		t.Errorf("rtl: slack on the right (leftskip order %d, lineend order %d), want it on the left", leftskip.StretchOrder, lineend.StretchOrder)
	}
}

// Hanging punctuation lets the glyph protrude past the line end. Left to
// right, the glyph drops its advance. Right to left, the glyph ends up at
// the left edge after the bidi reorder, so it keeps its advance and a kern
// of the same amount follows it, which the reorder moves to its left.
func TestHangingPunctuationProtrudesAtTheLineEnd(t *testing.T) {
	const w = 10 * bag.Factor
	build := func(dir TextDirection) *Glyph {
		var head, cur Node
		head, cur = glyphRun(head, cur, "AB.", w)
		dot := cur.(*Glyph)
		g := NewGlue()
		g.Width = 5 * bag.Factor
		head = InsertAfter(head, cur, g)
		cur = g
		head, cur = glyphRun(head, cur, "CD", w)
		AppendLineEndAfter(head, cur)
		settings := NewLinebreakSettings()
		settings.HSize = 32 * bag.Factor // "AB." fits, "AB. CD" does not
		settings.LineHeight = 12 * bag.Factor
		settings.HangingPunctuationEnd = true
		settings.TextDirection = dir
		// Ragged: the slack of the short first line goes into the line end.
		settings.LineEndGlue = NewGlue()
		settings.LineEndGlue.Stretch = bag.Factor
		settings.LineEndGlue.StretchOrder = StretchFill
		vlist, _ := Linebreak(head, settings)
		if got := countHLists(vlist); got != 2 {
			t.Fatalf("%s: got %d lines, want 2", dir, got)
		}
		return dot
	}
	dot := build(TextDirLTR)
	if dot.Width != 0 || dot.XOffset != 0 {
		t.Errorf("ltr: width %s, xoffset %s; want 0 and 0", dot.Width, dot.XOffset)
	}
	dot = build(TextDirRTL)
	if dot.Width != w {
		t.Errorf("rtl: width %s, want %s (the advance stays)", dot.Width, w)
	}
	if k, ok := dot.Next().(*Kern); !ok || k.Kern != -w {
		t.Errorf("rtl: after the glyph %v, want a kern of %s", dot.Next(), -w)
	}
}
