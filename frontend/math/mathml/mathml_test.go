package mathml

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pdf "github.com/boxesandglue/baseline-pdf"
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/frontend/math"
	"github.com/boxesandglue/textshape/ot"
)

// loadMathFont mirrors the helper in the parent math package — re-implemented
// here because the parent helper is test-private. testdata is a sibling of
// our package; the relative path walks up one level.
func loadMathFont(t *testing.T) *font.Font {
	t.Helper()
	path := filepath.Join("..", "testdata", "latinmodern-math.otf")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("math font not available at %s — see ../testdata/README.md", path)
	}
	pw := pdf.NewPDFWriter(io.Discard)
	face, err := pw.LoadFace(path, 0)
	if err != nil {
		t.Fatalf("LoadFace(%s): %v", path, err)
	}
	return font.NewFont(face, bag.MustSP("10pt"))
}

// glyphFor resolves a single rune to its glyph id, same helper as
// engine_test.go uses — makes assertions human-readable.
func glyphFor(t *testing.T, fnt *font.Font, r rune) ot.GlyphID {
	t.Helper()
	atoms := fnt.Shape(string(r), nil, nil)
	if len(atoms) == 0 {
		t.Fatalf("Shape(%q) returned no atoms", string(r))
	}
	return ot.GlyphID(atoms[0].Codepoint)
}

// singleAtom is a one-liner for the common pattern "Parse must produce
// exactly one MathAtom of the given class with a glyph nucleus".
func singleAtom(t *testing.T, items []math.MathItem) *math.MathAtom {
	t.Helper()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	a, ok := items[0].(*math.MathAtom)
	if !ok {
		t.Fatalf("expected *MathAtom, got %T", items[0])
	}
	return a
}

// TestParseSingleIdentifierItalic — <mi>x</mi> defaults to italic (single-
// char rule); the reader must look up the glyph for U+1D465 (italic x), not
// plain ASCII x. Catches the math-italic gotcha from Phase 2.
func TestParseSingleIdentifierItalic(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, display, err := Parse([]byte(`<math><mi>x</mi></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if display {
		t.Error("display should be false (no display attr)")
	}
	a := singleAtom(t, atoms)
	if a.Class != math.ClassOrd {
		t.Errorf("class: got %v, want ClassOrd", a.Class)
	}
	want := glyphFor(t, fnt, '𝑥') // U+1D465
	if a.Nucleus.Glyph != want {
		t.Errorf("glyph: got %d, want %d (italic x U+1D465)", a.Nucleus.Glyph, want)
	}
}

// TestParseMultiCharIdentifierUpright — <mi>sin</mi> defaults to normal
// (multi-char rule), so the reader uses ASCII s/i/n, not math-italic.
func TestParseMultiCharIdentifierUpright(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><mi>sin</mi></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(atoms) != 3 {
		t.Fatalf("expected 3 atoms (s, i, n), got %d", len(atoms))
	}
	wantS := glyphFor(t, fnt, 's') // ASCII, not italic
	a := atoms[0].(*math.MathAtom)
	if a.Nucleus.Glyph != wantS {
		t.Errorf("first glyph: got %d, want %d (upright s)", a.Nucleus.Glyph, wantS)
	}
}

// TestParseMathVariantNormalOverride — explicit mathvariant="normal" beats
// the single-char default-italic rule.
func TestParseMathVariantNormalOverride(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><mi mathvariant="normal">x</mi></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	a := singleAtom(t, atoms)
	wantUpright := glyphFor(t, fnt, 'x')
	if a.Nucleus.Glyph != wantUpright {
		t.Errorf("glyph: got %d, want %d (upright x)", a.Nucleus.Glyph, wantUpright)
	}
}

// TestParseHmapsToPlanck — italic h is encoded as U+210E (Planck constant)
// because U+1D455 is reserved. The reader's special case must trigger.
func TestParseHmapsToPlanck(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><mi>h</mi></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	a := singleAtom(t, atoms)
	want := glyphFor(t, fnt, 'ℎ') // U+210E
	if a.Nucleus.Glyph != want {
		t.Errorf("h glyph: got %d, want %d (U+210E Planck h)", a.Nucleus.Glyph, want)
	}
}

// TestParseNumber — <mn>123</mn> emits one Ord per digit, all upright.
func TestParseNumber(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><mn>123</mn></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(atoms) != 3 {
		t.Fatalf("expected 3 atoms, got %d", len(atoms))
	}
	for i, want := range []rune{'1', '2', '3'} {
		a := atoms[i].(*math.MathAtom)
		if a.Class != math.ClassOrd {
			t.Errorf("atom[%d].Class: got %v, want ClassOrd", i, a.Class)
		}
		if got := a.Nucleus.Glyph; got != glyphFor(t, fnt, want) {
			t.Errorf("atom[%d].glyph: got %d, want glyph(%q)", i, got, string(want))
		}
	}
}

// TestParseOperatorClasses — the operator dictionary must map common
// operators to the right class so the spacing pass kerns them correctly.
func TestParseOperatorClasses(t *testing.T) {
	fnt := loadMathFont(t)
	cases := []struct {
		mathml string
		class  math.MathClass
		name   string
	}{
		{`<math><mo>+</mo></math>`, math.ClassBin, "plus"},
		{`<math><mo>=</mo></math>`, math.ClassRel, "equals"},
		{`<math><mo>(</mo></math>`, math.ClassOpen, "lparen"},
		{`<math><mo>)</mo></math>`, math.ClassClose, "rparen"},
		{`<math><mo>,</mo></math>`, math.ClassPunct, "comma"},
		{`<math><mo>∑</mo></math>`, math.ClassOp, "sum"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			atoms, _, err := Parse([]byte(tc.mathml), fnt)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			a := singleAtom(t, atoms)
			if a.Class != tc.class {
				t.Errorf("got class %v, want %v", a.Class, tc.class)
			}
		})
	}
}

// TestParseSuperscript — <msup> attaches its second child as Sup on the
// first; the base atom is reused (not wrapped in a sublist) when it's a
// single bare atom.
func TestParseSuperscript(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><msup><mi>x</mi><mn>2</mn></msup></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	a := singleAtom(t, atoms)
	if a.Nucleus.Glyph != glyphFor(t, fnt, '𝑥') {
		t.Errorf("base glyph: got %d, want italic x", a.Nucleus.Glyph)
	}
	if a.Sup.IsEmpty() {
		t.Fatal("Sup is empty")
	}
	if len(a.Sup.Sublist) != 1 {
		t.Fatalf("Sup.Sublist: got %d items, want 1", len(a.Sup.Sublist))
	}
	supAtom := a.Sup.Sublist[0].(*math.MathAtom)
	if supAtom.Nucleus.Glyph != glyphFor(t, fnt, '2') {
		t.Errorf("sup glyph: got %d, want 2", supAtom.Nucleus.Glyph)
	}
}

// TestParseSubsup — <msubsup> attaches both sub and sup on the same base.
func TestParseSubsup(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><msubsup><mi>x</mi><mn>0</mn><mn>2</mn></msubsup></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	a := singleAtom(t, atoms)
	if a.Sub.IsEmpty() || a.Sup.IsEmpty() {
		t.Errorf("expected both Sub and Sup set, got Sub=%v Sup=%v", a.Sub, a.Sup)
	}
}

// TestParseFraction — <mfrac> produces a *Fraction with the two slot lists.
func TestParseFraction(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><mfrac><mn>1</mn><mn>2</mn></mfrac></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 item, got %d", len(atoms))
	}
	f, ok := atoms[0].(*math.Fraction)
	if !ok {
		t.Fatalf("expected *Fraction, got %T", atoms[0])
	}
	if f.Thickness != 0 {
		t.Errorf("Thickness: got %d, want 0 (default rule)", f.Thickness)
	}
	if len(f.Num) != 1 || len(f.Den) != 1 {
		t.Errorf("num/den length: got %d/%d, want 1/1", len(f.Num), len(f.Den))
	}
}

// TestParseBinomial — linethickness="0" selects the no-rule variant
// (engine uses StackTopShiftUp etc. instead of FractionNumeratorShiftUp).
func TestParseBinomial(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><mfrac linethickness="0"><mi>n</mi><mi>k</mi></mfrac></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	f := atoms[0].(*math.Fraction)
	if f.Thickness != -1 {
		t.Errorf("Thickness: got %d, want -1 (no rule)", f.Thickness)
	}
}

// TestParseRadical — <msqrt> wraps its children as the radical body and
// resolves U+221A as the radical glyph.
func TestParseRadical(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><msqrt><mn>2</mn></msqrt></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r, ok := atoms[0].(*math.Radical)
	if !ok {
		t.Fatalf("expected *Radical, got %T", atoms[0])
	}
	if r.Glyph != glyphFor(t, fnt, '√') {
		t.Errorf("radical glyph: got %d, want √", r.Glyph)
	}
	if len(r.Body) != 1 {
		t.Errorf("body length: got %d, want 1", len(r.Body))
	}
}

// TestParseRootWithIndex — <mroot> takes (base, index) — note that the
// MathML order is base-first, unlike how one would write it in TeX.
func TestParseRootWithIndex(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><mroot><mi>x</mi><mn>3</mn></mroot></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := atoms[0].(*math.Radical)
	if len(r.Degree) != 1 {
		t.Errorf("degree length: got %d, want 1", len(r.Degree))
	}
	deg := r.Degree[0].(*math.MathAtom)
	if deg.Nucleus.Glyph != glyphFor(t, fnt, '3') {
		t.Errorf("degree glyph: got %d, want 3", deg.Nucleus.Glyph)
	}
}

// TestParseDisplayBlock — the display="block" attribute on <math> must be
// passed back to the caller so it can route to DisplayMath.
func TestParseDisplayBlock(t *testing.T) {
	fnt := loadMathFont(t)
	_, display, err := Parse([]byte(`<math display="block"><mn>1</mn></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !display {
		t.Error("display: got false, want true")
	}
}

// TestParseSemanticsTransparent — <semantics> is just a wrapper around
// presentation MathML; the reader should drill through.
func TestParseSemanticsTransparent(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><semantics><mrow><mn>1</mn></mrow></semantics></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(atoms) != 1 {
		t.Errorf("got %d items, want 1 (semantics+mrow should be transparent)", len(atoms))
	}
}

// TestParseAnnotationSkipped — <annotation> carries TeX source or other
// alternate notation; the presentation pipeline must not consume its content.
func TestParseAnnotationSkipped(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(
		`<math><semantics><mn>1</mn><annotation encoding="application/x-tex">\frac{1}{2}</annotation></semantics></math>`,
	), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(atoms) != 1 {
		t.Errorf("got %d items, want 1 (annotation should be skipped)", len(atoms))
	}
}

// TestParseMrowTransparent — <mrow> just groups; it should not add an extra
// atom layer (engine spacing depends on the flat list).
func TestParseMrowTransparent(t *testing.T) {
	fnt := loadMathFont(t)
	atoms, _, err := Parse([]byte(`<math><mrow><mn>1</mn><mo>+</mo><mn>2</mn></mrow></math>`), fnt)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(atoms) != 3 {
		t.Errorf("got %d items, want 3 (1, +, 2 — flat through mrow)", len(atoms))
	}
}

// TestParseErrorNoMath — input without a <math> root must error.
func TestParseErrorNoMath(t *testing.T) {
	fnt := loadMathFont(t)
	_, _, err := Parse([]byte(`<mn>1</mn>`), fnt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestParseErrorWrongChildCount — <msup> requires exactly 2 children;
// 1 child must error rather than silently doing something odd.
func TestParseErrorWrongChildCount(t *testing.T) {
	fnt := loadMathFont(t)
	_, _, err := Parse([]byte(`<math><msup><mi>x</mi></msup></math>`), fnt)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "expects 2 child") {
		t.Errorf("error message: got %q, want it to mention child count", err.Error())
	}
}

// TestParseErrorMissingGlyph — a rune the font has no glyph for must
// surface a clear error.
func TestParseErrorMissingGlyph(t *testing.T) {
	fnt := loadMathFont(t)
	// CJK codepoint (U+4E2D 中) — Latin Modern Math has no CJK coverage,
	// so the glyph lookup must fail rather than silently emit .notdef.
	_, _, err := Parse([]byte("<math><mi>中</mi></math>"), fnt)
	if err == nil {
		t.Fatal("expected error for missing glyph, got nil")
	}
	if !strings.Contains(err.Error(), "no glyph") {
		t.Errorf("error message: got %q, want it to mention missing glyph", err.Error())
	}
}

// TestRenderInline — end-to-end: a one-atom formula renders to a non-empty
// HList. This is the smoke test that the Render convenience wrapper actually
// reaches the engine.
func TestRenderInline(t *testing.T) {
	fnt := loadMathFont(t)
	hl, err := Render([]byte(`<math><mi>x</mi></math>`), fnt)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if hl == nil {
		t.Fatal("HList is nil")
	}
	if hl.Width <= 0 {
		t.Errorf("HList.Width: got %d, want > 0", int64(hl.Width))
	}
}

// TestRenderComplexFormula — exercises the same formula as the math-phase1
// demo (Inline #1: x_i + ½ √(a²+b²)) but via MathML instead of hand-built
// atoms. End-to-end Parse → Render → HList. The width should be positive
// and the formula should pack roughly like the hand-built version.
func TestRenderComplexFormula(t *testing.T) {
	fnt := loadMathFont(t)
	src := `<math>
		<msub><mi>x</mi><mi>i</mi></msub>
		<mo>+</mo>
		<mfrac><mn>1</mn><mn>2</mn></mfrac>
		<msqrt>
			<msup><mi>a</mi><mn>2</mn></msup>
			<mo>+</mo>
			<msup><mi>b</mi><mn>2</mn></msup>
		</msqrt>
	</math>`
	hl, err := Render([]byte(src), fnt)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if hl.Width <= 0 {
		t.Errorf("HList.Width: got %d, want > 0", int64(hl.Width))
	}
}

// TestRenderBigSumDisplay — <math display="block"><munderover><mo>∑</mo>...
// triggers the engine's big-op limits placement.
func TestRenderBigSumDisplay(t *testing.T) {
	fnt := loadMathFont(t)
	src := `<math display="block">
		<munderover>
			<mo>∑</mo>
			<mrow><mi>k</mi><mo>=</mo><mn>0</mn></mrow>
			<mi>n</mi>
		</munderover>
		<msup><mi>k</mi><mn>2</mn></msup>
	</math>`
	hl, err := Render([]byte(src), fnt)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if hl.Width <= 0 {
		t.Errorf("HList.Width: got %d", int64(hl.Width))
	}
	// in display mode the sum should be tall — at least body-size + the
	// limits stacked above and below
	if hl.Height < bag.MustSP("10pt") {
		t.Errorf("Height: got %d, want >= 10pt (display sum should be tall)", int64(hl.Height))
	}
}

// TestAltText checks the plain-text fallback used for a Formula element's
// /Alt: an explicit alttext attribute wins, otherwise token content is
// concatenated in document order. Neither path needs a font.
func TestAltText(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "tokens concatenated in order",
			src:  `<math><msup><mi>a</mi><mn>2</mn></msup><mo>+</mo><mi>b</mi></math>`,
			want: "a 2 + b",
		},
		{
			name: "explicit alttext attribute wins",
			src:  `<math alttext="a squared plus b"><msup><mi>a</mi><mn>2</mn></msup></math>`,
			want: "a squared plus b",
		},
		{
			name: "whitespace inside tokens collapses",
			src:  "<math><mi> x </mi><mo> = </mo><mn> 1 </mn></math>",
			want: "x = 1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AltText([]byte(tc.src)); got != tc.want {
				t.Errorf("AltText() = %q, want %q", got, tc.want)
			}
		})
	}
}
