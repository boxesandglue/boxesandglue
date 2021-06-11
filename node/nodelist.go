package node

import (
	"container/list"
	"strings"

	"github.com/speedata/texperiments/lang"
)

// Nodelist contains a list of nodes
type Nodelist struct {
	List *list.List
}

// NewNodelist creates an empty node list
func NewNodelist() *Nodelist {
	n := &Nodelist{}
	n.List = list.New()
	return n
}

// AppendNode appends the node val to the end of the node list nl.
func (nl *Nodelist) AppendNode(val interface{}) {
	nl.List.PushBack(val)
}

func insertBreakpoints(l *lang.Lang, word *strings.Builder, nodelist *Nodelist, wordstart *list.Element) {
	cur := wordstart
	if word.Len() > 0 {
		str := word.String()
		word.Reset()
		bp := l.Hyphenate(str)
		for _, step := range bp {
			for i := 0; i <= step-1; i++ {
				cur = cur.Next()
			}
			disc := NewDiscWithContents(&Disc{})
			cur = nodelist.List.InsertBefore(disc, cur)
			cur = cur.Next()
		}
	}
}

// Hyphenate inserts hyphenation points in to the list
func Hyphenate(nodelist *Nodelist) {
	// hyphenation points should be inserted when a language changes or when
	// the word ends (with a comma or a space for example).
	nl := nodelist.List
	var curlang *lang.Lang
	var wordboundary bool
	var prevhyphenate bool
	var wordstart *list.Element
	var b strings.Builder
	e := nl.Front()
	for {
		switch v := e.Value.(type) {
		case *Glyph:
			if prevhyphenate != v.Hyphenate {
				wordboundary = true
			}

			if wordboundary {
				insertBreakpoints(curlang, &b, nodelist, wordstart)
				wordstart = e
			}

			prevhyphenate = v.Hyphenate
			wordboundary = false
			if v.Hyphenate {
				if v.Components != "" {
					b.WriteString(v.Components)
				} else {
					b.WriteRune(rune(v.GlyphID))
				}
			}
		case *Lang:
			curlang = v.Lang
			wordboundary = true
		default:
			wordboundary = true

		}
		e = e.Next()
		if e == nil {
			break
		}
	}
	insertBreakpoints(curlang, &b, nodelist, wordstart)
}
