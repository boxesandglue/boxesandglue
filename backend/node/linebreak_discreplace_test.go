package node

import (
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// ligatureWord builds "Schi[fff]ahrt b" with a Disc before the fff ligature
// that breaks it as Schiff-fahrt: Pre is ff and a hyphen, Post is f, and a
// break drops the ligature (bag#90). Glyphs are 1pt wide, the ligature
// 2.5pt, the space 1pt with 1pt stretch.
func ligatureWord() Node {
	head, cur := glyphRun(nil, nil, "Schi", bag.Factor)
	disc := NewDisc()
	disc.Pre, _ = glyphRun(nil, nil, "ff-", bag.Factor)
	disc.Post, _ = glyphRun(nil, nil, "f", bag.Factor)
	disc.ReplaceCount = 1
	head = InsertAfter(head, cur, disc)
	lig := NewGlyph()
	lig.Components = "fff"
	lig.Width = 5 * bag.Factor / 2
	head = InsertAfter(head, disc, lig)
	head, cur = glyphRun(head, lig, "ahrt", bag.Factor)
	sp := NewGlue()
	sp.Width = bag.Factor
	sp.Stretch = bag.Factor
	head = InsertAfter(head, cur, sp)
	head, cur = glyphRun(head, sp, "b", bag.Factor)
	head, _ = AppendLineEndAfter(head, cur)
	return head
}

// lineTexts is the text of each line, ligatures in brackets.
func lineTexts(vl *VList) []string {
	var lines []string
	for n := vl.List; n != nil; n = n.Next() {
		hl, ok := n.(*HList)
		if !ok {
			continue
		}
		var b strings.Builder
		for m := hl.List; m != nil; m = m.Next() {
			switch v := m.(type) {
			case *Glyph:
				if len(v.Components) > 1 {
					b.WriteString("[" + v.Components + "]")
				} else {
					b.WriteString(v.Components)
				}
			case *Glue:
				if o, _ := v.Attributes["origin"].(string); o != "leftskip" && o != "lineend" && v.Width > 0 {
					b.WriteString(" ")
				}
			}
		}
		lines = append(lines, b.String())
	}
	return lines
}

// A break inside a ligature ends the line with Pre and starts the next one
// with Post, and the ligature is gone. The first pass measures the second
// line with Post, 7pt: measured with the ligature it would be 8.5pt, past
// the 7pt measure.
func TestDiscReplaceBreaks(t *testing.T) {
	s := NewLinebreakSettings()
	s.HSize = 7 * bag.Factor
	s.LineHeight = 12 * bag.Factor
	vl, bps := Linebreak(ligatureWord(), s)
	if got, want := strings.Join(lineTexts(vl), "|"), "Schiff-|fahrt b"; got != want {
		t.Errorf("lines %q, want %q", got, want)
	}
	for _, bp := range bps {
		if bp.Emergency || bp.R != 0 {
			t.Errorf("line %d: r %g, emergency %t, want an exact fit", bp.Line, bp.R, bp.Emergency)
		}
	}
	if got, want := lineContentWidths(vl), []bag.ScaledPoint{7 * bag.Factor, 7 * bag.Factor}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("line widths %v, want %v", got, want)
	}
}

// Without a break there the ligature stays, and neither Pre nor Post shows.
func TestDiscReplaceNoBreak(t *testing.T) {
	s := NewLinebreakSettings()
	s.HSize = 20 * bag.Factor
	s.LineHeight = 12 * bag.Factor
	vl, _ := Linebreak(ligatureWord(), s)
	if got, want := strings.Join(lineTexts(vl), "|"), "Schi[fff]ahrt b"; got != want {
		t.Errorf("lines %q, want %q", got, want)
	}
}

// The line breaker leaves the Disc's Pre and Post as they were, so it can
// break the same list again.
func TestDiscReplaceKeepsPreAndPost(t *testing.T) {
	head := ligatureWord()
	var disc *Disc
	for n := head; n != nil; n = n.Next() {
		if d, ok := n.(*Disc); ok {
			disc = d
		}
	}
	s := NewLinebreakSettings()
	s.HSize = 7 * bag.Factor
	s.LineHeight = 12 * bag.Factor
	Linebreak(head, s)
	if got := Tail(disc.Pre); got.(*Glyph).Components != "-" || disc.Pre.Prev() != nil {
		t.Errorf("Pre is linked into the line: ends with %v", got)
	}
	if disc.Post.Next() != nil || disc.Post.Prev() != nil {
		t.Error("Post is linked into the line")
	}
}
