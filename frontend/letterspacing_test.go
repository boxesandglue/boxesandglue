package frontend

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// CSS Text 3 §7.2: letter-spacing is added after every typographic character
// unit, a space included, so a letter-spaced space is that much wider: the
// inter-word glue, and the glue or rule of a preserved space.
func TestLetterSpacingFollowsASpace(t *testing.T) {
	spacing := bag.MustSP("2pt")
	space := func(ls bag.ScaledPoint, ws WhiteSpace, text string) bag.ScaledPoint {
		fe, ff := lineModelDocument(t)
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		if ws != WhiteSpaceNormal {
			te.Settings[SettingWhiteSpace] = ws
		}
		if ls != 0 {
			te.Settings[SettingLetterSpacing] = ls
		}
		te.Items = append(te.Items, text)
		head, _, err := fe.Mknodes(te)
		if err != nil {
			t.Fatal(err)
		}
		for n := head; n != nil; n = n.Next() {
			switch v := n.(type) {
			case *node.Glue:
				return v.Width
			case *node.Rule:
				return v.Width
			}
		}
		t.Fatal("no space")
		return 0
	}
	for _, tc := range []struct {
		name string
		ws   WhiteSpace
		text string
	}{
		{"normal", WhiteSpaceNormal, "a b"},
		{"pre", WhiteSpacePre, "a b"},
		{"pre-wrap", WhiteSpacePreWrap, "a b"},
		{"thin space, pre", WhiteSpacePre, "a\u2009b"},
	} {
		plain, spaced := space(0, tc.ws, tc.text), space(spacing, tc.ws, tc.text)
		if spaced != plain+spacing {
			t.Errorf("%s: space %s with letter-spacing %s, want %s", tc.name, spaced, spacing, plain+spacing)
		}
	}
}
