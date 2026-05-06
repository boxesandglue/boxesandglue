package frontend

import (
	"io"
	"strings"

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
	l, err := lang.NewFromReader(strings.NewReader(""))
	if err != nil {
		return nil, err
	}
	l.Name = langname
	return l, nil
}

// GetLanguageCached returns the language object for langname using
// fe.Doc.Languages as a per-document cache. Use this from style-resolution
// code that may request the same language many times — pattern parsing for
// known languages still costs work, and creating a no-op for unknown tags
// allocates two empty maps. Cached entries are keyed by the original tag
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
	var r io.Reader
	if vn, ok := languagecodeVarname[newLangname]; ok {
		r = strings.NewReader(hyphenationpatterns[vn])
	} else {
		if split := strings.Split(newLangname, "_"); len(split) > 1 {
			newLangname = split[0]
			if vn, ok := languagecodeVarname[newLangname]; ok {
				r = strings.NewReader(hyphenationpatterns[vn])
			}
		}
	}
	if r == nil {
		bag.Logger.Debug("No hyphenation patterns; using no-op hyphenator", "name", langname)
		return newNoopLang(langname)
	}
	bag.Logger.Debug("Load language from memory", "name", langname)
	l, err := lang.NewFromReader(r)
	if err != nil {
		return nil, err
	}
	l.Name = langname
	return l, nil
}

func insertBreakpoints(l *lang.Lang, word *strings.Builder, wordstart node.Node, fnt *font.Font) {
	cur := wordstart
	if word.Len() > 0 {
		str := word.String()
		word.Reset()
		bp := l.Hyphenate(str)
		for _, step := range bp {
			for i := 0; i <= step-1; i++ {
				cur = cur.Next()
				if cur.Type() == node.TypeKern {
					cur = cur.Next()
				}
			}
			disc := node.NewDisc()
			hyphen := node.NewGlyph()
			if fnt != nil {
				hyphen.Font = fnt
				hyphen.Width = fnt.Hyphenchar.Advance
				hyphen.Components = fnt.Hyphenchar.Components
				hyphen.Codepoint = fnt.Hyphenchar.Codepoint
			}

			disc.Pre = hyphen
			node.InsertBefore(wordstart, cur, disc)
		}
	}
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
