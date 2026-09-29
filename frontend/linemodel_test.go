package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// recordingModel sets each line 20pt + 5pt with no glue between, and keeps
// the glyphs' line shifts and sizes it sees.
type recordingModel struct {
	shifts []bag.ScaledPoint
	sizes  []bag.ScaledPoint
}

func (m *recordingModel) LineBox(hl *node.HList, _ *node.LinebreakSettings) (bag.ScaledPoint, bag.ScaledPoint) {
	for n := hl.List; n != nil; n = n.Next() {
		if g, ok := n.(*node.Glyph); ok {
			m.shifts = append(m.shifts, g.LineShift)
			m.sizes = append(m.sizes, g.Font.Size)
		}
	}
	return bag.MustSP("20pt"), bag.MustSP("5pt")
}

func (m *recordingModel) Leading(*node.HList, *node.LinebreakSettings) *node.Glue { return nil }

func lineModelDocument(t *testing.T) (*Document, *FontFamily) {
	t.Helper()
	fe, err := NewForWriter(io.Discard)
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
	return fe, ff
}

// SettingLineModel reaches Linebreak through FormatParagraph, and a run's
// SettingLineShift reaches the model on its glyphs.
func TestSettingLineModel(t *testing.T) {
	fe, ff := lineModelDocument(t)
	m := &recordingModel{}
	raised := NewText()
	raised.Settings[SettingLineShift] = bag.MustSP("3pt")
	raised.Items = append(raised.Items, "b")
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	te.Settings[SettingHalfLeading] = true
	te.Settings[SettingLineModel] = node.LineModel(m)
	te.Items = append(te.Items, "a", raised)
	vl, _, err := fe.FormatParagraph(te, bag.MustSP("100pt"))
	if err != nil {
		t.Fatal(err)
	}
	var lines []*node.HList
	var glyphs []*node.Glyph
	for n := vl.List; n != nil; n = n.Next() {
		if hl, ok := n.(*node.HList); ok {
			lines = append(lines, hl)
			node.Walk(hl.List, func(n node.Node) bool {
				if g, ok := n.(*node.Glyph); ok {
					glyphs = append(glyphs, g)
				}
				return true
			})
		}
	}
	if len(lines) != 1 || lines[0].Height != bag.MustSP("20pt") || lines[0].Depth != bag.MustSP("5pt") {
		t.Fatalf("want one 20pt + 5pt line from the model, got %d lines", len(lines))
	}
	if len(m.shifts) != 2 || m.shifts[0] != 0 || m.shifts[1] != bag.MustSP("3pt") || m.sizes[0] != bag.MustSP("10pt") {
		t.Errorf("model saw line shifts %v and sizes %v, want [0 3pt] at 10pt", m.shifts, m.sizes)
	}
	if len(glyphs) != 2 || glyphs[1].YOffset != bag.MustSP("3pt") {
		t.Errorf("the raised glyph is not moved 3pt: %v", glyphs)
	}
}

// Without a model a line shift is a plain YOffset: the lines are as before.
func TestSettingLineShiftWithoutModel(t *testing.T) {
	format := func(setting SettingType) *node.VList {
		fe, ff := lineModelDocument(t)
		raised := NewText()
		raised.Settings[setting] = bag.MustSP("3pt")
		raised.Items = append(raised.Items, "b")
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Settings[SettingLeading] = bag.MustSP("12pt")
		te.Items = append(te.Items, "a", raised)
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("100pt"))
		if err != nil {
			t.Fatal(err)
		}
		return vl
	}
	a, b := format(SettingYOffset), format(SettingLineShift)
	if a.Height != b.Height || a.Depth != b.Depth {
		t.Errorf("line shift %s + %s, y offset %s + %s", b.Height, b.Depth, a.Height, a.Depth)
	}
}
