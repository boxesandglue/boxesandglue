package mathml

// applyVariant maps a rune through the MathML mathvariant attribute to its
// stylistic Unicode equivalent. The Unicode Mathematical Alphanumeric Symbols
// block (U+1D400–U+1D7FF) carries pre-styled letters; rather than relying on
// fonts to honour OpenType variant tags, we just pick the right codepoint
// up front and let the engine work on plain glyphs.
//
// v1 covers the two cases that actually arise in default MathML rendering:
// "italic" (single-letter <mi> default) and "normal" (multi-letter <mi>,
// <mn>, <mo>). Bold, bold-italic, double-struck, fraktur, script,
// sans-serif and monospace remain pass-through for now — they need their
// own per-variant lookup tables and aren't required for the default-rendered
// MathML that pandoc-style toolchains emit.
//
// Runes outside the ASCII letter range (digits, Greek, symbols, …) are
// returned unchanged. Specifically: variables remain in U+1D44E ff for
// italic; Greek letters are not yet remapped to the Mathematical Greek block.
func applyVariant(r rune, variant string) rune {
	switch variant {
	case "italic":
		return mathItalic(r)
	case "normal", "", "upright":
		return r
	default:
		// unknown variant (bold, sans-serif, …) — leave the rune as-is so
		// the user at least gets the upright glyph instead of a missing one
		return r
	}
}

// mathItalic maps an ASCII letter to its Mathematical Italic codepoint
// (U+1D434 ff for uppercase, U+1D44E ff for lowercase). Non-letters pass
// through unchanged.
//
// Special case: U+1D455 (italic lowercase h) is reserved by Unicode because
// the slot conflicts with the Planck constant ℎ (U+210E). Anyone rendering
// a math italic h must use U+210E instead — we map h → U+210E here so the
// font lookup succeeds.
func mathItalic(r rune) rune {
	switch {
	case r == 'h':
		return 0x210E
	case r >= 'a' && r <= 'z':
		return 0x1D44E + (r - 'a')
	case r >= 'A' && r <= 'Z':
		return 0x1D434 + (r - 'A')
	}
	return r
}
