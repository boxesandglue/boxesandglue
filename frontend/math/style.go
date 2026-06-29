package math

import (
	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/textshape/ot"
)

// MathStyle is the 4-way (display/text/script/scriptscript) × 2-way
// (uncramped/cramped) TeX style. Cramping affects superscript shift; the
// "denominator of fraction" rule, "body of radical" rule, and "body of
// accent" rule all enter cramped style — see TeXbook Appendix G.
//
// The 8 values are ordered so that `style/2` gives the size index
// (0=display, 1=text, 2=script, 3=scriptscript) and `style%2 == 1` means
// cramped. That arithmetic is used in subStyle / scaleFor.
type MathStyle uint8

const (
	DisplayStyle             MathStyle = iota // 0
	DisplayStyleCramped                       // 1
	TextStyle                                 // 2
	TextStyleCramped                          // 3
	ScriptStyle                               // 4
	ScriptStyleCramped                        // 5
	ScriptScriptStyle                         // 6
	ScriptScriptStyleCramped                  // 7
)

// IsCramped reports whether the style is one of the four cramped variants.
func (s MathStyle) IsCramped() bool { return s&1 == 1 }

// IsDisplay reports whether the style is DisplayStyle or DisplayStyleCramped.
// Display style triggers the FractionNumDisplayStyleShiftUp constants
// (taller, looser spacing) and large-op limits-above-and-below positioning.
func (s MathStyle) IsDisplay() bool { return s == DisplayStyle || s == DisplayStyleCramped }

// crampify returns the cramped variant of the given style. Cramping a
// cramped style is a no-op.
func crampify(s MathStyle) MathStyle { return s | 1 }

// subStyle is the style used inside a subscript. TeXbook Appendix G
// (page 442) gives:
//
//	D, D' → S, S'
//	T, T' → S, S'
//	S, S' → S', S'
//	S', S' → S', S'
//
// In other words: Display AND Text both step *directly* to Script
// (NOT sequentially through Text). That's why `k^2` in display mode
// renders the `2` in script style, the same size as in inline `k^2`
// — not in text style. Subscript is also always cramped.
func subStyle(s MathStyle) MathStyle {
	return crampify(scriptStepDown(s))
}

// supStyle is the style used inside a superscript. Same step rule as
// sub (D, T → S; S → S'), with cramping inherited from the caller.
func supStyle(s MathStyle) MathStyle {
	d := scriptStepDown(s)
	if s.IsCramped() {
		return crampify(d)
	}
	return d &^ 1
}

// scriptStepDown is the per-script step-down rule from TeXbook G:
// Display AND Text both go to Script, Script goes to ScriptScript,
// ScriptScript is the fixed point. This is DIFFERENT from the
// num/den-style rule (which steps D → T → S sequentially), and it's
// what makes superscripts compact regardless of whether the
// surrounding style is display or inline.
func scriptStepDown(s MathStyle) MathStyle {
	switch s {
	case DisplayStyle, DisplayStyleCramped:
		return ScriptStyle | (s & 1)
	case TextStyle, TextStyleCramped:
		return ScriptStyle | (s & 1)
	case ScriptStyle, ScriptStyleCramped:
		return ScriptScriptStyle | (s & 1)
	default:
		return s
	}
}

// numStyle is the style used inside a fraction numerator: one step down,
// cramping inherited.
func numStyle(s MathStyle) MathStyle {
	d := stepDown(s)
	if s.IsCramped() {
		return crampify(d)
	}
	return d &^ 1
}

// denStyle is the style used inside a fraction denominator: one step down,
// always cramped.
func denStyle(s MathStyle) MathStyle {
	return crampify(stepDown(s))
}

// stepDown shifts the size class one step toward ScriptScriptStyle. The
// cramping bit is preserved; callers apply their own cramping rule on top.
//
// display → text → script → scriptscript → scriptscript (fixed point)
func stepDown(s MathStyle) MathStyle {
	switch s {
	case DisplayStyle, DisplayStyleCramped:
		return TextStyle | (s & 1)
	case TextStyle, TextStyleCramped:
		return ScriptStyle | (s & 1)
	case ScriptStyle, ScriptStyleCramped:
		return ScriptScriptStyle | (s & 1)
	default:
		return s
	}
}

// scaleFor returns the per-size scale factor as a (numerator, denominator)
// pair, derived from the font's ScriptPercentScaleDown and
// ScriptScriptPercentScaleDown constants.
//
//   - display/text:    1   (numerator = denominator)
//   - script:          script_percent / 100
//   - scriptscript:    scriptscript_percent / 100
//
// Returning a fraction (not a float) lets callers do scaledpoint × num / den
// arithmetic with no rounding outside the final step. If the font's
// percent values are 0 (malformed MATH table), TeX's typical 70/50 defaults
// are used — matches LuaTeX `default_sup_*` behavior.
func scaleFor(s MathStyle, c *ot.MathConstants) (num, den int32) {
	switch s {
	case ScriptStyle, ScriptStyleCramped:
		p := int32(c.ScriptPercentScaleDown)
		if p <= 0 {
			p = 70
		}
		return p, 100
	case ScriptScriptStyle, ScriptScriptStyleCramped:
		p := int32(c.ScriptScriptPercentScaleDown)
		if p <= 0 {
			p = 50
		}
		return p, 100
	default:
		return 1, 1
	}
}

// scaledSize returns the effective body size at the given style.
func scaledSize(baseSize bag.ScaledPoint, s MathStyle, c *ot.MathConstants) bag.ScaledPoint {
	num, den := scaleFor(s, c)
	if num == den {
		return baseSize
	}
	return bag.ScaledPoint(int64(baseSize) * int64(num) / int64(den))
}
