package frontend

import (
	"bytes"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/document"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/lang"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/speedata/hyphenation"
)

func TestLoadLang(t *testing.T) {
	testdata := []struct {
		req  string
		want string
	}{
		{"en", "en"},
		{"en_US", "en_US"},
		{"de", "de"},
		{"de_DE", "de_DE"},
	}

	for _, tc := range testdata {
		l, err := GetLanguage(tc.req)
		if err != nil {
			t.Error(err)
		}
		if l == nil {
			t.Errorf("d.GetLanguage(%q) = nil, want lang", tc.req)
		}
		if l.Name != tc.want {
			t.Errorf("d.GetLanguage(%q) Name = %q, want %q", tc.req, l.Name, tc.want)
		}
	}
}

// TestNoOpLanguage exercises the CSS Text 3 §6 contract: tags without TeX
// hyphenation patterns must resolve without error and never produce break
// points. Arabic, Hebrew, CJK, and any custom non-listed tag fall in here.
func TestNoOpLanguage(t *testing.T) {
	testdata := []string{"ar", "he", "ja", "zh", "made-up-tag", "x-test"}
	for _, tag := range testdata {
		l, err := GetLanguage(tag)
		if err != nil {
			t.Fatalf("GetLanguage(%q) returned error: %v", tag, err)
		}
		if l == nil {
			t.Fatalf("GetLanguage(%q) returned nil", tag)
		}
		if l.Name != tag {
			t.Errorf("GetLanguage(%q).Name = %q, want %q", tag, l.Name, tag)
		}
		// A typical word in a Latin-pattern language would have several
		// breakpoints; with a no-op hyphenator we expect the empty slice.
		if pos := l.Hyphenate("supercalifragilistic"); len(pos) != 0 {
			t.Errorf("GetLanguage(%q).Hyphenate produced %d breakpoints, want 0",
				tag, len(pos))
		}
	}
}

// TestIsHyphenationSupported documents which tags qualify as "auto-hyphenable"
// without falling through to the no-op sentinel.
func TestIsHyphenationSupported(t *testing.T) {
	for _, tag := range []string{"en", "EN", "en_US", "de", "de_DE"} {
		if !IsHyphenationSupported(tag) {
			t.Errorf("IsHyphenationSupported(%q) = false, want true", tag)
		}
	}
	for _, tag := range []string{"ar", "he", "ja", "zh", "klingon", ""} {
		if IsHyphenationSupported(tag) {
			t.Errorf("IsHyphenationSupported(%q) = true, want false", tag)
		}
	}
}

func TestHyphenate(t *testing.T) {
	str := ` computer.`
	var head, cur node.Node
	for _, r := range str {
		if r == ' ' {
			n := node.NewGlue()
			n.Width = 5 * bag.Factor
			head = node.InsertAfter(head, cur, n)
			cur = n
		} else {
			n := node.NewGlyph()
			n.Hyphenate = unicode.IsLetter(r)
			n.Codepoint = 123
			n.Components = string(r)
			n.Width = 5 * bag.Factor
			head = node.InsertAfter(head, cur, n)
			cur = n
		}
	}

	var dummy bytes.Buffer
	doc := document.NewDocument(&dummy)
	l, err := doc.LoadPatternFile(filepath.Join("testdata", "hyph-en-us.pat.txt"), "dummylang")
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	Hyphenate(head, l)
	data := []node.Type{
		node.TypeGlue, node.TypeGlyph, node.TypeGlyph, node.TypeGlyph, node.TypeDisc, node.TypeGlyph, node.TypeGlyph, node.TypeGlyph, node.TypeGlyph, node.TypeGlyph,
	}

	for i, nt := range data {
		if want, got := nt, head.Type(); want != got {
			t.Errorf("head.Type() = %d, want %d (pos %d)", got, want, i)
		}
		head = head.Next()
	}
}

// A word that spans two runs with different scales gets a hyphen in the
// scale of the glyph before the break, the one the hyphen follows on the
// line. A kern between the two glyphs does not count as the glyph.
func TestHyphenTakesScaleOfGlyphBeforeBreak(t *testing.T) {
	for _, tc := range []struct{ first, second, want float64 }{
		{0.8, 0, 0.8},
		{0, 1.2, 0},
	} {
		var head, cur node.Node
		for i, r := range "computer" {
			n := node.NewGlyph()
			n.Hyphenate = true
			n.Codepoint = 123
			n.Components = string(r)
			n.Width = 5 * bag.Factor
			n.HorizontalScale = tc.first
			if i >= 3 {
				n.HorizontalScale = tc.second
			}
			head = node.InsertAfter(head, cur, n)
			cur = n
			if i == 2 {
				k := node.NewKern()
				k.Kern = bag.Factor
				head = node.InsertAfter(head, cur, k)
				cur = k
			}
		}
		var dummy bytes.Buffer
		doc := document.NewDocument(&dummy)
		l, err := doc.LoadPatternFile(filepath.Join("testdata", "hyph-en-us.pat.txt"), "dummylang")
		if err != nil {
			t.Fatal(err)
		}
		Hyphenate(head, l)
		var disc *node.Disc
		for n := head; n != nil; n = n.Next() {
			if d, ok := n.(*node.Disc); ok {
				disc = d
				break
			}
		}
		if disc == nil {
			t.Fatal("no hyphenation point in computer")
		}
		if g, ok := disc.Pre.(*node.Glyph); !ok || g.HorizontalScale != tc.want {
			t.Errorf("com|puter at %g|%g: hyphen %v, want scale %g", tc.first, tc.second, disc.Pre, tc.want)
		}
	}
}

// hyphenatedBy is what hyphenation's own minimums give for word, as the
// distances Lang.Hyphenate returns.
func hyphenatedBy(t *testing.T, patterns string, left, right int, word string) []int {
	t.Helper()
	hl, err := hyphenation.New(strings.NewReader(patterns))
	if err != nil {
		t.Fatal(err)
	}
	hl.Leftmin, hl.Rightmin = left, right
	points := hl.Hyphenate(word)
	for i := len(points) - 1; i > 0; i-- {
		points[i] -= points[i-1]
	}
	return points
}

var minimumWords = []string{"a", "an", "computer", "developers", "hyphenation", "extraordinary", "supercalifragilistic", "naïveté"}

func TestHyphenateBreaksAsThePatternsOwnMinimumsDid(t *testing.T) {
	for _, m := range []struct{ left, right int }{{-1, -1}, {0, 0}, {1, 1}, {2, 3}, {3, 2}, {4, 4}, {10, 10}} {
		l, err := GetLanguage("en")
		if err != nil {
			t.Fatal(err)
		}
		l.Lefthyphenmin, l.Righthyphenmin = m.left, m.right
		for _, word := range minimumWords {
			want := hyphenatedBy(t, hyphenationpatterns["en"], m.left, m.right, word)
			if got := l.Hyphenate(word); !slices.Equal(got, want) {
				t.Errorf("%q at minimums %d and %d: %v, want %v", word, m.left, m.right, got, want)
			}
		}
	}
}

// Two documents' languages share their patterns, and each hyphenates by its
// own minimums even while the other does.
func TestTwoLanguagesHyphenateAtOnceByTheirOwnMinimums(t *testing.T) {
	var langs [2]*lang.Lang
	for i, m := range []int{1, 5} {
		l, err := GetLanguage("en")
		if err != nil {
			t.Fatal(err)
		}
		l.Lefthyphenmin, l.Righthyphenmin = m, m
		langs[i] = l
	}
	var wants [2][][]int
	for i, m := range []int{1, 5} {
		for _, word := range minimumWords {
			wants[i] = append(wants[i], hyphenatedBy(t, hyphenationpatterns["en"], m, m, word))
		}
	}
	var wg sync.WaitGroup
	for i := range langs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				for k, word := range minimumWords {
					if got := langs[i].Hyphenate(word); !slices.Equal(got, wants[i][k]) {
						t.Errorf("%q at minimums %d: %v, want %v", word, langs[i].Lefthyphenmin, got, wants[i][k])
						return
					}
				}
			}
		}()
	}
	wg.Wait()
}

func TestGetLanguageParsesEachPatternSetOnce(t *testing.T) {
	for _, tag := range []string{"en", "de_DE", "made-up-tag"} {
		if _, err := GetLanguage(tag); err != nil {
			t.Fatal(err)
		}
		if allocs := testing.AllocsPerRun(10, func() { _, _ = GetLanguage(tag) }); allocs > 10 {
			t.Errorf("GetLanguage(%q) again made %.0f allocations, want at most 10: its patterns were parsed again", tag, allocs)
		}
	}
}

// A language from GetLanguage hyphenates with a word cache of its own, so the
// words one document hyphenates are not kept for every later one.
func TestEachLanguageKeepsAWordCacheOfItsOwn(t *testing.T) {
	const word = "hyphenation"
	warm, err := GetLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	warm.Hyphenate(word)
	cached := testing.AllocsPerRun(10, func() { warm.Hyphenate(word) })
	getting := testing.AllocsPerRun(10, func() { _, _ = GetLanguage("en") })
	fresh := testing.AllocsPerRun(10, func() {
		l, _ := GetLanguage("en")
		l.Hyphenate(word)
	})
	if fresh-getting <= cached {
		t.Errorf("a new language hyphenated %q with %.0f allocations, as a language that had hyphenated it already does (%.0f): it found the word in another's cache", word, fresh-getting, cached)
	}
}

// hyphenatedGlyphs hyphenates a word whose glyphs set the given characters,
// several for a ligature, and writes it back with a ligature in brackets
// and a hyphenation point as "-".
func hyphenatedGlyphs(t *testing.T, l *lang.Lang, glyphs []string) string {
	t.Helper()
	var head, cur node.Node
	for _, c := range glyphs {
		g := node.NewGlyph()
		g.Hyphenate = true
		g.Components = c
		g.Width = 5 * bag.Factor
		head = node.InsertAfter(head, cur, g)
		cur = g
	}
	Hyphenate(head, l)
	var b strings.Builder
	for n := head; n != nil; n = n.Next() {
		switch v := n.(type) {
		case *node.Glyph:
			if utf8.RuneCountInString(v.Components) > 1 {
				b.WriteString("[" + v.Components + "]")
			} else {
				b.WriteString(v.Components)
			}
		case *node.Disc:
			b.WriteString("-")
		}
	}
	return b.String()
}

// splitLigatures splits a word into one glyph per character, except for the
// ligatures in brackets.
func splitLigatures(word string) []string {
	var glyphs []string
	for len(word) > 0 {
		if word[0] == '[' {
			end := strings.IndexByte(word, ']')
			glyphs = append(glyphs, word[1:end])
			word = word[end+1:]
			continue
		}
		r, size := utf8.DecodeRuneInString(word)
		glyphs = append(glyphs, string(r))
		word = word[size:]
	}
	return glyphs
}

// A hyphenation point after a ligature counts the ligature's characters, so
// it does not move to the right with every ligature before it (bag#89). A
// point inside a ligature is left out when the glyphs have no font to shape
// its parts with.
func TestHyphenateAfterLigature(t *testing.T) {
	l, err := GetLanguage("de")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ word, want string }{
		{"Schi[fff]ahrt", "Schi[fff]ahrt"},
		{"Schi[ff]fahrt", "Schi[ff]-fahrt"},
		{"beschä[ft]igt", "beschä[ft]igt"},
		{"Donaudampfschi[fff]ahrtsgesellscha[ft]", "Donau-dampf-schi[fff]ahrts-ge-sell-scha[ft]"},
		{"Donaudampfschi[ff]fahrtsgesellscha[ft]", "Donau-dampf-schi[ff]-fahrts-ge-sell-scha[ft]"},
	} {
		if got := hyphenatedGlyphs(t, l, splitLigatures(tc.word)); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.word, got, tc.want)
		}
	}
}

// shapedWord sets word in TeX Gyre Heros, which has ff, fi, fl and ffi
// ligatures, as glyph nodes that may be hyphenated.
func shapedWord(t *testing.T, word string) (node.Node, *font.Font) {
	t.Helper()
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	face, err := fe.Doc.LoadFace("../qa/fonts/upem/fonts/texgyreheros-regular.otf", 0)
	if err != nil {
		t.Fatal(err)
	}
	fnt := font.NewFont(face, 10*bag.Factor)
	var head, cur node.Node
	for _, a := range fnt.Shape(word, nil, nil) {
		g := node.NewGlyph()
		g.Font = fnt
		g.Hyphenate = true
		g.Codepoint = a.Codepoint
		g.Components = a.Components
		g.Width = a.Advance
		head = node.InsertAfter(head, cur, g)
		cur = g
		if a.Kernafter != 0 {
			k := node.NewKern()
			k.Kern = a.Kernafter
			head = node.InsertAfter(head, cur, k)
			cur = k
		}
	}
	// The word does not start the list, as in a paragraph after its indent.
	start := node.NewGlue()
	head = node.InsertBefore(head, head, start)
	return head, fnt
}

// glyphString writes a glyph, a ligature in brackets.
func glyphString(g *node.Glyph) string {
	if utf8.RuneCountInString(g.Components) > 1 {
		return "[" + g.Components + "]"
	}
	return g.Components
}

// glyphText writes the glyphs of a list.
func glyphText(n node.Node) string {
	var b strings.Builder
	for ; n != nil; n = n.Next() {
		if g, ok := n.(*node.Glyph); ok {
			b.WriteString(glyphString(g))
		}
	}
	return b.String()
}

// A hyphenation point inside a ligature gets a Disc that sets the ligature
// apart if the word breaks there: the line ends with the characters before
// the point and a hyphen, the next starts with the rest, shaped anew, and a
// break drops the ligature. A point right after a ligature gets a plain
// hyphen (bag#90).
func TestHyphenateInsideLigature(t *testing.T) {
	l, err := GetLanguage("de")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ word, want string }{
		{"Auflage", "Au{f-|l}[fl]age"},
		{"Kaffeefilter", "Ka{f-|f}[ff]ee-[fi]l-ter"},
		{"offiziell", "o[ffi]-zi-ell"},
		{"Schifffahrt", "Schi[ff]-fahrt"},
	} {
		head, _ := shapedWord(t, tc.word)
		Hyphenate(head, l)
		var b strings.Builder
		for n := head; n != nil; n = n.Next() {
			switch v := n.(type) {
			case *node.Glyph:
				b.WriteString(glyphString(v))
			case *node.Disc:
				if v.ReplaceCount == 0 {
					b.WriteString("-")
				} else {
					b.WriteString("{" + glyphText(v.Pre) + "|" + glyphText(v.Post) + "}")
				}
			}
		}
		if got := strings.ReplaceAll(b.String(), "‐", "-"); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.word, got, tc.want)
		}
	}
}

// A paragraph that breaks inside the fl ligature of Auflage ends its first
// line with Auf- and starts the next with l, and neither line has the
// ligature (bag#90).
func TestLinebreakInsideLigature(t *testing.T) {
	l, err := GetLanguage("de")
	if err != nil {
		t.Fatal(err)
	}
	head, fnt := shapedWord(t, "Auflage")
	Hyphenate(head, l)
	node.AppendLineEndAfter(head, node.Tail(head))
	width := func(s string) (w bag.ScaledPoint) {
		for _, a := range fnt.Shape(s, nil, nil) {
			w += a.Advance + a.Kernafter
		}
		return w
	}
	s := node.NewLinebreakSettings()
	s.HSize = max(width("Auf")+fnt.Hyphenchar.Advance, width("lage")) + bag.Factor/10
	s.LineHeight = 12 * bag.Factor
	s.LineEndGlue.Stretch = bag.Factor
	s.LineEndGlue.StretchOrder = node.StretchFil
	vl, _ := node.Linebreak(head, s)
	var lines []string
	for n := vl.List; n != nil; n = n.Next() {
		if hl, ok := n.(*node.HList); ok {
			lines = append(lines, strings.ReplaceAll(glyphText(hl.List), "‐", "-"))
		}
	}
	if got, want := strings.Join(lines, "|"), "Auf-|lage"; got != want {
		t.Errorf("lines %q, want %q", got, want)
	}
}
