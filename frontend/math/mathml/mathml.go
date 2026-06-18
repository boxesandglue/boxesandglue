// Package mathml reads MathML markup and produces a math-engine atom tree
// ready to feed into [math.InlineMath] or [math.DisplayMath]. It is Phase 3a
// of the OpenType-MATH project: the bridge between web-style math markup and
// the GID-level engine in the parent [math] package.
//
// Element coverage (Phase 3a, v1):
//
//	<math>, <mrow>, <mi>, <mn>, <mo>,
//	<msup>, <msub>, <msubsup>,
//	<mfrac>, <msqrt>, <mroot>,
//	<munder>, <mover>, <munderover>,
//	<semantics> (transparent), <annotation*> (skipped)
//
// Two entry points: [Parse] returns the raw atom list plus the display flag,
// [Render] does Parse + dispatch to InlineMath/DisplayMath and returns the
// final HList. Use Parse from htmlbag (which may want to override the display
// mode via CSS), Render from glu/bagme one-shot calls.
package mathml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/node"
	"github.com/boxesandglue/boxesandglue/frontend/math"
	"github.com/boxesandglue/textshape/ot"
)

// Parse reads a MathML <math> fragment from src and returns the math-item
// tree it encodes. The boolean is true when the root carries display="block"
// (caller should route to DisplayMath); false otherwise.
//
// fnt is used to resolve character runes to glyph IDs — the engine works on
// GIDs, so the reader must know the font up front. fnt must be a math font
// (one with an OpenType MATH table); the engine will reject non-math fonts
// downstream with [math.ErrNoMathFont].
func Parse(src []byte, fnt *font.Font) ([]math.MathItem, bool, error) {
	if fnt == nil {
		return nil, false, errors.New("mathml: nil font")
	}
	p := &parser{
		dec: xml.NewDecoder(bytes.NewReader(src)),
		fnt: fnt,
	}
	return p.parseRoot()
}

// Render is a convenience wrapper: Parse + dispatch to InlineMath or
// DisplayMath based on the root display attribute. Returns the final HList
// suitable for direct insertion into a paragraph or vbox.
func Render(src []byte, fnt *font.Font) (*node.HList, error) {
	atoms, display, err := Parse(src, fnt)
	if err != nil {
		return nil, err
	}
	if display {
		return math.DisplayMath(fnt, atoms...)
	}
	return math.InlineMath(fnt, atoms...)
}

// AltText returns a plain-text approximation of a MathML fragment, suitable
// for the /Alt entry of a PDF Formula structure element (PDF/UA fallback when
// a reader cannot consume the MathML associated file).
//
// If the root <math> element carries an alttext attribute (the MathML-native
// way to supply alternative text, MathML 3 §2.1.5), that value is returned
// verbatim. Otherwise AltText concatenates the textual content of every
// token element (<mi>, <mn>, <mo>, <mtext>) in document order, separated by
// single spaces. This is a deliberately simple rendering: it preserves the
// symbols and their order but not the 2-D structure (a superscript reads the
// same as a following digit). Authors who need a precise spoken form should
// set alttext explicitly; the embedded MathML carries the full semantics.
func AltText(src []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(src))
	var parts []string
	// inToken tracks whether we are inside a leaf token element whose
	// character data should be collected. Nesting of token elements does
	// not occur in valid MathML, so a simple flag suffices.
	inToken := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "math" {
				for _, a := range t.Attr {
					if a.Name.Local == "alttext" && strings.TrimSpace(a.Value) != "" {
						return strings.TrimSpace(a.Value)
					}
				}
			}
			switch t.Name.Local {
			case "mi", "mn", "mo", "mtext":
				inToken = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "mi", "mn", "mo", "mtext":
				inToken = false
			}
		case xml.CharData:
			if inToken {
				if s := strings.TrimSpace(string(t)); s != "" {
					parts = append(parts, s)
				}
			}
		}
	}
	return strings.Join(parts, " ")
}

// parser holds the shared state for one Parse pass: the XML decoder and the
// font used for GID lookup. Stateless apart from those — the recursion is
// driven by the decoder cursor advancing through the token stream.
type parser struct {
	dec *xml.Decoder
	fnt *font.Font
}

// parseRoot scans the token stream for the first StartElement, requires it
// to be <math>, reads the display attribute, and parses the body.
func (p *parser) parseRoot() ([]math.MathItem, bool, error) {
	for {
		tok, err := p.dec.Token()
		if err == io.EOF {
			return nil, false, errors.New("mathml: no <math> element found")
		}
		if err != nil {
			return nil, false, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local != "math" {
			return nil, false, fmt.Errorf("mathml: root element must be <math>, got <%s>", se.Name.Local)
		}
		display := attr(se, "display") == "block"
		items, err := p.parseChildren(se)
		if err != nil {
			return nil, false, err
		}
		return items, display, nil
	}
}

// parseChildren reads the entire content of start up to (and consuming) its
// matching EndElement, returning the concatenated items of every child
// element. Whitespace CharData between elements is ignored. Text inside
// non-leaf elements (e.g. inside an <mrow>) is also ignored — only leaf
// token elements (<mi>, <mn>, <mo>) consume text.
func (p *parser) parseChildren(start xml.StartElement) ([]math.MathItem, error) {
	var items []math.MathItem
	for {
		tok, err := p.dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			sub, err := p.parseElement(t)
			if err != nil {
				return nil, err
			}
			items = append(items, sub...)
		case xml.EndElement:
			if t.Name.Local != start.Name.Local {
				return nil, fmt.Errorf("mathml: unexpected </%s> closing <%s>", t.Name.Local, start.Name.Local)
			}
			return items, nil
		}
	}
}

// parseFixedChildren reads exactly n direct child elements from start, each
// returned as its own slice. Used by elements with positional slots:
// <msup> (base, sup), <mfrac> (num, den), <msubsup> (base, sub, sup), etc.
func (p *parser) parseFixedChildren(start xml.StartElement, n int) ([][]math.MathItem, error) {
	out := make([][]math.MathItem, 0, n)
	for {
		tok, err := p.dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			sub, err := p.parseElement(t)
			if err != nil {
				return nil, err
			}
			out = append(out, sub)
		case xml.EndElement:
			if t.Name.Local != start.Name.Local {
				return nil, fmt.Errorf("mathml: unexpected </%s> closing <%s>", t.Name.Local, start.Name.Local)
			}
			if len(out) != n {
				return nil, fmt.Errorf("mathml: <%s> expects %d child elements, got %d", start.Name.Local, n, len(out))
			}
			return out, nil
		}
	}
}

// parseTextElement reads the character content of a leaf element (mi/mn/mo)
// up to and consuming its matching EndElement. Returns the trimmed text.
// Nested elements inside a leaf are an error.
func (p *parser) parseTextElement(start xml.StartElement) (string, error) {
	var sb strings.Builder
	for {
		tok, err := p.dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			sb.Write(t)
		case xml.EndElement:
			if t.Name.Local != start.Name.Local {
				return "", fmt.Errorf("mathml: unexpected </%s> closing <%s>", t.Name.Local, start.Name.Local)
			}
			return strings.TrimSpace(sb.String()), nil
		case xml.StartElement:
			return "", fmt.Errorf("mathml: unexpected child <%s> inside leaf <%s>", t.Name.Local, start.Name.Local)
		}
	}
}

// skipElement consumes the entire content of the current element (and its
// own EndElement) without producing anything. Caller has already consumed
// the StartElement. Used for <annotation> and <annotation-xml> — those carry
// alternate representations (TeX source, content MathML) that the
// presentation pipeline ignores.
func (p *parser) skipElement() error {
	depth := 1
	for depth > 0 {
		tok, err := p.dec.Token()
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

// parseElement dispatches a single child element to its handler. The handler
// is responsible for consuming everything up to (and including) the matching
// EndElement.
func (p *parser) parseElement(start xml.StartElement) ([]math.MathItem, error) {
	switch start.Name.Local {
	case "mi":
		return p.parseMi(start)
	case "mn":
		return p.parseMn(start)
	case "mo":
		return p.parseMo(start)
	case "mrow", "mstyle", "mpadded", "mphantom":
		// transparent grouping containers — emit the children flat
		return p.parseChildren(start)
	case "mfrac":
		return p.parseMfrac(start)
	case "msqrt":
		return p.parseMsqrt(start)
	case "mroot":
		return p.parseMroot(start)
	case "msup":
		return p.parseMsup(start)
	case "msub":
		return p.parseMsub(start)
	case "msubsup":
		return p.parseMsubsup(start)
	case "munder":
		return p.parseMunder(start)
	case "mover":
		return p.parseMover(start)
	case "munderover":
		return p.parseMunderover(start)
	case "semantics":
		// presentation MathML payload is the first child; subsequent
		// <annotation*> children are skipped by parseElement dispatch
		return p.parseChildren(start)
	case "annotation", "annotation-xml":
		return nil, p.skipElement()
	case "mspace", "mtext":
		// not yet supported — read past them, return nothing
		// (mtext should probably emit an Ord-string atom one day)
		return nil, p.skipElement()
	default:
		// unknown element: treat as transparent group rather than erroring,
		// so a stray <merror> or vendor extension doesn't crash the whole
		// parse. Unknown nested elements still report their position.
		return p.parseChildren(start)
	}
}

// parseMi reads an identifier element. MathML default: single-character
// content is rendered italic (variables a, x, …), multi-character is upright
// (function names sin, log, …). The mathvariant attribute overrides.
func (p *parser) parseMi(start xml.StartElement) ([]math.MathItem, error) {
	text, err := p.parseTextElement(start)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, nil
	}
	runes := []rune(text)
	variant := attr(start, "mathvariant")
	if variant == "" {
		if len(runes) == 1 {
			variant = "italic"
		} else {
			variant = "normal"
		}
	}
	items := make([]math.MathItem, 0, len(runes))
	for _, r := range runes {
		mapped := applyVariant(r, variant)
		gid, err := p.gid(mapped)
		if err != nil {
			return nil, err
		}
		items = append(items, math.Ord(gid))
	}
	return items, nil
}

// parseMn reads a numeric literal. MathML default is upright — we leave the
// runes as-is. mathvariant on <mn> is rare and not honoured in v1.
func (p *parser) parseMn(start xml.StartElement) ([]math.MathItem, error) {
	text, err := p.parseTextElement(start)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, nil
	}
	runes := []rune(text)
	items := make([]math.MathItem, 0, len(runes))
	for _, r := range runes {
		gid, err := p.gid(r)
		if err != nil {
			return nil, err
		}
		items = append(items, math.Ord(gid))
	}
	return items, nil
}

// parseMo reads an operator. Each rune is wrapped in an atom of the class
// the operator dictionary assigns (Bin / Rel / Open / Close / Punct / Op /
// Ord). The engine's spacing pass then picks the right inter-atom kern.
func (p *parser) parseMo(start xml.StartElement) ([]math.MathItem, error) {
	text, err := p.parseTextElement(start)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, nil
	}
	runes := []rune(text)
	items := make([]math.MathItem, 0, len(runes))
	for _, r := range runes {
		gid, err := p.gid(r)
		if err != nil {
			return nil, err
		}
		atom := &math.MathAtom{
			Class:   operatorClass(r),
			Nucleus: math.MathField{Glyph: gid},
		}
		items = append(items, atom)
	}
	return items, nil
}

// parseMfrac reads a fraction: 2 children (numerator, denominator). The
// linethickness="0" attribute selects the no-rule variant (binomial).
func (p *parser) parseMfrac(start xml.StartElement) ([]math.MathItem, error) {
	kids, err := p.parseFixedChildren(start, 2)
	if err != nil {
		return nil, err
	}
	f := math.Frac(kids[0], kids[1])
	if lt := attr(start, "linethickness"); lt == "0" || lt == "0pt" || lt == "0em" {
		f.Thickness = -1
	}
	return []math.MathItem{f}, nil
}

// parseMsqrt reads a radical. All children form the body (no separate
// degree). The radical glyph itself is U+221A resolved at parse time.
func (p *parser) parseMsqrt(start xml.StartElement) ([]math.MathItem, error) {
	body, err := p.parseChildren(start)
	if err != nil {
		return nil, err
	}
	radGid, err := p.gid('√')
	if err != nil {
		return nil, err
	}
	return []math.MathItem{math.Sqrt(radGid, body...)}, nil
}

// parseMroot reads an indexed radical: 2 children, base first then index
// (per MathML spec, opposite of what one might guess from "n-th root of x").
func (p *parser) parseMroot(start xml.StartElement) ([]math.MathItem, error) {
	kids, err := p.parseFixedChildren(start, 2)
	if err != nil {
		return nil, err
	}
	radGid, err := p.gid('√')
	if err != nil {
		return nil, err
	}
	return []math.MathItem{math.NRoot(radGid, kids[1], kids[0])}, nil
}

// parseMsup reads a superscript: 2 children (base, sup).
func (p *parser) parseMsup(start xml.StartElement) ([]math.MathItem, error) {
	kids, err := p.parseFixedChildren(start, 2)
	if err != nil {
		return nil, err
	}
	a := atomFromBase(kids[0])
	a.Sup = math.MathField{Sublist: kids[1]}
	return []math.MathItem{a}, nil
}

// parseMsub reads a subscript: 2 children (base, sub).
func (p *parser) parseMsub(start xml.StartElement) ([]math.MathItem, error) {
	kids, err := p.parseFixedChildren(start, 2)
	if err != nil {
		return nil, err
	}
	a := atomFromBase(kids[0])
	a.Sub = math.MathField{Sublist: kids[1]}
	return []math.MathItem{a}, nil
}

// parseMsubsup reads sub+sup together: 3 children (base, sub, sup).
func (p *parser) parseMsubsup(start xml.StartElement) ([]math.MathItem, error) {
	kids, err := p.parseFixedChildren(start, 3)
	if err != nil {
		return nil, err
	}
	a := atomFromBase(kids[0])
	a.Sub = math.MathField{Sublist: kids[1]}
	a.Sup = math.MathField{Sublist: kids[2]}
	return []math.MathItem{a}, nil
}

// parseMunder reads an under-script. accent="true" routes to AccentBottom;
// otherwise it becomes a Sub on the base (engine renders that as a limit
// below big operators in display style, as a regular subscript otherwise).
func (p *parser) parseMunder(start xml.StartElement) ([]math.MathItem, error) {
	kids, err := p.parseFixedChildren(start, 2)
	if err != nil {
		return nil, err
	}
	if attr(start, "accent") == "true" || attr(start, "accentunder") == "true" {
		accGid, ok := singleGid(kids[1])
		if !ok {
			return nil, fmt.Errorf("mathml: <munder accent=\"true\"> child must be a single glyph")
		}
		return []math.MathItem{math.AccentBottom(accGid, kids[0]...)}, nil
	}
	a := atomFromBase(kids[0])
	a.Sub = math.MathField{Sublist: kids[1]}
	return []math.MathItem{a}, nil
}

// parseMover reads an over-script (top accent or top-limit).
func (p *parser) parseMover(start xml.StartElement) ([]math.MathItem, error) {
	kids, err := p.parseFixedChildren(start, 2)
	if err != nil {
		return nil, err
	}
	if attr(start, "accent") == "true" {
		accGid, ok := singleGid(kids[1])
		if !ok {
			return nil, fmt.Errorf("mathml: <mover accent=\"true\"> child must be a single glyph")
		}
		return []math.MathItem{math.AccentTop(accGid, kids[0]...)}, nil
	}
	a := atomFromBase(kids[0])
	a.Sup = math.MathField{Sublist: kids[1]}
	return []math.MathItem{a}, nil
}

// parseMunderover reads under+over together: 3 children (base, under, over).
// Limit-style placement is engine-driven (Op atom in DisplayStyle).
func (p *parser) parseMunderover(start xml.StartElement) ([]math.MathItem, error) {
	kids, err := p.parseFixedChildren(start, 3)
	if err != nil {
		return nil, err
	}
	a := atomFromBase(kids[0])
	a.Sub = math.MathField{Sublist: kids[1]}
	a.Sup = math.MathField{Sublist: kids[2]}
	return []math.MathItem{a}, nil
}

// gid resolves a single rune to its glyph id via the text shaper. Errors
// when the font has no glyph for r — both the "no atoms returned" case and
// the "Shape returned .notdef (GID 0)" case count as missing, per the
// OpenType convention that GID 0 is the placeholder glyph.
func (p *parser) gid(r rune) (ot.GlyphID, error) {
	atoms := p.fnt.Shape(string(r), nil, nil)
	var gid ot.GlyphID
	if len(atoms) > 0 {
		gid = ot.GlyphID(atoms[0].Codepoint)
	}
	if gid == 0 {
		return 0, fmt.Errorf("mathml: font has no glyph for %q (U+%04X)", string(r), r)
	}
	return gid, nil
}

// atomFromBase coerces a child slot (e.g. the base of an msup) into a single
// MathAtom we can hang scripts on. If items is exactly one Atom with no
// existing scripts, it is reused; otherwise the items become an Ord-class
// atom with a sublist nucleus (the engine treats that as a grouped subformula
// for sub/sup attachment).
func atomFromBase(items []math.MathItem) *math.MathAtom {
	if len(items) == 1 {
		if a, ok := items[0].(*math.MathAtom); ok && a.Sub.IsEmpty() && a.Sup.IsEmpty() {
			return a
		}
	}
	return &math.MathAtom{
		Class:   math.ClassOrd,
		Nucleus: math.MathField{Sublist: items},
	}
}

// singleGid extracts a single glyph id from an items slice, for accent
// attachment. Reports (0, false) when items isn't exactly one single-glyph
// Ord atom.
func singleGid(items []math.MathItem) (ot.GlyphID, bool) {
	if len(items) != 1 {
		return 0, false
	}
	a, ok := items[0].(*math.MathAtom)
	if !ok {
		return 0, false
	}
	if a.Nucleus.Glyph == 0 {
		return 0, false
	}
	return a.Nucleus.Glyph, true
}

// attr returns the value of the named attribute on start, or "" if absent.
// Namespace-agnostic: the local name match is enough for MathML in practice.
func attr(start xml.StartElement, name string) string {
	for _, a := range start.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
