package frontend

import (
	"fmt"
	"io"
	"math"
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

	t.Run("a span of spaces alone gets its font's box", func(t *testing.T) {
		// <b style="background:yellow">one</b><span style="background:yellow"> </span>...
		te := NewText()
		for _, s := range []string{"one", " ", "two"} {
			span := NewText()
			span.Settings[SettingBackgroundColor] = yellow
			span.Items = append(span.Items, s)
			te.Items = append(te.Items, span)
		}
		lines := backgroundRules(format(t, te, "200pt"))
		if len(lines) != 1 || len(lines[0]) != 3 {
			t.Fatalf("got %d lines / %d rules, want one line with three rules", len(lines), countRules(lines))
		}
		// "x y w h re f": the space's box is as tall as its neighbours'.
		box := func(r *node.Rule) string {
			f := strings.Fields(r.Pre)
			for i, w := range f {
				if w == "re" && i >= 4 {
					return f[i-3] + " " + f[i-1]
				}
			}
			return r.Pre
		}
		if got, want := box(lines[0][1]), box(lines[0][0]); got != want {
			t.Errorf("space box y/height = %s, want %s as for the glyphs", got, want)
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

	t.Run("spaces at a paragraph's edge inside a span are stripped", func(t *testing.T) {
		// issue #55: spaces at paragraph edge should be stripped even inside spans with background
		span := func(s string) *Text {
			x := NewText()
			x.Settings[SettingBackgroundColor] = yellow
			x.Items = append(x.Items, s)
			return x
		}

		// Baseline: span("one") without spaces
		teWord := NewText()
		teWord.Items = append(teWord.Items, span("one"))
		vlWord := format(t, teWord, "200pt")
		rulesWord := backgroundRules(vlWord)
		if len(rulesWord) != 1 || len(rulesWord[0]) != 1 {
			t.Fatalf("span('one') got %d rules, want 1", countRules(rulesWord))
		}
		wantWidth := rulesWord[0][0].Width

		// Case 1: span(" one ") has leading and trailing spaces stripped
		te1 := NewText()
		te1.Items = append(te1.Items, span(" one "))
		vl1 := format(t, te1, "200pt")
		rules1 := backgroundRules(vl1)
		if len(rules1) != 1 || len(rules1[0]) != 1 {
			t.Fatalf("span(' one ') got %d rules, want 1", countRules(rules1))
		}
		if got := rules1[0][0].Width; got != wantWidth {
			t.Errorf("span(' one ') rule width = %v, want %v (same as span('one'))", got, wantWidth)
		}

		// Case 2: span("one"), span(" ") at paragraph end (trailing space in span stripped)
		teEnd := NewText()
		teEnd.Items = append(teEnd.Items, span("one"), span(" "))
		vlEnd := format(t, teEnd, "200pt")
		rulesEnd := backgroundRules(vlEnd)
		if len(rulesEnd) != 1 || len(rulesEnd[0]) != 1 {
			t.Fatalf("span('one'), span(' ') got %d rules, want 1", countRules(rulesEnd))
		}
		if got := rulesEnd[0][0].Width; got != wantWidth {
			t.Errorf("span('one'), span(' ') rule width = %v, want %v", got, wantWidth)
		}

		// Case 3: span(" "), span("one") at paragraph start (leading space in span stripped)
		teStart := NewText()
		teStart.Items = append(teStart.Items, span(" "), span("one"))
		vlStart := format(t, teStart, "200pt")
		rulesStart := backgroundRules(vlStart)
		if len(rulesStart) != 1 || len(rulesStart[0]) != 1 {
			t.Fatalf("span(' '), span('one') got %d rules, want 1", countRules(rulesStart))
		}
		if got := rulesStart[0][0].Width; got != wantWidth {
			t.Errorf("span(' '), span('one') rule width = %v, want %v", got, wantWidth)
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

// TestInlineBackgroundArea checks SettingBackgroundArea: the em box by
// default, and with BackgroundAreaAscentDescent the font's content area, on
// the span or inherited from the paragraph.
func TestInlineBackgroundArea(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ff := fe.NewFontFamily("test")
	if err := ff.AddMember(&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"}, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	yellow := fe.GetColor("yellow")
	// box returns "y height" of the one background rule.
	box := func(t *testing.T, onSpan, onPara any) string {
		t.Helper()
		span := NewText()
		span.Settings[SettingBackgroundColor] = yellow
		if onSpan != nil {
			span.Settings[SettingBackgroundArea] = onSpan
		}
		span.Items = append(span.Items, "marked")
		te := NewText()
		te.Settings[SettingFontFamily] = ff
		te.Settings[SettingSize] = bag.MustSP("10pt")
		if onPara != nil {
			te.Settings[SettingBackgroundArea] = onPara
		}
		te.Items = append(te.Items, "before ", span, " after")
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("200pt"))
		if err != nil {
			t.Fatal(err)
		}
		lines := backgroundRules(vl)
		if countRules(lines) != 1 {
			t.Fatalf("got %d background rules, want 1", countRules(lines))
		}
		f := strings.Fields(lines[0][0].Pre)
		for i, w := range f {
			if w == "re" && i >= 4 {
				return f[i-3] + " " + f[i-1]
			}
		}
		return lines[0][0].Pre
	}
	// TeX Gyre Heros at 10pt: hhea ascender 11.48pt, descender 2.84pt; the em
	// box is 10pt with the font's depth below the baseline.
	emBox := box(t, nil, nil)
	if got := box(t, BackgroundAreaEmBox, nil); got != emBox {
		t.Errorf("BackgroundAreaEmBox gives %q, want the default %q", got, emBox)
	}
	for name, c := range map[string][2]any{
		"on the span":      {BackgroundAreaAscentDescent, nil},
		"on the paragraph": {nil, BackgroundAreaAscentDescent},
	} {
		t.Run(name, func(t *testing.T) {
			if got := box(t, c[0], c[1]); got != "-2.84 14.32" {
				t.Errorf("box y/height %q, want \"-2.84 14.32\" (em box %q)", got, emBox)
			}
		})
	}
}

// TestInlineBackgroundFromItsText checks that the box of an inline background
// comes from the font of the Text that carries it, as CSS takes an inline
// box's content area from its own font: nested text in another family or
// size, or nested vertical-align, does not change or move it.
func TestInlineBackgroundFromItsText(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	sans := fe.NewFontFamily("sans")
	if err := sans.AddMember(&FontSource{Location: "../qa/fonts/upem/fonts/texgyreheros-regular.otf"}, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	serif := fe.NewFontFamily("serif")
	if err := serif.AddMember(&FontSource{Location: "../qa/fonts/upem/fonts/CrimsonPro-Regular.ttf"}, FontWeight400, FontStyleNormal); err != nil {
		t.Fatal(err)
	}
	yellow := fe.GetColor("yellow")

	// boxes returns "y height" of each background rule on the one line.
	boxes := func(t *testing.T, span *Text) []string {
		t.Helper()
		span.Settings[SettingBackgroundColor] = yellow
		span.Settings[SettingBackgroundArea] = BackgroundAreaAscentDescent
		te := NewText()
		te.Settings[SettingFontFamily] = serif
		te.Settings[SettingSize] = bag.MustSP("10pt")
		te.Items = append(te.Items, "before ", span, " after")
		vl, _, err := fe.FormatParagraph(te, bag.MustSP("300pt"))
		if err != nil {
			t.Fatal(err)
		}
		lines := backgroundRules(vl)
		if len(lines) != 1 {
			t.Fatalf("got %d lines, want 1", len(lines))
		}
		var ret []string
		for _, r := range lines[0] {
			f := strings.Fields(r.Pre)
			for i, w := range f {
				if w == "re" && i >= 4 {
					ret = append(ret, f[i-3]+" "+f[i-1])
				}
			}
		}
		return ret
	}
	nested := func(settings map[SettingType]any) *Text {
		child := NewText()
		for k, v := range settings {
			child.Settings[k] = v
		}
		child.Items = append(child.Items, "nested")
		span := NewText()
		span.Items = append(span.Items, "serif ", child, " serif")
		return span
	}
	plain := NewText()
	plain.Items = append(plain.Items, "serif nested serif")
	want := boxes(t, plain)
	if len(want) != 1 {
		t.Fatalf("a span in one face gets %d boxes, want 1", len(want))
	}

	for name, settings := range map[string]map[SettingType]any{
		"another family":          {SettingFontFamily: sans},
		"a larger size":           {SettingSize: bag.MustSP("14pt")},
		"a nested vertical-align": {SettingYOffset: bag.MustSP("3pt")},
	} {
		t.Run(name, func(t *testing.T) {
			got := boxes(t, nested(settings))
			if len(got) != 1 || got[0] != want[0] {
				t.Errorf("boxes %q, want the span's own box %q once", got, want[0])
			}
		})
	}

	t.Run("vertical-align on the span moves its box", func(t *testing.T) {
		span := NewText()
		span.Settings[SettingYOffset] = bag.MustSP("3pt")
		span.Items = append(span.Items, "serif nested serif")
		got := boxes(t, span)
		if len(got) != 1 || got[0] == want[0] {
			t.Fatalf("boxes %q, want one box moved from %q", got, want[0])
		}
		var y0, y1, h0, h1 float64
		fmt.Sscan(want[0], &y0, &h0)
		fmt.Sscan(got[0], &y1, &h1)
		if math.Abs(y1-y0-3) > 0.01 || h1 != h0 {
			t.Errorf("box %q, want %q raised by 3pt", got[0], want[0])
		}
	})
}
