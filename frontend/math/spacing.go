package math

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// Spacing codes used inside spacingTable. Stored as int8 so a single byte
// can carry both the magnitude (1/2/3 = thin/med/thick) and a sign bit:
// a NEGATIVE entry means "suppressed in script and scriptscript styles" —
// this is the TeXbook's parenthesis notation transposed to a numeric form.
const (
	spcNone  int8 = 0
	spcThin  int8 = 1 // 3 mu
	spcMed   int8 = 2 // 4 mu
	spcThick int8 = 3 // 5 mu
)

// spacingTable encodes TeXbook S. 170 (the canonical 8×8 inter-atom
// spacing table). Rows index the left atom's class, columns the right
// atom's class. Bin/Bin, Bin/Rel etc. positions are "*" in TeXbook
// (illegal pairs) — they are never queried in practice because the
// Bin→Ord reclassification (rewriteBinToOrd) runs first; we keep them
// at 0 defensively.
//
// Suppression in script and scriptscript styles is marked with a leading
// minus, matching LuaTeX's SPLIT_STYLES(... , 0) macro pattern in mlist.c.
var spacingTable = [8][8]int8{
	// rt:                  Ord       Op        Bin       Rel        Open      Close      Punct      Inner
	/* Ord   */ {spcNone, spcThin, -spcMed, -spcThick, spcNone, spcNone, spcNone, -spcThin},
	/* Op    */ {spcThin, spcThin, spcNone, -spcThick, spcNone, spcNone, spcNone, -spcThin},
	/* Bin   */ {-spcMed, -spcMed, spcNone, spcNone, -spcMed, spcNone, spcNone, -spcMed},
	/* Rel   */ {-spcThick, -spcThick, spcNone, spcNone, -spcThick, spcNone, spcNone, -spcThick},
	/* Open  */ {spcNone, spcNone, spcNone, spcNone, spcNone, spcNone, spcNone, spcNone},
	/* Close */ {spcNone, spcThin, -spcMed, -spcThick, spcNone, spcNone, spcNone, -spcThin},
	/* Punct */ {-spcThin, -spcThin, spcNone, -spcThin, -spcThin, -spcThin, -spcThin, -spcThin},
	/* Inner */ {-spcThin, spcThin, -spcMed, -spcThick, -spcThin, spcNone, -spcThin, -spcThin},
}

// interAtomSpace returns the kern (in scaled points) that must appear between
// a left atom of class lc and a right atom of class rc, at the given math
// style and body size. Returns 0 if the table says "no space" or the entry
// is suppressed at the current style.
//
// The math-unit (mu) is 1/18 of the body size, per TeX convention. The body
// size passed in is the SURROUNDING style's size, not a script-scaled one
// — TeX treats \thinmuskip etc. as referring to the outer text size.
func interAtomSpace(lc, rc MathClass, style MathStyle, size bag.ScaledPoint) bag.ScaledPoint {
	v := spacingTable[lc][rc]
	if v == 0 {
		return 0
	}
	if v < 0 {
		// Negative magnitude → suppressed in script / scriptscript.
		if style >= ScriptStyle {
			return 0
		}
		v = -v
	}
	var muCount int64
	switch v {
	case spcThin:
		muCount = 3
	case spcMed:
		muCount = 4
	case spcThick:
		muCount = 5
	default:
		return 0
	}
	return bag.ScaledPoint(int64(size) * muCount / 18)
}

// classOf returns the effective class for a non-Atom MathItem. Fractions
// become Inner (they get inner spacing on both sides); Radicals and Accents
// become Ord (they participate in ordinary-symbol spacing). For Atoms the
// stored class is returned.
func classOf(item MathItem) MathClass {
	switch i := item.(type) {
	case *MathAtom:
		return i.Class
	case *Fraction:
		return ClassInner
	case *Radical:
		return ClassOrd
	case *Accent:
		return ClassOrd
	}
	return ClassOrd
}

// rewriteBinToOrd implements TeXbook Appendix G rule 5: a Bin atom is
// reclassified to Ord when its position makes binary-operator semantics
// impossible. The conditions, in order of precedence:
//
//  1. The Bin is the first element in the list.
//  2. The left neighbor is Bin / Op / Rel / Open / Punct.
//  3. The right neighbor is Rel / Close / Punct.
//  4. The Bin is the last element in the list (no right operand at all).
//
// Mutates atom.Class in place. The function is idempotent: a second call on
// the same list is a no-op because Ord stays Ord. Non-Atom items are skipped
// — Fractions, Radicals and Accents have a fixed class.
func rewriteBinToOrd(items []MathItem) {
	// Pre-compute classes so a single pass can look at neighbors.
	classes := make([]MathClass, len(items))
	for i, it := range items {
		classes[i] = classOf(it)
	}
	for i, it := range items {
		atom, ok := it.(*MathAtom)
		if !ok || atom.Class != ClassBin {
			continue
		}
		reclassify := false
		switch {
		case i == 0:
			reclassify = true
		case i+1 == len(items):
			reclassify = true
		default:
			left := classes[i-1]
			right := classes[i+1]
			if left == ClassBin || left == ClassOp || left == ClassRel ||
				left == ClassOpen || left == ClassPunct {
				reclassify = true
			} else if right == ClassRel || right == ClassClose || right == ClassPunct {
				reclassify = true
			}
		}
		if reclassify {
			atom.Class = ClassOrd
			classes[i] = ClassOrd
		}
	}
}
