// Package math implements an OpenType-MATH-driven math typesetting engine
// for boxesandglue. Phase 1 covers simple atoms (ord, bin, rel, …) with
// sub/sup scripts, fractions, radicals, and single-glyph accents — enough
// to render expressions like `x_i + \frac{1}{2}\sqrt{a^2+b^2}` correctly.
//
// What this package is NOT:
//   - It does not stretch delimiters (MathVariants / GlyphAssembly — phase 2).
//   - It does not auto-pick big-op variants in display mode (phase 2).
//   - It does not parse MathML — that lives in htmlbag (phase 3).
//
// Atom trees are built programmatically via the constructor helpers in this
// file (Ord, Bin, Rel, Op, …, Frac, Sqrt, AccentTop) and then handed to
// InlineMath or DisplayMath, which return a *node.HList suitable for direct
// insertion into the surrounding paragraph/vbox.
package math

import (
	"github.com/boxesandglue/textshape/ot"
)

// MathClass is the TeX-style atom class — the value that drives the inter-
// atom spacing table (TeXbook S. 170) and the Bin→Ord reclassification rules
// (Appendix G rule 5). The eight values are fixed by tradition; do not
// reorder.
type MathClass uint8

const (
	ClassOrd   MathClass = iota // ordinary symbols: x, 0, ∑'s nucleus
	ClassOp                     // large operators: ∑, ∫, lim
	ClassBin                    // binary operators: +, −, ×
	ClassRel                    // relations: =, <, ≤
	ClassOpen                   // openers: (, [, {
	ClassClose                  // closers: ), ], }
	ClassPunct                  // punctuation: , ;
	ClassInner                  // inner formulas (fraction, parenthesized group)
)

// MathItem is the marker interface satisfied by every node in a math list.
// MathItems are an internal IR — they are NOT *node.Node, and the renderer
// pipeline (linebreak, debug, PDF writer) never sees one. The Engine lowers
// MathItem trees to plain HList/VList/Glyph/Kern/Rule.
type MathItem interface {
	isMathItem()
}

// MathField is the right-hand-side of a sub/sup/nucleus slot. Exactly one of
// (Glyph, Sublist) is set; the unused half is the zero value.
//
//   - Glyph != 0 (and Sublist == nil): a single resolved glyph id.
//   - Sublist != nil:                  a nested math list, recursively laid out.
//   - both zero:                       empty field (slot absent).
type MathField struct {
	Glyph   ot.GlyphID
	Sublist []MathItem
}

// IsEmpty reports whether the field is absent (no glyph and no sublist).
func (f MathField) IsEmpty() bool { return f.Glyph == 0 && f.Sublist == nil }

// MathAtom is the workhorse: a class plus a nucleus and optional scripts.
// All three fields are values, not pointers — keeps allocations down and
// matches the LuaTeX `simple_noad` layout.
type MathAtom struct {
	Class             MathClass
	Nucleus, Sub, Sup MathField
}

func (*MathAtom) isMathItem() {}

// Fraction is `\over` / `\atop` / `\above`: a numerator and a denominator
// stacked vertically with an optional horizontal rule between them.
//
//   - Thickness > 0  → use the explicit rule thickness in scaled points.
//   - Thickness == 0 → use the font's FractionRuleThickness (default).
//   - Thickness < 0  → no rule (binomial, \atop). Stack-* constants are used
//     instead of Fraction-* constants.
//
// LeftDelim / RightDelim are phase-1-limited: the base glyph is used at its
// natural size; no stretching to body height. A warning is logged when set.
type Fraction struct {
	Num, Den              []MathItem
	Thickness             int32 // scaled points; sentinel semantics above
	LeftDelim, RightDelim ot.GlyphID
}

func (*Fraction) isMathItem() {}

// Radical is √body or ⁿ√body. Glyph is the (pre-resolved) radical sign,
// typically gid for U+221A; Degree is optional.
//
// Phase 1 limitation: the radical glyph is used at its natural size — no
// MathVariants / GlyphAssembly stretching. For bodies taller than the
// radical glyph, a warning is logged once per layout.
type Radical struct {
	Glyph        ot.GlyphID
	Body, Degree []MathItem
}

func (*Radical) isMathItem() {}

// Accent is a single-glyph accent attached above or below the body. The body
// must collapse to a single glyph; multi-glyph bodies are rejected at
// construction time (see ErrAccentBodyMultiglyph).
type Accent struct {
	Glyph  ot.GlyphID
	Bottom bool
	Body   []MathItem
}

func (*Accent) isMathItem() {}

// --- Convenience constructors -------------------------------------------
//
// These exist to make hand-written test inputs read like real math lists:
//
//	math.InlineMath(fnt, math.Ord(xGid), math.Bin(plusGid), math.Frac(num, den))
//
// They are sugar — the public types above are the real surface.

// Ord wraps a single glyph in an ordinary atom (the most common case).
func Ord(gid ot.GlyphID) *MathAtom {
	return &MathAtom{Class: ClassOrd, Nucleus: MathField{Glyph: gid}}
}

// Bin wraps a single glyph in a binary-operator atom.
func Bin(gid ot.GlyphID) *MathAtom {
	return &MathAtom{Class: ClassBin, Nucleus: MathField{Glyph: gid}}
}

// Rel wraps a single glyph in a relation atom.
func Rel(gid ot.GlyphID) *MathAtom {
	return &MathAtom{Class: ClassRel, Nucleus: MathField{Glyph: gid}}
}

// Op wraps a single glyph in a large-operator atom.
func Op(gid ot.GlyphID) *MathAtom {
	return &MathAtom{Class: ClassOp, Nucleus: MathField{Glyph: gid}}
}

// Open wraps a single glyph in an opener atom.
func Open(gid ot.GlyphID) *MathAtom {
	return &MathAtom{Class: ClassOpen, Nucleus: MathField{Glyph: gid}}
}

// Close wraps a single glyph in a closer atom.
func Close(gid ot.GlyphID) *MathAtom {
	return &MathAtom{Class: ClassClose, Nucleus: MathField{Glyph: gid}}
}

// Punct wraps a single glyph in a punctuation atom.
func Punct(gid ot.GlyphID) *MathAtom {
	return &MathAtom{Class: ClassPunct, Nucleus: MathField{Glyph: gid}}
}

// Inner wraps a sublist in an inner atom (the result of a nested fraction or
// parenthesized group, when the caller wants explicit inner spacing).
func Inner(items ...MathItem) *MathAtom {
	return &MathAtom{Class: ClassInner, Nucleus: MathField{Sublist: items}}
}

// WithSub attaches a subscript to an atom and returns it. Chainable.
func (a *MathAtom) WithSub(items ...MathItem) *MathAtom {
	a.Sub = MathField{Sublist: items}
	return a
}

// WithSup attaches a superscript to an atom and returns it. Chainable.
func (a *MathAtom) WithSup(items ...MathItem) *MathAtom {
	a.Sup = MathField{Sublist: items}
	return a
}

// WithSubGlyph attaches a single-glyph subscript to an atom and returns it.
func (a *MathAtom) WithSubGlyph(gid ot.GlyphID) *MathAtom {
	a.Sub = MathField{Glyph: gid}
	return a
}

// WithSupGlyph attaches a single-glyph superscript to an atom and returns it.
func (a *MathAtom) WithSupGlyph(gid ot.GlyphID) *MathAtom {
	a.Sup = MathField{Glyph: gid}
	return a
}

// Frac is a default-thickness fraction (font's FractionRuleThickness).
func Frac(num, den []MathItem) *Fraction {
	return &Fraction{Num: num, Den: den, Thickness: 0}
}

// Binom is an over-the-top binomial: numerator stacked on denominator with
// NO rule, using StackTopShiftUp / StackBottomShiftDown / StackGapMin.
func Binom(num, den []MathItem) *Fraction {
	return &Fraction{Num: num, Den: den, Thickness: -1}
}

// Sqrt is a radical with no degree.
func Sqrt(glyph ot.GlyphID, body ...MathItem) *Radical {
	return &Radical{Glyph: glyph, Body: body}
}

// NRoot is a radical with a degree (e.g. ∛, ∜).
func NRoot(glyph ot.GlyphID, degree, body []MathItem) *Radical {
	return &Radical{Glyph: glyph, Degree: degree, Body: body}
}

// AccentTop builds an over-the-top accent (hat, bar, tilde, …).
func AccentTop(glyph ot.GlyphID, body ...MathItem) *Accent {
	return &Accent{Glyph: glyph, Bottom: false, Body: body}
}

// AccentBottom builds an under-the-body accent.
func AccentBottom(glyph ot.GlyphID, body ...MathItem) *Accent {
	return &Accent{Glyph: glyph, Bottom: true, Body: body}
}
