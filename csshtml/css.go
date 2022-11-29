package csshtml

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/speedata/boxesandglue/frontend"
	"github.com/speedata/css/scanner"
	"golang.org/x/net/html"
)

// Tokenstream is a list of CSS tokens
type Tokenstream []*scanner.Token

type qrule struct {
	Key   Tokenstream
	Value Tokenstream
}

// SBlock is a block with a selector
type SBlock struct {
	Name            string      // only set if this is an at-rule
	ComponentValues Tokenstream // the "selector"
	ChildAtRules    []*SBlock   // the block's at-rules, if any
	Blocks          []*SBlock   // the at-rule's blocks, if any
	Rules           []qrule     // the key-value pairs
}

// Page defines a page.
type Page struct {
	pagearea     map[string][]qrule
	Attributes   []html.Attribute
	Papersize    string
	MarginLeft   string
	MarginRight  string
	MarginTop    string
	MarginBottom string
}

// CSS has all the information
type CSS struct {
	FrontendDocument *frontend.Document
	dirstack         []string
	document         *goquery.Document
	Stylesheet       []SBlock
	Pages            map[string]Page
}

// CSSdefaults contains browser-like styling of some elements.
var CSSdefaults = `
li              { display: list-item; padding-inline-start: 40pt; }
head            { display: none }
table           { display: table }
tr              { display: table-row }
thead           { display: table-header-group }
tbody           { display: table-row-group }
tfoot           { display: table-footer-group }
td, th          { display: table-cell }
caption         { display: table-caption }
th              { font-weight: bold; text-align: center }
caption         { text-align: center }
body            { margin: 0pt; font-family: sans-serif; font-size: 10pt; line-height: 1.2; hyphens: auto; font-weight: normal; }
h1              { font-size: 2em; margin: .67em 0 }
h2              { font-size: 1.5em; margin: .75em 0 }
h3              { font-size: 1.17em; margin: .83em 0 }
h4, p,
blockquote, ul,
fieldset, form,
ol, dl, dir,
h5              { font-size: 1em; margin: 1.5em 0 }
h6              { font-size: .75em; margin: 1.67em 0 }
h1, h2, h3, h4,
h5, h6, b,
strong          { font-weight: bold }
blockquote      { margin-left: 40px; margin-right: 40px }
i, cite, em,
var, address    { font-style: italic }
pre, tt, code,
kbd, samp       { font-family: monospace }
pre             { white-space: pre; margin: 1em 0px; }
button, textarea,
input, select   { display: inline-block }
big             { font-size: 1.17em }
small, sub, sup { font-size: .83em }
sub             { vertical-align: sub }
sup             { vertical-align: super }
table           { border-spacing: 2pt; }
thead, tbody,
tfoot           { vertical-align: middle }
td, th, tr      { vertical-align: inherit }
s, strike, del  { text-decoration: line-through }
hr              { border: 1px inset }
ol, ul, dir, dd { padding-left: 20pt }
ol              { list-style-type: decimal }
ul              { list-style-type: disc }
ol ul, ul ol,
ul ul, ol ol    { margin-top: 0; margin-bottom: 0 }
u, ins          { text-decoration: underline }
center          { text-align: center }
`

// :link           { text-decoration: underline }

// Return the position of the matching closing brace "}"
func findClosingBrace(toks Tokenstream) int {
	level := 1
	for i, t := range toks {
		if t.Type == scanner.Delim {
			switch t.Value {
			case "{":
				level++
			case "}":
				level--
				if level == 0 {
					return i + 1
				}
			}
		}
	}
	return len(toks)
}

// fixupComponentValues changes DELIM[.] + IDENT[foo] to IDENT[.foo]
func fixupComponentValues(toks Tokenstream) Tokenstream {
	toks = trimSpace(toks)
	var combineNext bool
	for i := 0; i < len(toks)-1; i++ {
		combineNext = false
		if toks[i].Type == scanner.Delim && toks[i].Value == "." && toks[i+1].Type == scanner.Ident {
			toks[i+1].Value = "." + toks[i+1].Value
			combineNext = true
		} else if toks[i].Type == scanner.Delim && toks[i].Value == ":" && toks[i+1].Type == scanner.Ident {
			toks[i+1].Value = ":" + toks[i+1].Value
			combineNext = true
		} else if toks[i].Type == scanner.Hash {
			toks[i].Value = "#" + toks[i].Value
		}

		if combineNext {
			toks = append(toks[:i], toks[i+1:]...)
			i++
		}
	}
	return toks
}

func trimSpace(toks Tokenstream) Tokenstream {
	i := 0
	for {
		if i == len(toks) {
			break
		}
		if t := toks[i]; t.Type == scanner.S {
			i++
		} else {
			break
		}
	}
	toks = toks[i:]
	return toks
}

// ConsumeBlock get the contents of a block. The name (in case of an at-rule)
// and the selector will be added later on
func ConsumeBlock(toks Tokenstream, inblock bool) SBlock {
	// This is the whole block between the opening { and closing }
	if len(toks) <= 1 {
		return SBlock{}
	}
	b := SBlock{}
	i := 0
	// we might start with whitespace, skip it
	for {
		if i == len(toks) {
			break
		}
		if t := toks[i]; t.Type == scanner.S {
			i++
		} else {
			break
		}
	}
	start := i
	colon := 0
	for {
		if i == len(toks) {
			break
		}
		// There are only two cases: a key-value rule or something with
		// curly braces
		if t := toks[i]; t.Type == scanner.Delim {
			switch t.Value {
			case ":":
				if inblock {
					colon = i
				}
			case ";":
				key := trimSpace(toks[start:colon])
				value := trimSpace(toks[colon+1 : i])
				q := qrule{Key: key, Value: value}
				b.Rules = append(b.Rules, q)
				colon = 0
				start = i + 1
			case "{":
				var nb SBlock
				// l is the length of the sub block
				l := findClosingBrace(toks[i+1:])
				if l == 1 {
					break
				}
				subblock := toks[i+1 : i+l]
				// subblock is without the enclosing curly braces
				starttok := toks[start]
				startsWithATKeyword := starttok.Type == scanner.AtKeyword && (starttok.Value == "media" || starttok.Value == "supports")
				nb = ConsumeBlock(subblock, !startsWithATKeyword)
				if toks[start].Type == scanner.AtKeyword {
					nb.Name = toks[start].Value
					b.ChildAtRules = append(b.ChildAtRules, &nb)
					nb.ComponentValues = fixupComponentValues(toks[start+1 : i])
				} else {
					b.Blocks = append(b.Blocks, &nb)
					nb.ComponentValues = fixupComponentValues(toks[start:i])
				}

				i = i + l
				start = i + 1
				// skip over whitespace
				if start < len(toks) && toks[start].Type == scanner.S {
					start++
					i++
				}
			case ",", ")":
				// ignore
			default:
				w("unknown delimiter", t.Value)
			}
		}
		i++
		if i == len(toks) {
			break
		}
	}
	if colon > 0 {
		b.Rules = append(b.Rules, qrule{Key: toks[start:colon], Value: toks[colon+1:]})
	}
	return b
}

func (c *CSS) doFontFace(ff []qrule) {
	var fontweight frontend.FontWeight = 400
	var fontstyle frontend.FontStyle = frontend.FontStyleNormal
	var fontfamily string
	var fontsource frontend.FontSource
	for _, rule := range ff {
		key := strings.TrimSpace(rule.Key.String())
		value := strings.TrimSpace(rule.Value.String())
		switch key {
		case "font-family":
			fontfamily = value
		case "font-style":
			switch value {
			case "normal":
				fontstyle = frontend.FontStyleNormal
			case "italic":
				fontstyle = frontend.FontStyleItalic
			case "oblique":
				fontstyle = frontend.FontStyleOblique
			}
		case "font-weight":
			if i, err := strconv.Atoi(value); err == nil {
				fontweight = frontend.FontWeight(i)
			} else {
				switch strings.ToLower(value) {
				case "thin", "hairline":
					fontweight = 100
				case "extra light", "ultra light":
					fontweight = 200
				case "light":
					fontweight = 300
				case "normal":
					fontweight = 400
				case "medium":
					fontweight = 500
				case "semi bold", "demi bold":
					fontweight = 600
				case "bold":
					fontweight = 700
				case "extra bold", "ultra bold":
					fontweight = 800
				case "black", "heavy":
					fontweight = 900
				}
			}
		case "src":
			for _, v := range rule.Value {
				if v.Type == scanner.URI {
					fontsource.Source = c.FrontendDocument.FindFile(v.Value)
				} else if v.Type == scanner.Local {
					fontsource.Source = c.FrontendDocument.FindFile(v.Value)
				}
			}
		case "font-feature-settings":
			settingOn := true
			r := regexp.MustCompile(`(on|off|\d+)\s*$`)
			if r.MatchString(value) {
				idx := r.FindAllStringIndex(value, -1)
				if idx != nil {
					sw := value[idx[0][0]:idx[0][1]]
					if sw == "on" {
						// keep on
					} else if sw == "off" || sw == "0" {
						settingOn = false
					} else if sw >= "1" && sw <= "9" {
						// keep on
					}
				}
				value = value[:idx[0][0]]
			}
			var prefix string
			for _, v := range strings.Split(value, ",") {
				if settingOn {
					prefix = "+"
				} else {
					prefix = "-"
				}
				fontsource.FontFeatures = append(fontsource.FontFeatures, prefix+strings.TrimSpace(v))
			}
		default:
			fmt.Println("unhandled font setting", key)
		}
	}
	fam := c.FrontendDocument.FindFontFamily(fontfamily)
	if fam == nil {
		fam = c.FrontendDocument.NewFontFamily(fontfamily)
	}
	fam.AddMember(&fontsource, fontweight, fontstyle)
}

func (c *CSS) doPage(block *SBlock) {
	selector := block.ComponentValues.String()
	pg := c.Pages[selector]
	if pg.pagearea == nil {
		pg.pagearea = make(map[string][]qrule)
	}
	for _, v := range block.Rules {
		switch v.Key.String() {
		case "size":
			pg.Papersize = v.Value.String()
		case "margin":
			fv := getFourValues(v.Value.String())
			pg.MarginTop = fv["top"]
			pg.MarginBottom = fv["bottom"]
			pg.MarginLeft = fv["left"]
			pg.MarginRight = fv["right"]
		default:
			a := html.Attribute{Key: v.Key.String(), Val: stringValue(v.Value)}
			pg.Attributes = append(pg.Attributes, a)
		}
	}
	for _, rule := range block.ChildAtRules {
		pg.pagearea[rule.Name] = rule.Rules
	}
	c.Pages[selector] = pg
}

func (c *CSS) processAtRules() {
	c.Pages = make(map[string]Page)
	for _, stylesheet := range c.Stylesheet {
		for _, atrule := range stylesheet.ChildAtRules {
			switch atrule.Name {
			case "font-face":
				c.doFontFace(atrule.Rules)
			case "page":
				c.doPage(atrule)
			default:
				fmt.Println("unknown at rule", atrule)
			}
		}

	}
}

// Findfunc is called when loading a resource
type Findfunc func(string) (string, error)

// NewCSSParser returns a new CSS object
func NewCSSParser() *CSS {
	return &CSS{}
}

// NewCSSParserWithDefaults returns a new CSS object with the default stylesheet included.
func NewCSSParserWithDefaults() *CSS {
	c := &CSS{}
	c.Stylesheet = append(c.Stylesheet, ConsumeBlock(ParseCSSString(CSSdefaults), false))
	return c
}

// AddCSSText parses CSS text and saves the rules for later.
func (c *CSS) AddCSSText(fragment string) error {
	toks, err := c.ParseCSSString(fragment)
	if err != nil {
		return err
	}
	c.Stylesheet = append(c.Stylesheet, ConsumeBlock(toks, false))
	c.processAtRules()
	return nil
}

// ParseHTMLFragment takes the HTML text and the CSS text and goquery selection.
func (c *CSS) ParseHTMLFragment(htmltext, csstext string) (*goquery.Selection, error) {
	c.Stylesheet = append(c.Stylesheet, ConsumeBlock(ParseCSSString(CSSdefaults), false))
	c.Stylesheet = append(c.Stylesheet, ConsumeBlock(ParseCSSString(csstext), false))
	err := c.ReadHTMLChunk(htmltext)
	if err != nil {
		return nil, err
	}
	c.processAtRules()
	return c.ApplyCSS()
}
