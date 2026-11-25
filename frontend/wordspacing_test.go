package frontend

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// wordSpaceGlue returns the first glue or rule of text set in TeX Gyre
// Heros at 10pt with the given settings, and the font's space width.
func wordSpaceGlue(t *testing.T, text string, settings TypesettingSettings) (spaceNode, bag.ScaledPoint) {
	t.Helper()
	fe, ff := lineModelDocument(t)
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	for k, v := range settings {
		te.Settings[k] = v
	}
	te.Items = append(te.Items, text)
	head, _, err := fe.Mknodes(te)
	if err != nil {
		t.Fatal(err)
	}
	var space bag.ScaledPoint
	for n := head; n != nil; n = n.Next() {
		if g, ok := n.(*node.Glyph); ok {
			space = g.Font.Space
			break
		}
	}
	for n := head; n != nil; n = n.Next() {
		switch v := n.(type) {
		case *node.Glue:
			return spaceNode{width: v.Width, stretch: v.Stretch, shrink: v.Shrink}, space
		case *node.Rule:
			return spaceNode{width: v.Width, rule: true}, space
		}
	}
	t.Fatal("no space")
	return spaceNode{}, 0
}

// Without settings a word space is the font's space, stretching by a half
// and shrinking by a third.
func TestWordSpacingDefault(t *testing.T) {
	got, space := wordSpaceGlue(t, "a b", nil)
	if got.width != space || got.stretch != space/2 || got.shrink != space/3 {
		t.Errorf("word space %+v, want %s+%s-%s", got, space, space/2, space/3)
	}
}

// SettingWordSpacing adds to the width of a word space, as CSS
// word-spacing does, and leaves stretch and shrink as they are.
func TestWordSpacingAdds(t *testing.T) {
	plain, _ := wordSpaceGlue(t, "a b", nil)
	got, _ := wordSpaceGlue(t, "a b", TypesettingSettings{SettingWordSpacing: bag.MustSP("2pt")})
	if got.width != plain.width+bag.MustSP("2pt") || got.stretch != plain.stretch || got.shrink != plain.shrink {
		t.Errorf("word space %+v, want %+v wider by 2pt", got, plain)
	}
	// A preserved space gets it too, a thin space is no word space.
	pre, _ := wordSpaceGlue(t, "a b", TypesettingSettings{SettingWhiteSpace: WhiteSpacePre})
	preWide, _ := wordSpaceGlue(t, "a b", TypesettingSettings{SettingWhiteSpace: WhiteSpacePre, SettingWordSpacing: bag.MustSP("2pt")})
	if preWide.width != pre.width+bag.MustSP("2pt") {
		t.Errorf("preserved space %s with word-spacing 2pt, want %s", preWide.width, pre.width+bag.MustSP("2pt"))
	}
	thin, _ := wordSpaceGlue(t, "a b", nil)
	thinWide, _ := wordSpaceGlue(t, "a b", TypesettingSettings{SettingWordSpacing: bag.MustSP("2pt")})
	if thinWide.width != thin.width {
		t.Errorf("thin space %s with word-spacing 2pt, want %s", thinWide.width, thin.width)
	}
}

// SettingWordSpacingLimits sets the natural width, stretch and shrink of a
// word space as factors of the space width.
func TestWordSpacingLimits(t *testing.T) {
	ws := WordSpacing{Min: 0.8, Opt: 1.1, Max: 1.5}
	got, space := wordSpaceGlue(t, "a b", TypesettingSettings{SettingWordSpacingLimits: ws})
	want := spaceNode{
		width:   bag.MultiplyFloat(space, 1.1),
		stretch: bag.MultiplyFloat(space, 0.4),
		shrink:  bag.MultiplyFloat(space, 0.3),
	}
	if !near(got.width, want.width) || !near(got.stretch, want.stretch) || !near(got.shrink, want.shrink) {
		t.Errorf("word space %+v, want %+v", got, want)
	}
	// The additive word-spacing moves all three.
	got, _ = wordSpaceGlue(t, "a b", TypesettingSettings{SettingWordSpacingLimits: ws, SettingWordSpacing: bag.MustSP("1pt")})
	if !near(got.width, want.width+bag.MustSP("1pt")) || !near(got.stretch, want.stretch) {
		t.Errorf("word space with word-spacing %+v, want width %s", got, want.width+bag.MustSP("1pt"))
	}
	// Limits on the wrong side of Opt neither stretch nor shrink.
	got, _ = wordSpaceGlue(t, "a b", TypesettingSettings{SettingWordSpacingLimits: WordSpacing{Min: 1.2, Opt: 1, Max: 0.9}})
	if got.stretch != 0 || got.shrink != 0 {
		t.Errorf("word space %+v, want no stretch and no shrink", got)
	}
}

// In ragged text a word space has its natural width, Opt times the space
// width: a line with three word spaces at Opt 2 ends three spaces later.
func TestWordSpacingOptInRaggedText(t *testing.T) {
	fe, ff := lineModelDocument(t)
	end := func(ws *WordSpacing) bag.ScaledPoint {
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Settings[SettingHAlign] = HAlignLeft
		if ws != nil {
			te.Settings[SettingWordSpacingLimits] = *ws
		}
		te.Items = append(te.Items, "one two three four")
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("120mm"))
		if err != nil {
			t.Fatal(err)
		}
		return runEnds(vl)[0]
	}
	_, space := wordSpaceGlue(t, "a b", nil)
	plain, wide := end(nil), end(&WordSpacing{Min: 1, Opt: 2, Max: 3})
	if diff := wide - plain - 3*space; diff > 3 || diff < -3 {
		t.Errorf("line ends at %s with Opt 2, %s without, want 3 spaces (%s) later", wide, plain, 3*space)
	}
}
