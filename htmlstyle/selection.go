package htmlstyle

import (
	"fmt"
	"regexp"

	"github.com/speedata/boxesandglue/csshtml"
	"github.com/speedata/boxesandglue/frontend"
	"golang.org/x/net/html"
)

var (
	isSpace          = regexp.MustCompile(`^\s*$`)
	reLeadcloseWhtsp = regexp.MustCompile(`^[\s\p{Zs}]+|[\s\p{Zs}]+$`)
	reInsideWS       = regexp.MustCompile(`\n|[\s\p{Zs}]{2,}`) //to match 2 or more whitespace symbols inside a string or NL
)

// Mode is the progression direction of the current HTML element.
type Mode int

func (m Mode) String() string {
	if m == ModeHorizontal {
		return "→"
	}
	return "↓"
}

const (
	// ModeHorizontal represents inline progression direction.
	ModeHorizontal Mode = iota
	// ModeVertical represents block progression direction.
	ModeVertical
)

var preserveWhitespace = []bool{false}

// HTMLItem is a struct which represents a HTML element or a text node.
type HTMLItem struct {
	Typ        html.NodeType
	Data       string
	Dir        Mode
	Attributes map[string]string
	Styles     map[string]string
	Children   []*HTMLItem
}

func (itm *HTMLItem) String() string {
	switch itm.Typ {
	case html.TextNode:
		return fmt.Sprintf("%q", itm.Data)
	case html.ElementNode:
		return fmt.Sprintf("<%s>", itm.Data)
	default:
		return fmt.Sprintf("%s", itm.Data)
	}
}

// DumpElement fills the firstItem with the contents of thisNode. Comments are ignored.
func DumpElement(thisNode *html.Node, direction Mode, firstItem *HTMLItem) {
	newDir := direction
	for {
		if thisNode == nil {
			break
		}

		switch thisNode.Type {
		case html.CommentNode:
			// ignore
		case html.TextNode:
			itm := &HTMLItem{}
			ws := preserveWhitespace[len(preserveWhitespace)-1]
			txt := thisNode.Data
			if !ws {
				if isSpace.MatchString(txt) {
					txt = " "
				}
			}
			if !isSpace.MatchString(txt) {
				if direction == ModeVertical {
					newDir = ModeHorizontal
				}
			}
			if txt != "" {
				if !ws {
					txt = reLeadcloseWhtsp.ReplaceAllString(txt, " ")
					txt = reInsideWS.ReplaceAllString(txt, " ")
				}
			}
			itm.Data = txt
			itm.Typ = html.TextNode
			firstItem.Children = append(firstItem.Children, itm)
		case html.ElementNode:
			ws := preserveWhitespace[len(preserveWhitespace)-1]
			eltname := thisNode.Data

			if eltname == "body" || eltname == "address" || eltname == "article" || eltname == "aside" || eltname == "blockquote" || eltname == "br" || eltname == "canvas" || eltname == "dd" || eltname == "div" || eltname == "dl" || eltname == "dt" || eltname == "fieldset" || eltname == "figcaption" || eltname == "figure" || eltname == "footer" || eltname == "form" || eltname == "h1" || eltname == "h2" || eltname == "h3" || eltname == "h4" || eltname == "h5" || eltname == "h6" || eltname == "header" || eltname == "hr" || eltname == "li" || eltname == "main" || eltname == "nav" || eltname == "noscript" || eltname == "ol" || eltname == "p" || eltname == "pre" || eltname == "section" || eltname == "table" || eltname == "tfoot" || eltname == "thead" || eltname == "tbody" || eltname == "tr" || eltname == "td" || eltname == "th" || eltname == "ul" || eltname == "video" {
				newDir = ModeVertical
			} else if eltname == "b" || eltname == "big" || eltname == "i" || eltname == "small" || eltname == "tt" || eltname == "abbr" || eltname == "acronym" || eltname == "cite" || eltname == "code" || eltname == "dfn" || eltname == "em" || eltname == "kbd" || eltname == "strong" || eltname == "samp" || eltname == "var" || eltname == "a" || eltname == "bdo" || eltname == "img" || eltname == "map" || eltname == "object" || eltname == "q" || eltname == "script" || eltname == "span" || eltname == "sub" || eltname == "sup" || eltname == "button" || eltname == "input" || eltname == "label" || eltname == "select" || eltname == "textarea" {
				newDir = ModeHorizontal
			} else {
				// keep dir
			}

			itm := &HTMLItem{
				Typ:  html.ElementNode,
				Data: thisNode.Data,
				Dir:  newDir,
			}
			firstItem.Children = append(firstItem.Children, itm)
			attributes := thisNode.Attr
			if len(attributes) > 0 {
				itm.Styles, itm.Attributes = csshtml.ResolveAttributes(attributes)
				for key, value := range itm.Styles {
					if key == "white-space" {
						if value == "pre" {
							ws = true
						} else {
							ws = false
						}
					}
				}
			}
			if thisNode.FirstChild != nil {
				preserveWhitespace = append(preserveWhitespace, ws)
				DumpElement(thisNode.FirstChild, newDir, itm)
				preserveWhitespace = preserveWhitespace[:len(preserveWhitespace)-1]
			}
		default:
			fmt.Println(thisNode.Type)
		}
		thisNode = thisNode.NextSibling
	}
}

func HtmlNodeToText(n *html.Node, ss StylesStack, df *frontend.Document) (*frontend.Text, error) {
	h := &HTMLItem{Dir: ModeVertical}
	DumpElement(n, ModeVertical, h)
	return Output(h, ss, df)
}
