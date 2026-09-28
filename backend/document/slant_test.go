package document

import (
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// A font with a synthetic slant shears its glyphs through the text matrix, and
// the upright glyphs after it get an unsheared one back.
func TestSlantedFontShearsTheTextMatrix(t *testing.T) {
	var slanted *font.Font
	out, _, _ := renderGlyphs(t, "abcd", func(gs []*node.Glyph) {
		c := *gs[0].Font
		c.Slant = 0.2126
		slanted = &c
		gs[0].Font, gs[1].Font = slanted, slanted
	})
	lines := tjLines(out)
	if !strings.Contains(out, "1 0 0.2126 1 ") {
		t.Fatalf("no sheared Tm:\n%s", lines)
	}
	at := strings.Index(out, "1 0 0.2126 1 ")
	if !strings.Contains(out[at:], "1 0 0 1 ") {
		t.Errorf("the upright glyphs after the slanted ones keep the shear:\n%s", lines)
	}
}
