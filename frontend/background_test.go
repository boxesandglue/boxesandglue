package frontend

import (
	"io"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

// backgroundRules collects the inline background rules of a paragraph, line
// by line, in list order.
func backgroundRules(vl *node.VList) [][]*node.Rule {
	var lines [][]*node.Rule
	for e := vl.List; e != nil; e = e.Next() {
		hl, ok := e.(*node.HList)
		if !ok {
			continue
		}
		var rules []*node.Rule
		for n := hl.List; n != nil; n = n.Next() {
			if r, ok := n.(*node.Rule); ok {
				if origin, _ := r.GetAttribute("origin"); origin == "inline background" {
					rules = append(rules, r)
				}
			}
		}
		lines = append(lines, rules)
	}
	return lines
}

func countRules(lines [][]*node.Rule) int {
	n := 0
	for _, l := range lines {
		n += len(l)
	}
	return n
}

// TestInlineBackground checks that a child Text with its own background
// colour gets a filled box behind its glyphs, one per line it spans, while
// the paragraph's own background colour stays a block property.
func TestInlineBackground(t *testing.T) {
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
	yellow := fe.GetColor("yellow")
	red := fe.GetColor("red")

	format := func(t *testing.T, te *Text, width string) *node.VList {
		t.Helper()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		vl, _, err := fe.FormatParagraph(te, bag.MustSP(width))
		if err != nil {
			t.Fatal(err)
		}
		return vl
	}

	t.Run("one line, one box", func(t *testing.T) {
		span := NewText()
		span.Settings[SettingBackgroundColor] = yellow
		span.Items = append(span.Items, "marked")
		te := NewText()
		te.Items = append(te.Items, "before ", span, " after")
		lines := backgroundRules(format(t, te, "200pt"))
		if got := countRules(lines); got != 1 {
			t.Fatalf("got %d background rules, want 1", got)
		}
		r := lines[0][0]
		if !r.Hide || !strings.Contains(r.Pre, " re f") {
			t.Errorf("rule is not a hidden filled rectangle: %q", r.Pre)
		}
		if !strings.Contains(r.Pre, yellow.PDFStringNonStroking()) {
			t.Errorf("rule does not set the span's colour: %q", r.Pre)
		}
		// The box must precede the span's glyphs so the text is painted on
		// top of it.
		for n := r.Prev(); n != nil; n = n.Prev() {
			if g, ok := n.(*node.Glyph); ok && g.Components == "m" {
				t.Error("background rule sits after the first glyph of the span")
			}
		}
	})

	t.Run("a wrapped span gets one box per line", func(t *testing.T) {
		span := NewText()
		span.Settings[SettingBackgroundColor] = yellow
		span.Items = append(span.Items, "one two three four five six seven eight nine ten")
		te := NewText()
		te.Items = append(te.Items, span)
		lines := backgroundRules(format(t, te, "60pt"))
		if len(lines) < 2 {
			t.Fatalf("expected the span to wrap, got %d line(s)", len(lines))
		}
		for i, l := range lines {
			if len(l) != 1 {
				t.Errorf("line %d: got %d background rules, want 1", i, len(l))
			}
		}
	})

	t.Run("the paragraph's own colour is not painted inline", func(t *testing.T) {
		te := NewText()
		te.Settings[SettingBackgroundColor] = yellow
		te.Items = append(te.Items, "block level text")
		if got := countRules(backgroundRules(format(t, te, "200pt"))); got != 0 {
			t.Errorf("got %d background rules, want none for a block colour", got)
		}
	})

	t.Run("a child repeating the parent's colour paints nothing", func(t *testing.T) {
		span := NewText()
		span.Settings[SettingBackgroundColor] = "yellow"
		span.Items = append(span.Items, "same")
		te := NewText()
		te.Settings[SettingBackgroundColor] = yellow
		te.Items = append(te.Items, "block ", span)
		if got := countRules(backgroundRules(format(t, te, "200pt"))); got != 0 {
			t.Errorf("got %d background rules, want none", got)
		}
	})

	t.Run("nested spans paint the inner box on top", func(t *testing.T) {
		inner := NewText()
		inner.Settings[SettingBackgroundColor] = red
		inner.Items = append(inner.Items, "inner")
		outer := NewText()
		outer.Settings[SettingBackgroundColor] = yellow
		outer.Items = append(outer.Items, "outer ", inner, " outer")
		te := NewText()
		te.Items = append(te.Items, outer)
		lines := backgroundRules(format(t, te, "200pt"))
		if len(lines) != 1 || len(lines[0]) != 2 {
			t.Fatalf("got %d lines / %d rules, want one line with two rules", len(lines), countRules(lines))
		}
		if !strings.Contains(lines[0][0].Pre, yellow.PDFStringNonStroking()) {
			t.Errorf("first painted rule should be the outer (yellow) box: %q", lines[0][0].Pre)
		}
		if !strings.Contains(lines[0][1].Pre, red.PDFStringNonStroking()) {
			t.Errorf("second painted rule should be the inner (red) box: %q", lines[0][1].Pre)
		}
	})

	t.Run("re-formatting the same Text paints once per pass", func(t *testing.T) {
		// Table layout formats a cell's Text several times; the inherited
		// settings that Mknodes copies into the child must not turn the
		// parent's colour into a child colour on the second pass.
		span := NewText()
		span.Settings[SettingBackgroundColor] = yellow
		span.Items = append(span.Items, "marked")
		te := NewText()
		te.Items = append(te.Items, "before ", span)
		format(t, te, "200pt")
		if got := countRules(backgroundRules(format(t, te, "200pt"))); got != 1 {
			t.Errorf("second pass: got %d background rules, want 1", got)
		}
	})
}

// TestDecorationDrawnOncePerLine guards postLinebreak against walking the
// lines more than once: the decoration pass used to be started at every line
// and followed the Next chain to the end, so the underline of line n was
// drawn n times on top of itself.
func TestDecorationDrawnOncePerLine(t *testing.T) {
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
	last := NewText()
	last.Settings[SettingTextDecorationLine] = TextDecorationUnderline
	last.Items = append(last.Items, "end")
	te := NewText()
	te.Settings[SettingFontFamily] = ff
	te.Settings[SettingSize] = bag.MustSP("10pt")
	te.Items = append(te.Items, "one two three four five six seven eight nine ten ", last)
	vl, _, err := fe.FormatParagraph(te, bag.MustSP("60pt"))
	if err != nil {
		t.Fatal(err)
	}
	nLines, nRules := 0, 0
	for e := vl.List; e != nil; e = e.Next() {
		hl, ok := e.(*node.HList)
		if !ok {
			continue
		}
		nLines++
		for n := hl.List; n != nil; n = n.Next() {
			if r, ok := n.(*node.Rule); ok && r.Hide && strings.Contains(r.Pre, " S") {
				nRules++
			}
		}
	}
	if nLines < 3 {
		t.Fatalf("expected at least three lines, got %d", nLines)
	}
	if nRules != 1 {
		t.Errorf("got %d decoration rules, want exactly 1", nRules)
	}
}
