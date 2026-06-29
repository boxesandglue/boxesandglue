package mathml

import "github.com/boxesandglue/boxesandglue/frontend/math"

// opClasses is the minimal operator dictionary that drives <mo> classification.
// Entries follow the W3C MathML Operator Dictionary (Appendix C of MathML 3),
// but cherry-picked to the most common Latin-input operators — the full
// dictionary has ~1100 entries and is overkill for v1.
//
// Conflicts (same glyph used as both bin and rel in different sources): we
// pick the more common reading. Unknown operators fall through to ClassOrd
// which gives them neutral spacing.
var opClasses = map[rune]math.MathClass{
	// binary operators
	'+': math.ClassBin,
	'-': math.ClassBin, '−': math.ClassBin, // ASCII -, U+2212 MINUS SIGN
	'*': math.ClassBin, '×': math.ClassBin, '⋅': math.ClassBin, // *, ×, ⋅
	'/': math.ClassBin, '÷': math.ClassBin, // /, ÷
	'±': math.ClassBin, '∓': math.ClassBin, // ±, ∓
	'∧': math.ClassBin, '∨': math.ClassBin, // ∧, ∨
	'∩': math.ClassBin, '∪': math.ClassBin, // ∩, ∪
	'⊕': math.ClassBin, '⊗': math.ClassBin, // ⊕, ⊗
	'∘': math.ClassBin, // ∘
	'∖': math.ClassBin, // ∖ set-minus

	// relations
	'=': math.ClassRel,
	'<': math.ClassRel, '>': math.ClassRel,
	'≤': math.ClassRel, '≥': math.ClassRel, // ≤, ≥
	'≠': math.ClassRel, '≈': math.ClassRel, // ≠, ≈
	'≡': math.ClassRel, '∼': math.ClassRel, // ≡, ∼
	'∈': math.ClassRel, '∉': math.ClassRel, // ∈, ∉
	'⊂': math.ClassRel, '⊃': math.ClassRel, // ⊂, ⊃
	'⊆': math.ClassRel, '⊇': math.ClassRel, // ⊆, ⊇
	'→': math.ClassRel, '←': math.ClassRel, // →, ←
	'↔': math.ClassRel,                     // ↔
	'⇒': math.ClassRel, '⇐': math.ClassRel, // ⇒, ⇐
	'⇔': math.ClassRel,                     // ⇔
	'⊢': math.ClassRel, '⊣': math.ClassRel, // ⊢, ⊣
	'⊨': math.ClassRel, // ⊨

	// openers
	'(': math.ClassOpen,
	'[': math.ClassOpen,
	'{': math.ClassOpen,
	'⟨': math.ClassOpen, // ⟨
	'⌈': math.ClassOpen, // ⌈
	'⌊': math.ClassOpen, // ⌊

	// closers
	')': math.ClassClose,
	']': math.ClassClose,
	'}': math.ClassClose,
	'⟩': math.ClassClose, // ⟩
	'⌉': math.ClassClose, // ⌉
	'⌋': math.ClassClose, // ⌋

	// punctuation
	',': math.ClassPunct,
	';': math.ClassPunct,

	// large operators
	'∑': math.ClassOp, // ∑
	'∏': math.ClassOp, // ∏
	'∐': math.ClassOp, // ∐
	'∫': math.ClassOp, // ∫
	'∬': math.ClassOp, // ∬
	'∭': math.ClassOp, // ∭
	'∮': math.ClassOp, // ∮
	'⋃': math.ClassOp, // ⋃
	'⋂': math.ClassOp, // ⋂
	'⨁': math.ClassOp, // ⨁
	'⨂': math.ClassOp, // ⨂
}

// operatorClass returns the MathClass for an operator rune. Unknown operators
// degrade to ClassOrd (zero kern in the spacing table) rather than erroring —
// this keeps unfamiliar MathML rendering rather than failing.
func operatorClass(r rune) math.MathClass {
	if c, ok := opClasses[r]; ok {
		return c
	}
	return math.ClassOrd
}
