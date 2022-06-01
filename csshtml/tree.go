package csshtml

import (
	"io"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"
	"github.com/speedata/css/scanner"
)

var (
	level int
	out   io.Writer

	dimen              = regexp.MustCompile(`px|mm|cm|in|pt|pc|ch|em|ex|lh|rem|0`)
	zeroDimen          = regexp.MustCompile(`^0+(px|mm|cm|in|pt|pc|ch|em|ex|lh|rem)?`)
	style              = regexp.MustCompile(`^none|hidden|dotted|dashed|solid|double|groove|ridge|inset|outset$`)
	toprightbottomleft = [...]string{"top", "right", "bottom", "left"}
)

func normalizespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func stringValue(toks Tokenstream) string {
	ret := []string{}
	negative := false
	prevNegative := false
	for _, tok := range toks {
		prevNegative = negative
		negative = false
		switch tok.Type {
		case scanner.Ident, scanner.String:
			ret = append(ret, tok.Value)
		case scanner.Number, scanner.Dimension:
			if prevNegative {
				ret = append(ret, "-"+tok.Value)
			} else {
				ret = append(ret, tok.Value)
			}
		case scanner.Percentage:
			ret = append(ret, tok.Value+"%")
		case scanner.Hash:
			ret = append(ret, "#"+tok.Value)
		case scanner.Function:
			ret = append(ret, tok.Value+"(")
		case scanner.S:
			// ret = append(ret, " ")
		case scanner.Delim:
			switch tok.Value {
			case ";":
				// ignore
			case ",", ")":
				ret = append(ret, tok.Value)
			case "-":
				negative = true
			default:
				w("unhandled delimiter", tok)
			}
		case scanner.URI:
			ret = append(ret, "url("+tok.Value+")")
		default:
			w("unhandled token", tok)
		}
	}
	return strings.Join(ret, " ")
}

// Recurse through the HTML tree and resolve the style attribute
func resolveStyle(i int, sel *goquery.Selection) {
	a, b := sel.Attr("style")
	if b {
		var tokens Tokenstream

		s := scanner.New(a)
		for {
			token := s.Next()
			if token.Type == scanner.EOF || token.Type == scanner.Error {
				break
			}
			switch token.Type {
			case scanner.Comment:
				// ignore
			default:
				tokens = append(tokens, token)
			}
		}
		block := ConsumeBlock(tokens, true)
		for _, rule := range block.Rules {
			sel.SetAttr("!"+stringValue(rule.Key), stringValue(rule.Value))

		}
		sel.RemoveAttr("style")
	}
	sel.Children().Each(resolveStyle)
}

func isDimension(str string) (bool, string) {
	switch str {
	case "thick":
		return true, "2pt"
	case "medium":
		return true, "1pt"
	case "thin":
		return true, "0.5pt"
	}
	return dimen.MatchString(str), str
}
func isBorderStyle(str string) (bool, string) {
	return style.MatchString(str), str
}

// getFourValues fills all four values for top, bottom, left and right from one
// to four values in margin/padding etc.
func getFourValues(str string) map[string]string {
	fields := strings.Fields(str)
	fourvalues := make(map[string]string)
	switch len(fields) {
	case 1:
		fourvalues["top"] = fields[0]
		fourvalues["bottom"] = fields[0]
		fourvalues["left"] = fields[0]
		fourvalues["right"] = fields[0]
	case 2:
		fourvalues["top"] = fields[0]
		fourvalues["bottom"] = fields[0]
		fourvalues["left"] = fields[1]
		fourvalues["right"] = fields[1]
	case 3:
		fourvalues["top"] = fields[0]
		fourvalues["left"] = fields[1]
		fourvalues["right"] = fields[1]
		fourvalues["bottom"] = fields[2]
	case 4:
		fourvalues["top"] = fields[0]
		fourvalues["right"] = fields[1]
		fourvalues["bottom"] = fields[2]
		fourvalues["left"] = fields[3]
	}

	return fourvalues
}

// ResolveAttributes returns the resolved styles and the attributes of the node.
// It changes "margin: 1cm;" into "margin-left: 1cm; margin-right: 1cm; ...".
func ResolveAttributes(attrs []html.Attribute) (map[string]string, map[string]string) {
	resolved := make(map[string]string)
	attributes := make(map[string]string)
	// attribute resolving must be in order of appearance.
	// For example the following border-left-style has no effect:
	//    border-left-style: dotted;
	//    border-left: thick green;
	// because the second line overrides the first line (style defaults to "none")
	for _, attr := range attrs {
		key := attr.Key
		if !strings.HasPrefix(key, "!") {
			attributes[key] = attr.Val
			continue
		}
		key = strings.TrimPrefix(key, "!")

		switch key {
		case "margin":
			values := getFourValues(attr.Val)
			for _, margin := range toprightbottomleft {
				resolved["margin-"+margin] = values[margin]
			}
		case "list-style":
			for _, part := range strings.Split(attr.Val, " ") {
				switch part {
				case "inside", "outside":
					resolved["list-style-position"] = part
				default:
					if strings.HasPrefix(part, "url") {
						resolved["list-style-image"] = part
					} else {
						resolved["list-style-type"] = part
					}
				}
			}
		case "padding":
			values := getFourValues(attr.Val)
			for _, padding := range toprightbottomleft {
				resolved["padding-"+padding] = values[padding]
			}
		case "border":
			for _, loc := range toprightbottomleft {
				resolved["border-"+loc+"-style"] = "none"
				resolved["border-"+loc+"-width"] = "1pt"
				resolved["border-"+loc+"-color"] = "currentcolor"
			}

			// This does not work with colors such as rgb(1 , 2 , 4) which have spaces in them
			for _, part := range strings.Split(attr.Val, " ") {
				for _, border := range toprightbottomleft {
					if ok, str := isBorderStyle(part); ok {
						resolved["border-"+border+"-style"] = str
					} else if ok, str := isDimension(part); ok {
						resolved["border-"+border+"-width"] = str
					} else {
						resolved["border-"+border+"-color"] = part
					}
				}
			}
		case "border-radius":
			for _, lr := range []string{"left", "right"} {
				for _, tb := range []string{"top", "bottom"} {
					resolved["border-"+tb+"-"+lr+"-radius"] = attr.Val
				}
			}
		case "border-top", "border-right", "border-bottom", "border-left":
			resolved[attr.Key+"-width"] = "1pt"
			resolved[attr.Key+"-style"] = "none"
			resolved[attr.Key+"-color"] = "currentcolor"

			for _, part := range strings.Split(attr.Val, " ") {
				if ok, str := isDimension(part); ok {
					resolved[attr.Key+"-width"] = str
				} else if ok, str := isBorderStyle(part); ok {
					resolved[attr.Key+"-style"] = str
				} else {
					resolved[attr.Key+"-color"] = str
				}
			}
		case "border-color":
			values := getFourValues(attr.Val)
			for _, loc := range toprightbottomleft {
				resolved["border-"+loc+"-color"] = values[loc]
			}
		case "border-style":
			values := getFourValues(attr.Val)
			for _, loc := range toprightbottomleft {
				resolved["border-"+loc+"-style"] = values[loc]
			}
		case "border-width":
			values := getFourValues(attr.Val)
			for _, loc := range toprightbottomleft {
				resolved["border-"+loc+"-width"] = values[loc]
			}
		case "font":
			fontstyle := "normal"
			fontweight := "normal"

			/*
				it must include values for:
					<font-size>
					<font-family>
				it may optionally include values for:
					<font-style>
					<font-variant>
					<font-weight>
					<font-stretch>
					<line-height>
				* font-style, font-variant and font-weight must precede font-size
				* font-variant may only specify the values defined in CSS 2.1, that is normal and small-caps
				* font-stretch may only be a single keyword value.
				* line-height must immediately follow font-size, preceded by "/", like this: "16px/3"
				* font-family must be the last value specified.
			*/
			val := attr.Val
			fields := strings.Fields(val)
			l := len(fields)
			for idx, field := range fields {
				if idx > l-3 {
					if dimen.MatchString(field) || strings.Contains(field, "%") {
						resolved["font-size"] = field
					} else {
						resolved["font-name"] = field
					}
				}
			}
			resolved["font-style"] = fontstyle
			resolved["font-weight"] = fontweight
		// font-stretch: ultra-condensed; extra-condensed; condensed; semi-condensed; normal; semi-expanded; expanded; extra-expanded; ultra-expanded;
		case "text-decoration":
			for _, part := range strings.Split(attr.Val, " ") {
				if part == "none" || part == "underline" || part == "overline" || part == "line-through" {
					resolved["text-decoration-line"] = part
				} else if part == "solid" || part == "double" || part == "dotted" || part == "dashed" || part == "wavy" {
					resolved["text-decoration-style"] = part
				}
			}

		case "background":
			// background-clip, background-color, background-image, background-origin, background-position, background-repeat, background-size, and background-attachment
			for _, part := range strings.Split(attr.Val, " ") {
				resolved["background-color"] = part
			}
		default:
			resolved[key] = attr.Val
		}
	}
	if str, ok := resolved["text-decoration-line"]; ok && str != "none" {
		resolved["text-decoration-style"] = "solid"
	}
	return resolved, attributes
}

// ApplyCSS resolves CSS rules in the DOM.
func (c *CSS) ApplyCSS() (*goquery.Selection, error) {
	type selRule struct {
		selector cascadia.Sel
		rule     []qrule
	}

	rules := map[int][]selRule{}

	c.document.Each(resolveStyle)
	for _, stylesheet := range c.Stylesheet {
		for _, block := range stylesheet.Blocks {
			selector := block.ComponentValues.String()
			selectors, err := cascadia.ParseGroupWithPseudoElements(selector)
			if err != nil {
				return nil, err
			}
			for _, sel := range selectors {
				selSpecificity := sel.Specificity()
				s := selSpecificity[0]*100 + selSpecificity[1]*10 + selSpecificity[2]
				rules[s] = append(rules[s], selRule{selector: sel, rule: block.Rules})
			}
		}
	}
	// sort map keys
	keys := make([]int, 0, len(rules))
	for k := range rules {
		keys = append(keys, k)
	}
	// now sorted by specificity
	sort.Ints(keys)
	doc := c.document.Get(0)
	for _, k := range keys {
		for _, r := range rules[k] {
			for _, singlerule := range r.rule {
				for _, node := range cascadia.QueryAll(doc, r.selector) {
					var prefix string
					if pe := r.selector.PseudoElement(); pe != "" {
						prefix = pe + "::"
					}
					node.Attr = append(node.Attr, html.Attribute{Key: "!" + prefix + stringValue(singlerule.Key), Val: stringValue(singlerule.Value)})
				}
			}
		}
	}

	return c.document.Find(":root"), nil
}

func (c *CSS) dumpTree() (*goquery.Selection, error) {
	type selRule struct {
		selector cascadia.Sel
		rule     []qrule
	}

	rules := map[int][]selRule{}

	c.document.Each(resolveStyle)
	for _, stylesheet := range c.Stylesheet {
		for _, block := range stylesheet.Blocks {
			selector := block.ComponentValues.String()
			selectors, err := cascadia.ParseGroupWithPseudoElements(selector)
			if err != nil {
				return nil, err
			}
			for _, sel := range selectors {
				selSpecificity := sel.Specificity()
				s := selSpecificity[0]*100 + selSpecificity[1]*10 + selSpecificity[2]
				rules[s] = append(rules[s], selRule{selector: sel, rule: block.Rules})
			}
		}
	}
	// sort map keys
	keys := make([]int, 0, len(rules))
	for k := range rules {
		keys = append(keys, k)
	}
	// now sorted by specificity
	sort.Ints(keys)
	doc := c.document.Get(0)
	for _, k := range keys {
		for _, r := range rules[k] {
			for _, singlerule := range r.rule {
				for _, node := range cascadia.QueryAll(doc, r.selector) {
					var prefix string
					if pe := r.selector.PseudoElement(); pe != "" {
						prefix = pe + "::"
					}
					node.Attr = append(node.Attr, html.Attribute{Key: "!" + prefix + stringValue(singlerule.Key), Val: stringValue(singlerule.Value)})
				}
			}
		}
	}
	return c.document.Find(":root"), nil
}

func papersize(typ string) (string, string) {
	typ = strings.ToLower(typ)
	var width, height string
	portrait := true
	for i, e := range strings.Fields(typ) {
		switch e {
		case "portrait":
			// good, nothing to do
		case "landscape":
			portrait = false
		case "a5":
			width = "148mm"
			height = "210mm"
		case "a4":
			width = "210mm"
			height = "297mm"
		case "a3":
			width = "297mm"
			height = "420mm"
		case "b5":
			width = "176mm"
			height = "250mm"
		case "b4":
			width = "250mm"
			height = "353mm"
		case "jis-b5":
			width = "182mm"
			height = "257mm"
		case "jis-b4":
			width = "257mm"
			height = "364mm"
		case "letter":
			width = "8.5in"
			height = "11in"
		case "legal":
			width = "8.5in"
			height = "14in"
		case "ledger":
			width = "11in"
			height = "17in"
		default:
			if i == 0 {
				width = e
				height = e
			} else {
				height = e
			}
		}
	}

	if portrait {
		return width, height
	}
	return height, width
}

// ReadHTMLChunk reads the HTML text.
func (c *CSS) ReadHTMLChunk(htmltext string) error {
	var err error
	r := strings.NewReader(htmltext)
	c.document, err = goquery.NewDocumentFromReader(r)
	if err != nil {
		return err
	}
	var errcond error
	c.document.Find(":root > head link").Each(func(i int, sel *goquery.Selection) {
		if stylesheetfile, attExists := sel.Attr("href"); attExists {
			block, err := c.ParseCSSFile(stylesheetfile)
			if err != nil {
				errcond = err
			}
			parsedStyles := ConsumeBlock(block, false)
			c.Stylesheet = append(c.Stylesheet, parsedStyles)
		}
	})
	return errcond
}
