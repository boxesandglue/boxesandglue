package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// TestColorKeepsDecoration guards the interaction between SettingColor and
// SettingTextDecorationLine. Both prepend a marker to the run, and the colour
// marker was made the head unconditionally, which dropped whatever was already
// there: with any colour in scope the decoration's start marker fell off the
// front of the list and no underline was drawn.
//
// Both markers only affect what follows them, so both have to sit before the
// first glyph.
func TestColorKeepsDecoration(t *testing.T) {
	fe, err := initDocument(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("test")
	if err := ff.AddMember(
		&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"},
		FontWeight400, FontStyleNormal,
	); err != nil {
		t.Fatal(err)
	}

	// markersBeforeText reports which of the two markers precede the first glyph.
	markersBeforeText := func(t *testing.T, withColor bool) (decoration, color bool) {
		t.Helper()
		ts := TypesettingSettings{
			SettingFontFamily:         ff,
			SettingSize:               bag.MustSP("10pt"),
			SettingTextDecorationLine: TextDecorationUnderline,
		}
		if withColor {
			ts[SettingColor] = "red"
		}
		head, err := fe.BuildNodelistFromString(ts, "text")
		if err != nil {
			t.Fatal(err)
		}
		for n := head; n != nil; n = n.Next() {
			switch v := n.(type) {
			case *node.StartStop:
				if _, ok := v.GetAttribute("decoration"); ok {
					decoration = true
				}
				if v.ShipoutCallback != nil {
					color = true
				}
			case *node.Glyph:
				return decoration, color
			}
		}
		t.Fatal("no glyphs in the list")
		return
	}

	t.Run("without a colour", func(t *testing.T) {
		if decoration, _ := markersBeforeText(t, false); !decoration {
			t.Error("no decoration marker before the text")
		}
	})

	t.Run("with a colour", func(t *testing.T) {
		decoration, color := markersBeforeText(t, true)
		if !decoration {
			t.Error("no decoration marker before the text: the colour marker displaced it")
		}
		if !color {
			t.Error("no colour marker before the text, so the run is not coloured")
		}
	})
}
