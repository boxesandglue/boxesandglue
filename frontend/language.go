package frontend

import (
	"fmt"
	"io"
	"strings"

	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/boxesandglue/backend/lang"
	"github.com/speedata/boxesandglue/backend/node"
	"golang.org/x/exp/slog"
)

//go:generate rake genpatterns

var codeVarname = map[string]string{
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

// GetLanguage returns a language object for the language.
func GetLanguage(langname string) (*lang.Lang, error) {
	newLangname := strings.ToLower(langname)
	var r io.Reader
	if vn, ok := codeVarname[newLangname]; ok {
		r = strings.NewReader(hyphenationpatterns[vn])
	} else {
		if split := strings.Split(newLangname, "_"); len(split) > 1 {
			newLangname = split[0]
			if vn, ok := codeVarname[newLangname]; ok {
				r = strings.NewReader(hyphenationpatterns[vn])
			}
		}
	}
	if r == nil {
		return nil, fmt.Errorf("Language %q not found", langname)
	}
	slog.Debug("Load language from memory", "name", langname)
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
