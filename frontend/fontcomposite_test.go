package frontend

import (
	"io"
	"testing"
)

// latinRange and latinExtRange are the gist of the latin and latin-ext
// subsets Google Fonts splits a family into.
var (
	latinRange    = []CodepointRange{{0x0000, 0x00FF}}
	latinExtRange = []CodepointRange{{0x0100, 0x024F}}
)

func newCompositeDoc(t *testing.T) *Document {
	t.Helper()
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatalf("NewForWriter: %v", err)
	}
	return fe
}

func TestFontSourceCovers(t *testing.T) {
	all := &FontSource{}
	if !all.Covers('a') || !all.Covers(0x10FFFF) {
		t.Error("a source without a range must cover every code point")
	}
	fs := &FontSource{UnicodeRange: []CodepointRange{{0x41, 0x5A}, {0x2116, 0x2116}}}
	for r, want := range map[rune]bool{'A': true, 'Z': true, 'a': false, 0x2116: true, 0x2117: false} {
		if got := fs.Covers(r); got != want {
			t.Errorf("Covers(%U) = %v, want %v", r, got, want)
		}
	}
}

// TestCompositeFaceSplitsByRange is the Google Fonts case: the latin and the
// latin-ext subset of one family share weight and style, and ő must come
// from latin-ext while the rest stays in latin.
func TestCompositeFaceSplitsByRange(t *testing.T) {
	fe := newCompositeDoc(t)
	ext := &FontSource{Name: "latin-ext", UnicodeRange: latinExtRange}
	latin := &FontSource{Name: "latin", UnicodeRange: latinRange}
	ff := fe.NewFontFamily("Roboto")
	ff.AddMember(ext, FontWeight400, FontStyleNormal)
	ff.AddMember(latin, FontWeight400, FontStyleNormal)
	seedCoverage(fe, latin, "Tschüß ", true)
	seedCoverage(fe, ext, "ő", true)

	stack := []*FontFamily{ff}
	if !needsCoverage(stack, FontWeight400, FontStyleNormal, synthesisSetting{}) {
		t.Fatal("a composite face must take the coverage path")
	}
	runs := fe.coverageSegments("Tschüß ő", stack, FontWeight400, FontStyleNormal, synthesisSetting{})
	want := []struct {
		text string
		src  *FontSource
	}{{"Tschüß ", latin}, {"ő", ext}}
	if len(runs) != len(want) {
		t.Fatalf("got %d runs, want %d: %#v", len(runs), len(want), runs)
	}
	for i, r := range runs {
		if r.Text != want[i].text || r.Source != want[i].src || r.StackIndex != 0 {
			t.Errorf("run %d = %q from %s (stack %d), want %q from %s", i, r.Text, r.Source.Name, r.StackIndex, want[i].text, want[i].src.Name)
		}
	}
}

// TestCompositeFaceSkipsUnusedSubset checks that a source no character falls
// into is never probed, so its face is never loaded: the subsets of a
// Google Fonts family the text does not use cost nothing.
func TestCompositeFaceSkipsUnusedSubset(t *testing.T) {
	fe := newCompositeDoc(t)
	cyrillic := &FontSource{Name: "cyrillic", Location: "/does/not/exist.woff2", UnicodeRange: []CodepointRange{{0x0400, 0x045F}}}
	latin := &FontSource{Name: "latin", UnicodeRange: latinRange}
	ff := fe.NewFontFamily("Roboto")
	ff.AddMember(cyrillic, FontWeight400, FontStyleNormal)
	ff.AddMember(latin, FontWeight400, FontStyleNormal)
	seedCoverage(fe, latin, "abc", true)

	fe.coverageSegments("abc", []*FontFamily{ff}, FontWeight400, FontStyleNormal, synthesisSetting{})
	if _, probed := fe.coverageCache.entries[cyrillic]; probed {
		t.Error("the cyrillic subset was probed although no character lies in its range")
	}
	if cyrillic.face != nil {
		t.Error("the cyrillic subset was loaded")
	}
}

// TestCompositeFaceFallsThroughMissingGlyph is the Font Awesome v4 case: the
// faces without a range cover everything, and a character the last added one
// lacks comes from the one before it.
func TestCompositeFaceFallsThroughMissingGlyph(t *testing.T) {
	fe := newCompositeDoc(t)
	solid := &FontSource{Name: "solid"}
	brands := &FontSource{Name: "brands"}
	compat := &FontSource{Name: "v4compatibility", UnicodeRange: []CodepointRange{{0xF041, 0xF041}}}
	ff := fe.NewFontFamily("FontAwesome")
	ff.AddMember(solid, FontWeight400, FontStyleNormal)
	ff.AddMember(brands, FontWeight400, FontStyleNormal)
	ff.AddMember(compat, FontWeight400, FontStyleNormal)
	// U+F041 is in the compatibility file, U+F09B (github) in brands, U+F004
	// (heart) only in solid.
	seedCoverage(fe, compat, "", true)
	seedCoverage(fe, brands, " ", true)
	seedCoverage(fe, brands, "", false)
	seedCoverage(fe, solid, " ", true)

	if fs, err := ff.GetFontSource(FontWeight400, FontStyleNormal); err != nil || fs != brands {
		t.Errorf("first available font = %v, want brands: the compatibility file's range lacks the space", fs)
	}
	runs := fe.coverageSegments("", []*FontFamily{ff}, FontWeight400, FontStyleNormal, synthesisSetting{})
	want := []*FontSource{compat, brands, solid}
	if len(runs) != len(want) {
		t.Fatalf("got %d runs, want %d: %#v", len(runs), len(want), runs)
	}
	for i, r := range runs {
		if r.Source != want[i] {
			t.Errorf("run %d %q from %s, want %s", i, r.Text, r.Source.Name, want[i].Name)
		}
	}
}

// TestCompositeFaceBeforeNextFamily checks that every source of a family's
// composite face is tried before the next family of the stack, and that a
// character outside every range of the family moves on to the next family
// even if a source has the glyph.
func TestCompositeFaceBeforeNextFamily(t *testing.T) {
	fe := newCompositeDoc(t)
	ext := &FontSource{Name: "latin-ext", UnicodeRange: latinExtRange}
	latin := &FontSource{Name: "latin", UnicodeRange: latinRange}
	fallback := &FontSource{Name: "fallback"}
	ff := fe.NewFontFamily("Roboto")
	ff.AddMember(ext, FontWeight400, FontStyleNormal)
	ff.AddMember(latin, FontWeight400, FontStyleNormal)
	fb := seedFamily(fe, "fallback", fallback)
	seedCoverage(fe, latin, "a", true)
	seedCoverage(fe, ext, "ő", true)
	// latin has a glyph for α, but α is outside its range.
	seedCoverage(fe, latin, "α", true)
	seedCoverage(fe, fallback, "aőα", true)

	runs := fe.coverageSegments("aőα", []*FontFamily{ff, fb}, FontWeight400, FontStyleNormal, synthesisSetting{})
	want := []struct {
		src *FontSource
		idx int
	}{{latin, 0}, {ext, 0}, {fallback, 1}}
	if len(runs) != len(want) {
		t.Fatalf("got %d runs, want %d: %#v", len(runs), len(want), runs)
	}
	for i, r := range runs {
		if r.Source != want[i].src || r.StackIndex != want[i].idx {
			t.Errorf("run %d %q from %s (stack %d), want %s (stack %d)", i, r.Text, r.Source.Name, r.StackIndex, want[i].src.Name, want[i].idx)
		}
	}
}

// TestSingleSourceSkipsCoverage keeps the single-shape path for a family
// with one source, even one with a range.
func TestSingleSourceSkipsCoverage(t *testing.T) {
	fe := newCompositeDoc(t)
	ff := seedFamily(fe, "latin", &FontSource{Name: "latin", UnicodeRange: latinRange})
	if needsCoverage([]*FontFamily{ff}, FontWeight400, FontStyleNormal, synthesisSetting{}) {
		t.Error("a single source must not take the coverage path")
	}
	if needsCoverage(nil, FontWeight400, FontStyleNormal, synthesisSetting{}) {
		t.Error("an empty stack must not take the coverage path")
	}
}

// TestCompositeRangeMembers is Google's CSS for a variable font, which
// combines a weight range with unicode-range: the instances at one weight
// form a composite face, the last added first.
func TestCompositeRangeMembers(t *testing.T) {
	fe := newCompositeDoc(t)
	ext := &FontSource{Name: "latin-ext", Location: "ext.woff2", UnicodeRange: latinExtRange}
	latin := &FontSource{Name: "latin", Location: "latin.woff2", UnicodeRange: latinRange}
	ff := fe.NewFontFamily("Inter")
	ff.AddMemberRange(ext, 100, 900, FontStyleNormal)
	ff.AddMemberRange(latin, 100, 900, FontStyleNormal)

	faces, err := ff.faces(FontWeight700, FontStyleNormal, synthesisSetting{})
	if err != nil {
		t.Fatal(err)
	}
	if len(faces) != 2 || faces[0].Location != "latin.woff2" || faces[1].Location != "ext.woff2" {
		t.Fatalf("faces = %v, want the latin then the latin-ext instance", faces)
	}
	for _, fs := range faces {
		if fs.VariationSettings["wght"] != 700 || len(fs.UnicodeRange) != 1 {
			t.Errorf("instance %s lost its weight or range: %v %v", fs.Location, fs.VariationSettings, fs.UnicodeRange)
		}
	}
	again, _ := ff.faces(FontWeight700, FontStyleNormal, synthesisSetting{})
	if again[0] != faces[0] || again[1] != faces[1] {
		t.Error("repeated lookups must return the same instances")
	}
}

// TestCompositeSyntheticItalic slants every source of the upright composite
// face for a missing italic, ranges kept.
func TestCompositeSyntheticItalic(t *testing.T) {
	fe := newCompositeDoc(t)
	ext := &FontSource{Name: "latin-ext", UnicodeRange: latinExtRange}
	latin := &FontSource{Name: "latin", UnicodeRange: latinRange}
	ff := fe.NewFontFamily("Roboto")
	ff.AddMember(ext, FontWeight400, FontStyleNormal)
	ff.AddMember(latin, FontWeight400, FontStyleNormal)

	on := synthesisSetting{on: true, set: true}
	faces, err := ff.faces(FontWeight400, FontStyleItalic, on)
	if err != nil {
		t.Fatal(err)
	}
	if len(faces) != 2 || faces[0].upright != latin || faces[1].upright != ext {
		t.Fatalf("faces = %v, want slanted latin then latin-ext", faces)
	}
	for _, fs := range faces {
		if fs.Slant != SyntheticSlant || len(fs.UnicodeRange) != 1 {
			t.Errorf("synthetic %s: slant %v range %v", fs.Name, fs.Slant, fs.UnicodeRange)
		}
	}
	again, _ := ff.faces(FontWeight400, FontStyleItalic, on)
	if again[0] != faces[0] || again[1] != faces[1] {
		t.Error("repeated lookups must return the same synthetic sources")
	}
}
