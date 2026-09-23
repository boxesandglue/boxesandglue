package frontend

import (
	"bytes"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/node"
)

func TestDetectParagraphDirection(t *testing.T) {
	cases := []struct {
		name string
		text string
		want Direction
	}{
		{"latin", "Hello world", DirectionLTR},
		{"hebrew", "שלום עולם", DirectionRTL},
		{"arabic", "مرحبا بالعالم", DirectionRTL},
		{"empty", "", DirectionLTR},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			te := NewText()
			te.Items = append(te.Items, tc.text)
			if got := detectParagraphDirection(te); got != tc.want {
				t.Errorf("detectParagraphDirection(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestDetectParagraphDirectionNested(t *testing.T) {
	// Direction should be detected across nested Text items, since real
	// paragraphs are built from spans (e.g., bold/italic) of mixed nesting.
	outer := NewText()
	inner := NewText()
	inner.Items = append(inner.Items, "שלום")
	outer.Items = append(outer.Items, inner, " עולם")
	if got := detectParagraphDirection(outer); got != DirectionRTL {
		t.Errorf("nested Hebrew got %v, want DirectionRTL", got)
	}
}

// makeLine builds a synthetic HList with one Glyph node per cluster, each
// glyph tagged with the matching bidi level. Components carry the cluster
// label so a test can assert visual order by reading them back.
func makeLine(clusters []struct {
	label string
	level uint8
}) *node.HList {
	hl := node.NewHList()
	var head, tail node.Node
	for _, c := range clusters {
		g := node.NewGlyph()
		g.Components = c.label
		g.SetBidiLevel(c.level)
		head = node.InsertAfter(head, tail, g)
		tail = g
	}
	hl.List = head
	return hl
}

func lineLabels(hl *node.HList) []string {
	var out []string
	for n := hl.List; n != nil; n = n.Next() {
		if g, ok := n.(*node.Glyph); ok {
			out = append(out, g.Components)
		}
	}
	return out
}

func TestBidiReorderLine(t *testing.T) {
	type cluster = struct {
		label string
		level uint8
	}
	cases := []struct {
		name string
		in   []cluster
		want []string
	}{
		{
			name: "all LTR is unchanged",
			in:   []cluster{{"H", 0}, {"i", 0}},
			want: []string{"H", "i"},
		},
		{
			name: "single RTL run is unchanged (shaper output already visual)",
			in:   []cluster{{"שלום", 1}},
			want: []string{"שלום"},
		},
		{
			name: "LTR-RTL-LTR within LTR base swaps middle's position only when isolated",
			in:   []cluster{{"Hello ", 0}, {"עולם", 1}, {", world", 0}},
			want: []string{"Hello ", "עולם", ", world"},
		},
		{
			name: "RTL-LTR-RTL within RTL base reverses run order",
			in:   []cluster{{"שלום ", 1}, {"world", 2}, {" עברית", 1}},
			want: []string{" עברית", "world", "שלום "},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hl := makeLine(tc.in)
			// paragraphLevel doesn't matter here — none of these inputs has a
			// trailing whitespace cluster, so L1 is a no-op.
			bidiReorderLine(hl, 0)
			got := lineLabels(hl)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d items, want %d (%v vs %v)", len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("at index %d: got %q, want %q (full: %v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

// TestApplyL1 verifies that trailing whitespace at line end gets its
// BidiLevel reset to the paragraph base, while inner whitespace at the
// same elevated level is left alone.
func TestApplyL1(t *testing.T) {
	// Build a line resembling: glyph(lvl 1), glue(lvl 1) [inner], glyph(lvl 2),
	// glue(lvl 2) [trailing], penalty(lvl 0) [linebreak artefact].
	g1 := node.NewGlyph()
	g1.Components = "ש"
	g1.SetBidiLevel(1)
	innerGlue := node.NewGlue()
	innerGlue.SetBidiLevel(1)
	g2 := node.NewGlyph()
	g2.Components = "w"
	g2.SetBidiLevel(2)
	trailingGlue := node.NewGlue()
	trailingGlue.SetBidiLevel(2)
	pen := node.NewPenalty()
	pen.SetBidiLevel(0)
	var head, tail node.Node
	for _, n := range []node.Node{g1, innerGlue, g2, trailingGlue, pen} {
		head = node.InsertAfter(head, tail, n)
		tail = n
	}
	hl := node.NewHList()
	hl.List = head

	applyL1(hl, 1) // paragraph base = RTL

	if got := trailingGlue.BidiLevel(); got != 1 {
		t.Errorf("trailing glue BidiLevel = %d, want 1 (paragraph base)", got)
	}
	if got := innerGlue.BidiLevel(); got != 1 {
		t.Errorf("inner glue BidiLevel = %d, want 1 (unchanged)", got)
	}
	if got := g1.BidiLevel(); got != 1 {
		t.Errorf("g1 BidiLevel changed to %d, want 1", got)
	}
	if got := g2.BidiLevel(); got != 2 {
		t.Errorf("g2 BidiLevel changed to %d, want 2 (must not be reset)", got)
	}
	if got := pen.BidiLevel(); got != 0 {
		t.Errorf("penalty BidiLevel = %d, want 0 (linebreak artefact untouched)", got)
	}
}

// TestApplyL1ResetsTabs: a tab inside an embedded run, and the whitespace
// before it, go to the paragraph level, so that in a right to left paragraph
// the segments between the tab stops keep their order from the right edge.
func TestApplyL1ResetsTabs(t *testing.T) {
	var nodes []node.Node
	add := func(n node.Node, level uint8) node.Node {
		n.SetBidiLevel(level)
		nodes = append(nodes, n)
		return n
	}
	g1 := add(node.NewGlyph(), 2)
	space := add(node.NewGlue(), 2)
	tab := node.NewGlue()
	tab.Subtype = node.GlueTab
	add(tab, 2)
	g2 := add(node.NewGlyph(), 2)
	var head, tail node.Node
	for _, n := range nodes {
		head = node.InsertAfter(head, tail, n)
		tail = n
	}
	hl := node.NewHList()
	hl.List = head

	applyL1(hl, 1)

	for _, c := range []struct {
		name string
		n    node.Node
		want uint8
	}{{"glyph before", g1, 2}, {"space before the tab", space, 1}, {"tab", tab, 1}, {"glyph after", g2, 2}} {
		if got := c.n.BidiLevel(); got != c.want {
			t.Errorf("%s: BidiLevel %d, want %d", c.name, got, c.want)
		}
	}
}

// TestApplyL1NoTrailingWhitespace covers the case where the line ends in a
// glyph (no trailing whitespace at all). L1 should be a no-op.
func TestApplyL1NoTrailingWhitespace(t *testing.T) {
	g := node.NewGlyph()
	g.Components = "x"
	g.SetBidiLevel(2)
	hl := node.NewHList()
	hl.List = g

	applyL1(hl, 1)

	if got := g.BidiLevel(); got != 2 {
		t.Errorf("glyph BidiLevel = %d, want 2 (must not be reset)", got)
	}
}

func TestCollectParagraphTextIgnoresNonStrings(t *testing.T) {
	// Images and other non-string items are bidi-neutral and must not crash
	// the collector.
	te := NewText()
	te.Items = append(te.Items, "abc", 42, struct{ X int }{1}, "def")
	var b bytes.Buffer
	collectParagraphText(te, &b)
	if got := b.String(); got != "abcdef" {
		t.Errorf("collectParagraphText = %q, want %q", got, "abcdef")
	}
}

// TestBidiReorderLineKeepsEdgeGlues verifies that the linebreaker's own
// leftskip and line-end glue stay at their physical edges in an RTL
// paragraph while the content between them, the paragraph's fill glue
// included, is mirrored.
func TestBidiReorderLineKeepsEdgeGlues(t *testing.T) {
	glue := func(origin string, level uint8) *node.Glue {
		g := node.NewGlue()
		g.Attributes = node.H{"origin": origin}
		g.SetBidiLevel(level)
		return g
	}
	glyph := func(label string) *node.Glyph {
		g := node.NewGlyph()
		g.Components = label
		g.SetBidiLevel(1)
		return g
	}
	leftskip := glue("leftskip", 0)
	parfill := glue("lineend", 0) // AppendLineEndAfter's fill glue, still level 0 before L1
	lineend := glue("lineend", 0)
	a, b := glyph("א"), glyph("ב")
	var head, tail node.Node
	for _, n := range []node.Node{leftskip, a, b, parfill, lineend} {
		head = node.InsertAfter(head, tail, n)
		tail = n
	}
	hl := node.NewHList()
	hl.List = head

	bidiReorderLine(hl, 1)

	var got []node.Node
	for n := hl.List; n != nil; n = n.Next() {
		got = append(got, n)
	}
	want := []node.Node{leftskip, parfill, b, a, lineend}
	if len(got) != len(want) {
		t.Fatalf("got %d nodes, want %d", len(got), len(want))
	}
	names := []string{"leftskip", "parfill", "ב", "א", "lineend"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: want %s", i, names[i])
		}
	}
	if got := parfill.BidiLevel(); got != 1 {
		t.Errorf("parfill BidiLevel = %d, want 1 (L1 resets trailing glue)", got)
	}
	if got := lineend.BidiLevel(); got != 0 {
		t.Errorf("lineend BidiLevel = %d, want 0 (edge glue untouched)", got)
	}
}

// TestBidiReorderLastLineFillGoesLeft covers the last line of a justified
// RTL paragraph: the paragraph's fill glue sits behind a penalty at the
// logical end and has to travel to the visual left with the text, so the
// line is flush right. The penalty must not cut the run.
func TestBidiReorderLastLineFillGoesLeft(t *testing.T) {
	glue := func(origin string) *node.Glue {
		g := node.NewGlue()
		g.Attributes = node.H{"origin": origin}
		return g
	}
	glyph := func(label string) *node.Glyph {
		g := node.NewGlyph()
		g.Components = label
		g.SetBidiLevel(1)
		return g
	}
	leftskip, parfill, lineend := glue("leftskip"), glue("lineend"), glue("lineend")
	pen := node.NewPenalty()
	pen.Penalty = 10000
	a, b := glyph("א"), glyph("ב")
	var head, tail node.Node
	for _, n := range []node.Node{leftskip, a, b, pen, parfill, lineend} {
		head = node.InsertAfter(head, tail, n)
		tail = n
	}
	hl := node.NewHList()
	hl.List = head

	bidiReorderLine(hl, 1)

	want := []node.Node{leftskip, parfill, pen, b, a, lineend}
	names := []string{"leftskip", "parfill", "penalty", "ב", "א", "lineend"}
	i := 0
	for n := hl.List; n != nil; n = n.Next() {
		if i >= len(want) {
			t.Fatalf("more than %d nodes", len(want))
		}
		if n != want[i] {
			t.Errorf("position %d: want %s", i, names[i])
		}
		i++
	}
	if i != len(want) {
		t.Fatalf("got %d nodes, want %d", i, len(want))
	}
}
