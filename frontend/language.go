package frontend

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/boxesandglue/boxesandglue/backend/bag"
	"github.com/boxesandglue/boxesandglue/backend/font"
	"github.com/boxesandglue/boxesandglue/backend/lang"
	"github.com/boxesandglue/boxesandglue/backend/node"
)

//go:generate rake genpatterns

var languagecodeVarname = map[string]string{
	"bg":    "bg",
	"ca":    "ca",
	"cs":    "cs",
	"cy":    "cy",
	"da":    "da",
	"de":    "de",
	"el":    "el",
	"en":    "en",
	"en_gb": "engb",
	"en_us": "enus",
	"eo":    "eo",
	"es":    "es",
	"et":    "et",
	"eu":    "eu",
	"fi":    "fi",
	"fr":    "fr",
	"ga":    "ga",
	"gl":    "gl",
	"grc":   "grc",
	"gu":    "gu",
	"hi":    "hi",
	"hr":    "hr",
	"hu":    "hu",
	"hy":    "hy",
	"id":    "id",
	"is":    "is",
	"it":    "it",
	"ku":    "ku",
	"kn":    "kn",
	"lt":    "lt",
	"ml":    "ml",
	"lv":    "lv",
	"nb":    "nb",
	"nl":    "nl",
	"nn":    "nn",
	"no":    "no",
	"pl":    "pl",
	"pt":    "pt",
	"ro":    "ro",
	"ru":    "ru",
	"sk":    "sk",
	"sl":    "sl",
	"sc":    "sc",
	"sv":    "sv",
	"tr":    "tr",
	"uk":    "uk",
}

// IsHyphenationSupported reports whether hyphenation patterns are available
// for the given BCP47 language tag. The check is conservative: only the
// canonical codes listed in languagecodeVarname (and their primary subtag
// fallbacks) qualify.
func IsHyphenationSupported(langname string) bool {
	newLangname := strings.ToLower(langname)
	if _, ok := languagecodeVarname[newLangname]; ok {
		return true
	}
	if split := strings.Split(newLangname, "_"); len(split) > 1 {
		if _, ok := languagecodeVarname[split[0]]; ok {
			return true
		}
	}
	return false
}

// newNoopLang returns a language object whose Hyphenate() never produces any
// breakpoints. Used for languages without TeX patterns (Arabic, Hebrew, CJK,
// …) and for runs whose CSS `hyphens` property forbids automatic hyphenation.
// The empty pattern set is the natural no-op: doHyphenate finds no matching
// substring patterns, so no break positions are emitted.
func newNoopLang(langname string) (*lang.Lang, error) {
	l, err := sharedLang("")
	if err != nil {
		return nil, err
	}
	l.Name = langname
	return l, nil
}

var (
	sharedLangsMu sync.Mutex
	// sharedLangs holds each pattern set parsed once, by its name in
	// hyphenationpatterns, "" being the empty set.
	sharedLangs = map[string]*lang.Lang{}
)

// sharedLang returns a Lang of the caller's own over the patterns named vn,
// parsing them on their first use.
func sharedLang(vn string) (*lang.Lang, error) {
	sharedLangsMu.Lock()
	defer sharedLangsMu.Unlock()
	l, ok := sharedLangs[vn]
	if !ok {
		var err error
		if l, err = lang.NewFromReader(strings.NewReader(hyphenationpatterns[vn])); err != nil {
			return nil, err
		}
		sharedLangs[vn] = l
	}
	return l.Copy(), nil
}

// GetLanguageCached returns the language object for langname using
// fe.Doc.Languages as a per-document cache. Use this from style-resolution
// code that may request the same language many times, so the document holds
// one language object per tag. Cached entries are keyed by the original tag
// (case preserved); GetLanguage normalisation still applies on miss.
func (fe *Document) GetLanguageCached(langname string) (*lang.Lang, error) {
	if l, ok := fe.Doc.Languages[langname]; ok {
		return l, nil
	}
	l, err := GetLanguage(langname)
	if err != nil {
		return nil, err
	}
	if fe.Doc.Languages == nil {
		fe.Doc.Languages = make(map[string]*lang.Lang)
	}
	fe.Doc.Languages[langname] = l
	return l, nil
}

// GetLanguage returns a language object for the given BCP47 tag. Tags can be
// the primary subtag ("de") or a primary+region composite ("de_DE", "en_US").
//
// For tags without TeX hyphenation patterns the function returns a no-op
// language (see newNoopLang). This matches CSS Text 3 §6 hyphenation: a UA
// without matching patterns must not hyphenate. Callers that need to verify
// pattern availability should consult IsHyphenationSupported first.
func GetLanguage(langname string) (*lang.Lang, error) {
	newLangname := strings.ToLower(langname)
	vn, ok := languagecodeVarname[newLangname]
	if !ok {
		if split := strings.Split(newLangname, "_"); len(split) > 1 {
			newLangname = split[0]
			vn, ok = languagecodeVarname[newLangname]
		}
	}
	if !ok {
		bag.Logger.Debug("No hyphenation patterns; using no-op hyphenator", "name", langname)
		return newNoopLang(langname)
	}
	bag.Logger.Debug("Load language from memory", "name", langname)
	l, err := sharedLang(vn)
	if err != nil {
		return nil, err
	}
	l.Name = langname
	return l, nil
}

// glyphChars is the number of characters a glyph sets: those of its
// Components, more than one for a ligature, or its one codepoint.
func glyphChars(g *node.Glyph) int {
	if g.Components == "" {
		return 1
	}
	return utf8.RuneCountInString(g.Components)
}

func insertBreakpoints(l *lang.Lang, word *strings.Builder, wordstart node.Node, fnt *font.Font) {
	if word.Len() == 0 {
		return
	}
	str := word.String()
	word.Reset()
	// The points count characters, and a ligature glyph sets several of
	// them, so the walk counts characters too. chars is the number of
	// characters up to cur.
	cur := wordstart
	chars, at := 0, 0
	var before, split node.Node
	for _, step := range l.Hyphenate(str) {
		at += step
		for cur != nil && chars < at {
			if g, ok := cur.(*node.Glyph); ok {
				chars += glyphChars(g)
				before = cur
			}
			cur = cur.Next()
		}
		if cur == nil {
			break
		}
		if chars != at {
			// The point is inside the ligature before: a Disc sets it
			// apart if the word breaks there, once for each ligature.
			if g, ok := before.(*node.Glyph); ok && before != split {
				splitLigature(g, glyphChars(g)-(chars-at), fnt)
				split = before
			}
			continue
		}
		if cur.Type() == node.TypeKern {
			cur = cur.Next()
		}
		disc := node.NewDisc()
		disc.Pre = newHyphen(fnt, before)
		node.InsertBefore(wordstart, cur, disc)
	}
}

// newHyphen is the hyphen that ends a line after the glyph before.
func newHyphen(fnt *font.Font, before node.Node) *node.Glyph {
	hyphen := node.NewGlyph()
	if fnt != nil {
		hyphen.Font = fnt
		hyphen.Width = fnt.Hyphenchar.Advance
		hyphen.Components = fnt.Hyphenchar.Components
		hyphen.Codepoint = fnt.Hyphenchar.Codepoint
	}
	// The hyphen ends the line after the glyph before the break, so it
	// takes that glyph's scale when a word spans two runs.
	if g, ok := before.(*node.Glyph); ok && g.HorizontalScale != 0 && g.HorizontalScale != 1 {
		hyphen.Width = bag.MultiplyFloat(hyphen.Width, g.HorizontalScale)
		hyphen.HorizontalScale = g.HorizontalScale
	}
	return hyphen
}

// splitLigature puts a Disc before the ligature g that breaks the word after
// the first k characters of g, as TeX does (bag#90): the line ends with
// those characters and a hyphen, the next one starts with the rest, and g is
// dropped. Without a break g stays. Both parts are shaped anew the way g's
// run was, so they can be ligatures of their own.
func splitLigature(g *node.Glyph, k int, fnt *font.Font) {
	// A Disc before the first node of the list would need a new head.
	if g.Prev() == nil || g.Font == nil {
		return
	}
	r := []rune(g.Components)
	// Pre is followed by the hyphen, so it takes the letter spacing after
	// its last glyph. After Post the letter spacing of g follows in the
	// list.
	pre := shapeLike(g, string(r[:k]), true)
	post := shapeLike(g, string(r[k:]), false)
	if pre == nil || post == nil {
		return
	}
	tail := node.Tail(pre)
	node.InsertAfter(pre, tail, newHyphen(fnt, tail))
	disc := node.NewDisc()
	disc.Pre = pre
	disc.Post = post
	disc.ReplaceCount = 1
	disc.SetBidiLevel(g.BidiLevel())
	node.InsertBefore(g.Prev(), g, disc)
}

// shapeLike shapes text in the font of g with the features, variations and
// letter spacing of its run and returns the glyphs, which take the scale,
// offsets and attributes of g. spaceAfter adds the letter spacing after the
// last glyph too.
func shapeLike(g *node.Glyph, text string, spaceAfter bool) node.Node {
	scaled := func(wd bag.ScaledPoint) bag.ScaledPoint {
		if g.HorizontalScale == 0 || g.HorizontalScale == 1 {
			return wd
		}
		return bag.MultiplyFloat(wd, g.HorizontalScale)
	}
	sh := g.Shaping
	if sh == nil {
		sh = &font.Shaping{}
	}
	var head, cur node.Node
	add := func(n node.Node) {
		n.SetBidiLevel(g.BidiLevel())
		head = node.InsertAfter(head, cur, n)
		cur = n
	}
	atoms := g.Font.Shape(text, sh.Features, sh.Variations)
	for i, a := range atoms {
		n := g.Copy().(*node.Glyph)
		n.Codepoint = a.Codepoint
		n.Components = a.Components
		n.Width = scaled(a.Advance)
		n.Height = a.Height
		n.Depth = a.Depth
		n.XOffset = scaled(a.XOffset)
		add(n)
		last := i == len(atoms)-1
		if a.Kernafter != 0 && !last {
			k := node.NewKern()
			k.Kern = scaled(a.Kernafter)
			add(k)
		}
		if sh.LetterSpacing != 0 && (!last || spaceAfter) {
			k := node.NewKern()
			k.Kern = sh.LetterSpacing
			add(k)
		}
	}
	return head
}

// Hyphenate inserts hyphenation points in to the list.
func Hyphenate(nodelist node.Node, defaultLang *lang.Lang) {
	// hyphenation points should be inserted when a language changes or when
	// the word ends (with a comma or a space for example).
	curlang := defaultLang
	var wordboundary bool

	var wordstart node.Node
	var b strings.Builder
	var curfont *font.Font

	for e := nodelist; e != nil; e = e.Next() {
		switch v := e.(type) {
		case *node.Glyph:
			curfont = v.Font
			if wordstart == nil && v.Hyphenate {
				b.Reset()
				wordstart = e
			}
			if wordstart != nil && !v.Hyphenate {
				wordboundary = true
			}

			if v.Hyphenate {
				if v.Components != "" {
					b.WriteString(v.Components)
				} else {
					b.WriteRune(rune(v.Codepoint))
				}
			}
		case *node.Glue:
			wordboundary = true
		case *node.Lang:
			curlang = v.Lang
			wordboundary = true
		case *node.Kern:
			wordboundary = false
		default:
			wordboundary = true

		}
		if wordboundary {
			insertBreakpoints(curlang, &b, wordstart, curfont)
			wordstart = nil
			wordboundary = false
		}
	}
	if wordstart != nil {
		insertBreakpoints(curlang, &b, wordstart, curfont)
	}
}
