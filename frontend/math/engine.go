package math

import (
	"errors"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/textshape/ot"
)

// ErrNoMathFont is returned by InlineMath / DisplayMath when the given font
// has no OpenType MATH table. Callers should fall back to a math-capable
// font (Latin Modern Math, STIX Two Math, …) or report the missing data to
// the user.
var ErrNoMathFont = errors.New("math: font has no OpenType MATH table")

// ErrPhase2Feature is the sentinel for features the engine acknowledges but
// does not implement in phase 1 — stretchy delimiters, big-op variants,
// \mathchoice, mid-list style switch. Constructors that detect such input
// at build time return this; layout-time degradations only log a warning.
var ErrPhase2Feature = errors.New("math: feature not yet supported")

// ErrAccentBodyMultiglyph is returned by AccentTop / AccentBottom when the
// body collapses to more than one glyph at layout time. Phase 1 only
// supports single-glyph bodies under accents.
var ErrAccentBodyMultiglyph = errors.New("math: accent body must be a single glyph in phase 1")

// InlineMath lays out a math list in text style and returns the resulting
// HList, suitable for embedding inside a paragraph. The HList carries
// height and depth chosen so the surrounding line spacing can react to a
// tall numerator or a deep radical without overlapping neighbors.
func InlineMath(fnt *font.Font, items ...MathItem) (*node.HList, error) {
	return runEngine(items, TextStyle, fnt)
}

// DisplayMath lays out a math list in display style and returns the
// resulting HList, suitable for centering on its own line. Display style
// triggers the FractionNumeratorDisplayStyleShiftUp etc. constants, which
// give a taller, more open look than text style.
func DisplayMath(fnt *font.Font, items ...MathItem) (*node.HList, error) {
	return runEngine(items, DisplayStyle, fnt)
}

func runEngine(items []MathItem, style MathStyle, fnt *font.Font) (*node.HList, error) {
	if fnt == nil {
		return nil, errors.New("math: nil font")
	}
	c := fnt.MathConstantsFU()
	if c == nil {
		return nil, ErrNoMathFont
	}
	ctx := newEngineCtx(fnt, c)
	return mlistToHlist(items, style, ctx), nil
}

// engineCtx carries the constants and a per-style font cache through one
// mlistToHlist pass. Each MathStyle has a distinct effective size (display
// / text at full body size; script and scriptscript reduced via
// ScriptPercentScaleDown / ScriptScriptPercentScaleDown). The PDF renderer
// emits one `Tf` per Glyph from `Glyph.Font.Size`, so a sub/sup glyph must
// actually carry a smaller-size *font.Font — measuring with the right
// width but rendering at full size produces visible overlap, which is the
// "indices look full-sized" bug that prompted this refactor.
//
// Builds are lazy and capped at 8 entries (one per MathStyle); building
// one entry runs font.NewFont, which shapes "-" and " " for hyphen and
// space metrics. That cost is paid once per style per engine pass.
type engineCtx struct {
	base  *font.Font
	cons  *ot.MathConstants
	cache [8]*font.Font
}

func newEngineCtx(base *font.Font, c *ot.MathConstants) *engineCtx {
	ctx := &engineCtx{base: base, cons: c}
	// Display and Text styles share the base font's size — populate the
	// cache so .at() returns the canonical instance without a re-build.
	ctx.cache[DisplayStyle] = base
	ctx.cache[DisplayStyleCramped] = base
	ctx.cache[TextStyle] = base
	ctx.cache[TextStyleCramped] = base
	return ctx
}

// at returns the *font.Font for the given math style. Display/Text get the
// base; Script and ScriptScript get freshly-built instances at the scaled
// body size. Cached so a deeply-nested formula doesn't pay the rebuild
// cost on every glyph.
func (ctx *engineCtx) at(style MathStyle) *font.Font {
	if f := ctx.cache[style]; f != nil {
		return f
	}
	size := scaledSize(ctx.base.Size, style, ctx.cons)
	f := font.NewFont(ctx.base.Face, size)
	ctx.cache[style] = f
	return f
}

// mlistToHlist is TeX's `mlist_to_hlist` translated to our IR — see TeXbook
// Appendix G for the conceptual chapter. Three passes:
//
//	A. rewriteBinToOrd — reclassify Bin atoms whose position makes the
//	   binary-operator reading impossible (rule 5).
//	B. per-item layout — lower each MathItem to an HList, remembering its
//	   class (so the spacing pass can look it up).
//	C. spliceWithSpacing — emit inter-atom kerns from the 8×8 spacing
//	   table.
//
// The result is a flat HList: parent containers see one box, the math
// internals are opaque.
func mlistToHlist(items []MathItem, style MathStyle, ctx *engineCtx) *node.HList {
	if len(items) == 0 {
		return emptyHList()
	}
	rewriteBinToOrd(items)

	parts := make([]laidPart, 0, len(items))
	for _, it := range items {
		p := layoutItem(it, style, ctx)
		if p != nil {
			parts = append(parts, *p)
		}
	}
	// Spacing computes mu = size/18 at the outer text style's size (not
	// script-scaled). For DisplayStyle and TextStyle that is base.Size;
	// for nested Script/ScriptScript recursion (sub of sub, denom of
	// fraction's sup, etc.), the outer style's scaled size is what TeX
	// uses for inter-atom kerns.
	return spliceWithSpacing(parts, style, ctx.at(style).Size)
}

// laidPart is one already-laid-out item, carrying its final HList plus the
// class the spacing pass uses to look up the inter-atom kern.
type laidPart struct {
	hl    *node.HList
	class MathClass
}

func layoutItem(it MathItem, style MathStyle, ctx *engineCtx) *laidPart {
	switch i := it.(type) {
	case *MathAtom:
		hl := layoutAtom(i, style, ctx)
		return &laidPart{hl: hl, class: i.Class}
	case *Fraction:
		hl := layoutFraction(i, style, ctx)
		return &laidPart{hl: hl, class: ClassInner}
	case *Radical:
		hl := layoutRadical(i, style, ctx)
		return &laidPart{hl: hl, class: ClassOrd}
	case *Accent:
		hl := layoutAccent(i, style, ctx)
		return &laidPart{hl: hl, class: ClassOrd}
	}
	return nil
}

// spliceWithSpacing concatenates the per-item HLists into one outer HList,
// inserting Kern nodes at the boundaries to realize TeX's inter-atom
// spacing table. The outer HList's bounding box is the union of the
// children's boxes — Shift-positioned children are already accounted for
// in their own Height / Depth by the layout helpers.
func spliceWithSpacing(parts []laidPart, style MathStyle, size bag.ScaledPoint) *node.HList {
	if len(parts) == 0 {
		return emptyHList()
	}
	var head, tail node.Node
	var width, height, depth bag.ScaledPoint
	for i, p := range parts {
		if i > 0 {
			kern := interAtomSpace(parts[i-1].class, p.class, style, size)
			if kern > 0 {
				k := node.NewKern()
				k.Kern = kern
				head, tail = appendNode(head, tail, k)
				width += kern
			}
		}
		head, tail = appendNode(head, tail, p.hl)
		width += p.hl.Width
		if p.hl.Height > height {
			height = p.hl.Height
		}
		if p.hl.Depth > depth {
			depth = p.hl.Depth
		}
	}
	out := node.NewHList()
	out.List = head
	out.Width = width
	out.Height = height
	out.Depth = depth
	return out
}

// appendNode links n at the tail of (head, tail) and returns the new
// (head, tail). Empty list initializes head and tail to n.
func appendNode(head, tail, n node.Node) (node.Node, node.Node) {
	if head == nil {
		return n, n
	}
	tail.SetNext(n)
	n.SetPrev(tail)
	return head, n
}

func emptyHList() *node.HList {
	return node.NewHList()
}
