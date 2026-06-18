package math

import (
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// 18 sp at body size = 18 sp = 1 mu. Easy arithmetic for the tests below.
const testSize bag.ScaledPoint = 18

func TestInterAtomSpace_OrdOp_Thin(t *testing.T) {
	got := interAtomSpace(ClassOrd, ClassOp, TextStyle, testSize)
	if got != 3 {
		t.Errorf("Ord/Op = %d sp, want 3 (=3mu)", got)
	}
}

func TestInterAtomSpace_OrdBin_MedSuppressedInScript(t *testing.T) {
	if got := interAtomSpace(ClassOrd, ClassBin, TextStyle, testSize); got != 4 {
		t.Errorf("Ord/Bin (text) = %d, want 4", got)
	}
	if got := interAtomSpace(ClassOrd, ClassBin, ScriptStyle, testSize); got != 0 {
		t.Errorf("Ord/Bin (script) = %d, want 0 (suppressed)", got)
	}
	if got := interAtomSpace(ClassOrd, ClassBin, ScriptScriptStyle, testSize); got != 0 {
		t.Errorf("Ord/Bin (scriptscript) = %d, want 0 (suppressed)", got)
	}
}

func TestInterAtomSpace_OpOp_ThinKeptInScript(t *testing.T) {
	// Op/Op is unsigned-thin in the table — kept in every style.
	for _, st := range []MathStyle{TextStyle, ScriptStyle, ScriptScriptStyle} {
		if got := interAtomSpace(ClassOp, ClassOp, st, testSize); got != 3 {
			t.Errorf("Op/Op @ style %d = %d, want 3", st, got)
		}
	}
}

func TestInterAtomSpace_RelRel_NoSpace(t *testing.T) {
	if got := interAtomSpace(ClassRel, ClassRel, TextStyle, testSize); got != 0 {
		t.Errorf("Rel/Rel = %d, want 0 (table position is 0)", got)
	}
}

func TestRewriteBinToOrd_LeadingBin(t *testing.T) {
	// `+ x` — the leading Bin must be reclassified.
	items := []MathItem{Bin(1), Ord(2)}
	rewriteBinToOrd(items)
	if got := items[0].(*MathAtom).Class; got != ClassOrd {
		t.Errorf("leading Bin → class = %v, want Ord", got)
	}
}

func TestRewriteBinToOrd_TrailingBin(t *testing.T) {
	// `x +` — the trailing Bin must be reclassified.
	items := []MathItem{Ord(1), Bin(2)}
	rewriteBinToOrd(items)
	if got := items[1].(*MathAtom).Class; got != ClassOrd {
		t.Errorf("trailing Bin → class = %v, want Ord", got)
	}
}

func TestRewriteBinToOrd_BinAfterBin(t *testing.T) {
	// `x + + y` — TeXbook rule 5 reclassifies the second Bin (its left
	// neighbor is Bin), but leaves the first Bin alone (between Ord and Bin,
	// no trigger fires). LuaTeX mlist.c:4396-4420 has the same behavior.
	items := []MathItem{Ord(1), Bin(2), Bin(3), Ord(4)}
	rewriteBinToOrd(items)
	if got := items[1].(*MathAtom).Class; got != ClassBin {
		t.Errorf("first Bin (Ord on left, Bin on right) class = %v, want Bin", got)
	}
	if got := items[2].(*MathAtom).Class; got != ClassOrd {
		t.Errorf("second Bin (Bin on left) class = %v, want Ord", got)
	}
}

func TestRewriteBinToOrd_BinBeforeRel(t *testing.T) {
	// `x + = y` — the Bin precedes Rel, which is illegal → reclassify to Ord.
	items := []MathItem{Ord(1), Bin(2), Rel(3), Ord(4)}
	rewriteBinToOrd(items)
	if got := items[1].(*MathAtom).Class; got != ClassOrd {
		t.Errorf("Bin before Rel → class = %v, want Ord", got)
	}
}

func TestRewriteBinToOrd_BinAfterRel(t *testing.T) {
	// `x = + y` — the Bin follows Rel → reclassify to Ord.
	items := []MathItem{Ord(1), Rel(2), Bin(3), Ord(4)}
	rewriteBinToOrd(items)
	if got := items[2].(*MathAtom).Class; got != ClassOrd {
		t.Errorf("Bin after Rel → class = %v, want Ord", got)
	}
}

func TestRewriteBinToOrd_BinSurvives(t *testing.T) {
	// `x + y` — the Bin is between Ord and Ord → must stay Bin.
	items := []MathItem{Ord(1), Bin(2), Ord(3)}
	rewriteBinToOrd(items)
	if got := items[1].(*MathAtom).Class; got != ClassBin {
		t.Errorf("Bin between Ords → class = %v, want Bin (no reclassify)", got)
	}
}

func TestRewriteBinToOrd_IsIdempotent(t *testing.T) {
	// Running twice produces the same result.
	items := []MathItem{Bin(1), Ord(2), Bin(3), Rel(4), Ord(5)}
	rewriteBinToOrd(items)
	rewriteBinToOrd(items)
	want := []MathClass{ClassOrd, ClassOrd, ClassOrd, ClassRel, ClassOrd}
	for i, w := range want {
		if got := items[i].(*MathAtom).Class; got != w {
			t.Errorf("item[%d] class = %v, want %v after two rewrite passes", i, got, w)
		}
	}
}

func TestClassOf_NonAtoms(t *testing.T) {
	if classOf(Frac(nil, nil)) != ClassInner {
		t.Errorf("Fraction class should be Inner")
	}
	if classOf(Sqrt(0)) != ClassOrd {
		t.Errorf("Radical class should be Ord")
	}
	if classOf(AccentTop(0, Ord(1))) != ClassOrd {
		t.Errorf("Accent class should be Ord")
	}
}
