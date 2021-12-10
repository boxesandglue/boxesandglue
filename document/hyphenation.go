package document

import (
	"strings"

	"github.com/speedata/boxesandglue/backend/font"
	"github.com/speedata/boxesandglue/backend/lang"
	"github.com/speedata/boxesandglue/backend/node"
)

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
func (d *Document) Hyphenate(nodelist node.Node) {
	// hyphenation points should be inserted when a language changes or when
	// the word ends (with a comma or a space for example).
	curlang := d.DefaultLanguage
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
