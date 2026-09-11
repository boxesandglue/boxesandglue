package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/color"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// TestDecorationColorDefaultsToTextColor guards the currentColor fallback:
// glyphs are filled but the decoration rule is stroked, so without an
// explicit stroking colour the rule falls back to the PDF default black.
// When no SettingTextDecorationColor is given, the run's SettingColor is
// handed to the drawing instead; an explicit decoration colour still wins.
func TestDecorationColorDefaultsToTextColor(t *testing.T) {
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

	// decorationColor returns the decorationcolor attribute of the run's
	// decoration marker, or nil when the marker carries none.
	decorationColor := func(t *testing.T, ts TypesettingSettings) *color.Color {
		t.Helper()
		ts[SettingFontFamily] = ff
		ts[SettingSize] = bag.MustSP("10pt")
		ts[SettingTextDecorationLine] = TextDecorationUnderline
		head, err := fe.BuildNodelistFromString(ts, "text")
		if err != nil {
			t.Fatal(err)
		}
		for n := head; n != nil; n = n.Next() {
			if ss, ok := n.(*node.StartStop); ok {
				if _, ok := ss.GetAttribute("decoration"); ok {
					if v, ok := ss.GetAttribute("decorationcolor"); ok {
						return v.(*color.Color)
					}
					return nil
				}
			}
		}
		t.Fatal("no decoration marker in the list")
		return nil
	}

	t.Run("text colour fills in", func(t *testing.T) {
		col := decorationColor(t, TypesettingSettings{SettingColor: "red"})
		if col == nil {
			t.Fatal("expected the run's text colour on the decoration marker")
		}
		if want := fe.GetColor("red").PDFStringStroking(); col.PDFStringStroking() != want {
			t.Errorf("decoration colour = %s, want %s", col.PDFStringStroking(), want)
		}
	})

	t.Run("explicit decoration colour wins", func(t *testing.T) {
		col := decorationColor(t, TypesettingSettings{
			SettingColor:               "red",
			SettingTextDecorationColor: fe.GetColor("blue"),
		})
		if col == nil {
			t.Fatal("expected the explicit decoration colour on the marker")
		}
		if want := fe.GetColor("blue").PDFStringStroking(); col.PDFStringStroking() != want {
			t.Errorf("decoration colour = %s, want %s", col.PDFStringStroking(), want)
		}
	})

	t.Run("no colour at all stays bare", func(t *testing.T) {
		if col := decorationColor(t, TypesettingSettings{}); col != nil {
			t.Errorf("expected no decorationcolor attribute, got %s", col.PDFStringStroking())
		}
	})
}
