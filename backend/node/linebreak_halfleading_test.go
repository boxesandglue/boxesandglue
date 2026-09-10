package node

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// glyphRunHD appends glyphs with explicit vertical metrics, so the built
// lines have a realistic height and depth.
func glyphRunHD(head, cur Node, s string, width, height, depth bag.ScaledPoint) (Node, Node) {
	for _, r := range s {
		g := NewGlyph()
		g.Width = width
		g.Height = height
		g.Depth = depth
		g.Components = string(r)
		head = InsertAfter(head, cur, g)
		cur = g
	}
	return head, cur
}

// buildTwoLineParagraph returns a node list that breaks into exactly two
// lines at the given HSize (two words per line, four words total).
func buildTwoLineParagraph(charWidth, height, depth bag.ScaledPoint) Node {
	const spaceWidth = bag.ScaledPoint(3 * bag.Factor)
	var head, cur Node
	for i := range 4 {
		if i > 0 {
			sp := NewGlue()
			sp.Width = spaceWidth
			sp.Stretch = bag.Factor
			sp.Shrink = bag.Factor / 2
			head = InsertAfter(head, cur, sp)
			cur = sp
		}
		head, cur = glyphRunHD(head, cur, "word", charWidth, height, depth)
	}
	head, _ = AppendLineEndAfter(head, cur)
	return head
}

func linesOf(v *VList) []*HList {
	var lines []*HList
	for n := v.List; n != nil; n = n.Next() {
		if hl, ok := n.(*HList); ok {
			lines = append(lines, hl)
		}
	}
	return lines
}

// TestHalfLeading checks the CSS half-leading mode: the leading is
// distributed into each line box (half above, half below), no lineskip
// glue is emitted, and baseline distances and the total paragraph height
// match the trailing-leading default.
func TestHalfLeading(t *testing.T) {
	const charWidth = bag.ScaledPoint(6 * bag.Factor)
	height := bag.ScaledPoint(7 * bag.Factor)
	depth := bag.ScaledPoint(3 * bag.Factor)
	lineHeight := bag.ScaledPoint(12 * bag.Factor)

	newSettings := func(half bool) *LinebreakSettings {
		s := NewLinebreakSettings()
		s.HSize = 55 * bag.Factor // two 24pt words + space per line
		s.LineHeight = lineHeight
		s.HalfLeading = half
		return s
	}

	trailing, _ := Linebreak(buildTwoLineParagraph(charWidth, height, depth), newSettings(false))
	half, _ := Linebreak(buildTwoLineParagraph(charWidth, height, depth), newSettings(true))

	trailingLines := linesOf(trailing)
	halfLines := linesOf(half)
	if len(trailingLines) != 2 || len(halfLines) != 2 {
		t.Fatalf("expected 2 lines in both modes, got %d (trailing) and %d (half)", len(trailingLines), len(halfLines))
	}

	// Every half-leading line box spans the full line height, with the
	// leading split around the natural metrics: extra = 12 - (7+3) = 2pt.
	wantHeight := height + bag.Factor
	wantDepth := depth + bag.Factor
	for i, hl := range halfLines {
		if hl.Height != wantHeight || hl.Depth != wantDepth {
			t.Errorf("half-leading line %d has height %s depth %s, want %s and %s", i, hl.Height, hl.Depth, wantHeight, wantDepth)
		}
	}

	// No lineskip glue in half-leading mode, neither between lines nor
	// after the last one.
	for n := half.List; n != nil; n = n.Next() {
		if g, ok := n.(*Glue); ok {
			if origin, _ := g.Attributes["origin"].(string); origin == "lineskip" || origin == "last lineskip" {
				t.Errorf("half-leading mode must not emit %s glue", origin)
			}
		}
	}

	// Total paragraph height is unchanged: 2 x line height.
	if got, want := half.Height+half.Depth, trailing.Height+trailing.Depth; got != want {
		t.Errorf("half-leading paragraph is %s tall, trailing %s: they must match", got, want)
	}
	if got := half.Height + half.Depth; got != 2*lineHeight {
		t.Errorf("half-leading paragraph is %s tall, want %s", got, 2*lineHeight)
	}

	// Baseline-to-baseline distance is the line height in both modes.
	baselineGap := func(v *VList) bag.ScaledPoint {
		var gap bag.ScaledPoint
		lines := linesOf(v)
		// distance = depth of first line + everything between + height of second
		gap += lines[0].Depth + lines[1].Height
		for n := v.List; n != nil; n = n.Next() {
			if n == lines[1] {
				break
			}
			if g, ok := n.(*Glue); ok && n != v.List {
				gap += g.Width
			}
		}
		return gap
	}
	if got, want := baselineGap(half), baselineGap(trailing); got != want {
		t.Errorf("baseline distance differs: half %s, trailing %s", got, want)
	}
	if got := baselineGap(half); got != lineHeight {
		t.Errorf("baseline distance is %s, want %s", got, lineHeight)
	}
}

// TestHalfLeadingTallLine checks that a line taller than the line height
// keeps its natural size (no negative leading).
func TestHalfLeadingTallLine(t *testing.T) {
	const charWidth = bag.ScaledPoint(6 * bag.Factor)
	height := bag.ScaledPoint(11 * bag.Factor)
	depth := bag.ScaledPoint(4 * bag.Factor) // 15pt natural > 12pt line height

	head := buildTwoLineParagraph(charWidth, height, depth)
	settings := NewLinebreakSettings()
	settings.HSize = 55 * bag.Factor
	settings.LineHeight = 12 * bag.Factor
	settings.HalfLeading = true

	vl, _ := Linebreak(head, settings)
	for i, hl := range linesOf(vl) {
		if hl.Height != height || hl.Depth != depth {
			t.Errorf("tall line %d was resized to height %s depth %s, want natural %s and %s", i, hl.Height, hl.Depth, height, depth)
		}
	}
}
