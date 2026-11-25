package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// inkSpaceNodes returns the node list of text set in testdata/ink-space.ttf,
// whose spaces are color glyphs, at 10pt.
func inkSpaceNodes(t *testing.T, ws WhiteSpace, text string) node.Node {
	t.Helper()
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("ink")
	if err := ff.AddMember(&FontSource{Location: "testdata/ink-space.ttf"}, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	if ws != WhiteSpaceNormal {
		te.Settings[SettingWhiteSpace] = ws
	}
	te.Items = append(te.Items, text)
	head, _, err := fe.Mknodes(te)
	if err != nil {
		t.Fatal(err)
	}
	return head
}

// firstOf returns the first node of type T in the list.
func firstOf[T node.Node](head node.Node) (T, bool) {
	for n := head; n != nil; n = n.Next() {
		if v, ok := n.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// A word space whose glyph has ink keeps the glyph as a leader shown once
// at the start of the glue. The glue stretches, the stretch stays empty,
// and it does not shrink below the glyph.
func TestAWordSpaceWithInk(t *testing.T) {
	g, ok := firstOf[*node.Glue](inkSpaceNodes(t, WhiteSpaceNormal, "b c"))
	if !ok {
		t.Fatal("no glue")
	}
	if g.Leader == nil || g.LeaderType != node.LeaderSingle {
		t.Fatalf("glue without a single leader: %+v", g)
	}
	sp, ok := g.Leader.List.(*node.Glyph)
	if !ok || sp.Components != " " || !near(sp.Width, bag.MustSP("2.5pt")) {
		t.Errorf("leader %v, want the space glyph, 2.5pt wide", g.Leader.List)
	}
	if g.Shrink != 0 || g.Stretch == 0 {
		t.Errorf("glue stretch %s shrink %s, want stretch and no shrink", g.Stretch, g.Shrink)
	}
}

// A thin space with ink is a fixed-width glue with its glyph as leader.
func TestAThinSpaceWithInk(t *testing.T) {
	g, ok := firstOf[*node.Glue](inkSpaceNodes(t, WhiteSpaceNormal, "b c"))
	if !ok || g.Leader == nil || g.LeaderType != node.LeaderSingle {
		t.Fatalf("thin space without a single leader: %+v", g)
	}
	if !near(g.Width, bag.MustSP("1pt")) || g.Stretch != 0 || g.Shrink != 0 {
		t.Errorf("thin space %s+%s-%s, want a rigid 1pt", g.Width, g.Stretch, g.Shrink)
	}
}

// Under white-space: pre a space with ink is a glyph, not an empty rule.
func TestAPreservedSpaceWithInk(t *testing.T) {
	head := inkSpaceNodes(t, WhiteSpacePre, "b c")
	if _, ok := firstOf[*node.Rule](head); ok {
		t.Error("the space is an empty rule")
	}
	var spaces int
	for n := head; n != nil; n = n.Next() {
		if g, ok := n.(*node.Glyph); ok && g.Components == " " {
			spaces++
		}
	}
	if spaces != 1 {
		t.Errorf("%d space glyphs, want 1", spaces)
	}
}

// A space without ink stays a plain glue.
func TestAWordSpaceWithoutInk(t *testing.T) {
	fe, ff := lineModelDocument(t)
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	te.Items = append(te.Items, "b c")
	head, _, err := fe.Mknodes(te)
	if err != nil {
		t.Fatal(err)
	}
	if g, ok := firstOf[*node.Glue](head); !ok || g.Leader != nil {
		t.Errorf("glue %+v, want one without leader", g)
	}
}
