package frontend

import (
	"io"
	"reflect"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
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

// lineTrims are the start and end trims of vl's lines, nil where a line has
// none.
func lineTrims(vl *node.VList) [][2]any {
	var trims [][2]any
	for n := vl.List; n != nil; n = n.Next() {
		if hl, ok := n.(*node.HList); ok {
			trims = append(trims, [2]any{hl.Attributes[node.LineTrimStart], hl.Attributes[node.LineTrimEnd]})
		}
	}
	return trims
}

// trimParagraph is a half-leading paragraph of family ff at 10pt with line
// height leading, recording its trims when record is set.
func trimParagraph(ff *FontFamily, leading bag.ScaledPoint, record bool, items ...any) *Text {
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	te.Settings[SettingLeading] = leading
	te.Settings[SettingHalfLeading] = true
	if record {
		te.Settings[SettingRecordLineTrims] = true
	}
	te.Items = append(te.Items, items...)
	return te
}

// With SettingRecordLineTrims, FormatParagraph gives Linebreak the
// paragraph's own font, so each line records its half-leading above that
// font's text-over edge and below its text-under edge. At a line height
// close to the font size the content area reaches past the line box, and
// the start trim is negative.
func TestParagraphRecordsTrims(t *testing.T) {
	for _, c := range []struct {
		name     string
		leading  string
		positive bool
	}{
		{"20pt", "20pt", true},
		{"11pt", "11pt", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			fe, ff := lineModelDocument(t)
			te := trimParagraph(ff, bag.MustSP(c.leading), true, "Text with a descender, y and g, on two lines.")
			vl, _, err := fe.FormatParagraph(te, bag.MustSP("100pt"))
			if err != nil {
				t.Fatal(err)
			}
			fs, err := ff.GetFontSource(FontWeight400, FontStyleNormal)
			if err != nil {
				t.Fatal(err)
			}
			fnt, _, _, _, err := fe.shapeFontFor(fs, bag.MustSP("10pt"), nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			lines := 0
			for n := vl.List; n != nil; n = n.Next() {
				hl, ok := n.(*node.HList)
				if !ok {
					continue
				}
				lines++
				got, ok := hl.Attributes[node.LineTrimStart].(bag.ScaledPoint)
				if want := hl.Height - fnt.ContentAscent; !ok || got != want || (want > 0) != c.positive {
					t.Errorf("line %d: start trim %v (%t), want %s", lines, got, ok, want)
				}
				got, ok = hl.Attributes[node.LineTrimEnd].(bag.ScaledPoint)
				if want := hl.Depth - fnt.ContentDescent; !ok || got != want || (c.positive && want <= 0) {
					t.Errorf("line %d: end trim %v (%t), want %s", lines, got, ok, want)
				}
			}
			if lines < 2 {
				t.Errorf("%d lines, want at least 2", lines)
			}
		})
	}
}

// Without SettingRecordLineTrims a paragraph records no trim and loads no
// face its glyphs don't use. With it, a paragraph whose own face no glyph is
// set in loads it and records the same trims alone as after a paragraph in
// that face.
func TestParagraphTrimsWithoutItsFace(t *testing.T) {
	boldParagraph := func(fe *Document, ff *FontFamily, record bool) [][2]any {
		t.Helper()
		bold := NewText()
		bold.Settings[SettingFontWeight] = FontWeight700
		bold.Items = append(bold.Items, "All of it bold, on two lines.")
		vl, _, err := fe.FormatParagraph(trimParagraph(ff, bag.MustSP("20pt"), record, bold), bag.MustSP("100pt"))
		if err != nil {
			t.Fatal(err)
		}
		return lineTrims(vl)
	}
	document := func() (*Document, *FontFamily) {
		fe, ff := lineModelDocument(t)
		if err := ff.AddMember(
			&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"},
			FontWeight700, FontStyleNormal,
		); err != nil {
			t.Fatal(err)
		}
		return fe, ff
	}

	fe, ff := document()
	for i, tr := range boldParagraph(fe, ff, false) {
		if tr[0] != nil || tr[1] != nil {
			t.Errorf("not recording: line %d has trims %v", i+1, tr)
		}
	}
	if fs, _ := ff.GetFontSource(FontWeight400, FontStyleNormal); fs.face != nil {
		t.Error("not recording: the paragraph's own face was loaded")
	}

	fe, ff = document()
	alone := boldParagraph(fe, ff, true)
	fe, ff = document()
	if _, _, err := fe.FormatParagraph(trimParagraph(ff, bag.MustSP("20pt"), false, "Regular."), bag.MustSP("100pt")); err != nil {
		t.Fatal(err)
	}
	after := boldParagraph(fe, ff, true)
	if len(alone) < 2 {
		t.Fatalf("%d lines, want at least 2", len(alone))
	}
	for i, tr := range alone {
		if tr[0] == nil || tr[1] == nil {
			t.Errorf("alone: line %d has trims %v", i+1, tr)
		}
	}
	if !reflect.DeepEqual(alone, after) {
		t.Errorf("trims alone %v, after a regular paragraph %v", alone, after)
	}
}

// The paragraph's font merges SettingFontVariationSettings over the
// source's own variation settings as its glyphs' font does, so a source
// pinned to an instance of a weight range, with variations on top, gets the
// glyphs' font.
func TestParagraphFontVariations(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("vf")
	ff.AddMemberRange(
		&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"},
		200, 900, FontStyleNormal,
	)
	te := trimParagraph(ff, bag.MustSP("20pt"), true, "Text")
	te.Settings[SettingFontWeight] = FontWeight700
	te.Settings[SettingFontVariationSettings] = map[string]float64{"wdth": 90}
	vl, _, err := fe.FormatParagraph(te, bag.MustSP("100pt"))
	if err != nil {
		t.Fatal(err)
	}
	var glyphFont *font.Font
	for n := vl.List; n != nil && glyphFont == nil; n = n.Next() {
		if hl, ok := n.(*node.HList); ok {
			for g := hl.List; g != nil; g = g.Next() {
				if gl, ok := g.(*node.Glyph); ok {
					glyphFont = gl.Font
					break
				}
			}
		}
	}
	if glyphFont == nil {
		t.Fatal("no glyph")
	}
	if pf := fe.paragraphFont(te); pf != glyphFont {
		t.Errorf("paragraph font %p, the glyphs' %p", pf, glyphFont)
	}
	if v := glyphFont.Face.VariationSettings; v["wght"] != 700 || v["wdth"] != 90 {
		t.Errorf("the glyphs' face pins %v", v)
	}
}
