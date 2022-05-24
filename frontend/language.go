package frontend

import (
	"fmt"
	"io"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/boxesandglue/backend/lang"
	"github.com/speedata/boxesandglue/backend/node"
)

//go:generate rake genpatterns

var codeVarname = map[string]bool{
	"bg":    true,
	"ca":    true,
	"cs":    true,
	"cy":    true,
	"da":    true,
	"de":    true,
	"el":    true,
	"en":    true,
	"en_gb": true,
	"en_us": true,
	"eo":    true,
	"es":    true,
	"et":    true,
	"eu":    true,
	"fi":    true,
	"fr":    true,
	"ga":    true,
	"gl":    true,
	"grc":   true,
	"gu":    true,
	"hi":    true,
	"hr":    true,
	"hu":    true,
	"hy":    true,
	"id":    true,
	"is":    true,
	"it":    true,
	"ku":    true,
	"kn":    true,
	"lt":    true,
	"ml":    true,
	"lv":    true,
	"nb":    true,
	"nl":    true,
	"nn":    true,
	"no":    true,
	"pl":    true,
	"pt":    true,
	"ro":    true,
	"ru":    true,
	"sk":    true,
	"sl":    true,
	"sc":    true,
	"sv":    true,
	"tr":    true,
	"uk":    true,
}

// GetLanguage returns a language object for the language.
func GetLanguage(langname string) (*lang.Lang, error) {
	newLangname := strings.ToLower(langname)
	var r io.Reader
	if _, ok := codeVarname[newLangname]; ok {
		r = strings.NewReader(hyphenationpatterns[newLangname])
	} else {
		if split := strings.Split(newLangname, "_"); len(split) > 1 {
			newLangname = split[0]
			if _, ok := codeVarname[newLangname]; ok {
				r = strings.NewReader(hyphenationpatterns[newLangname])
			}
		}
	}
	if r == nil {
		return nil, fmt.Errorf("Language %q not found", langname)
	}
	bag.Logger.Debugf("Load language %s from memory", langname)
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
