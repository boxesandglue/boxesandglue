package csshtml

import (
	"bufio"
	"fmt"
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

	dimen              = regexp.MustCompile(`^^[+\-]?(?:(?:0+|[1-9]\d*)(?:\.\d*)?|\.\d+)(px|mm|cm|in|pt|pc|ch|em|ex|lh|rem|0)$`)
	zeroDimen          = regexp.MustCompile(`^0+(px|mm|cm|in|pt|pc|ch|em|ex|lh|rem)?`)
	style              = regexp.MustCompile(`^none|hidden|dotted|dashed|solid|double|groove|ridge|inset|outset$`)
	colorMatcher       = regexp.MustCompile(`^rgba?\s*\(`)
	toprightbottomleft = [...]string{"top", "right", "bottom", "left"}
)

func normalizespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func stringValue(toks tokenstream) string {
	ret := []string{}
	negative := false
	prevNegative := false
	for _, tok := range toks {
		prevNegative = negative
		negative = false
		switch tok.Type {
		case scanner.Ident:
			ret = append(ret, tok.Value)
		case scanner.String:
			ret = append(ret, fmt.Sprintf("%q", tok.Value))
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
				fmt.Println("unhandled delimiter", tok)
			}
		case scanner.URI:
			ret = append(ret, "url("+tok.Value+")")
		case scanner.Local:
			ret = append(ret, "local("+tok.Value+")")
		default:
			fmt.Println("unhandled token", tok)
		}
	}
	return strings.Join(ret, " ")
}

// Recurse through the HTML tree and resolve the style attribute
func resolveStyle(i int, sel *goquery.Selection) {
	a, b := sel.Attr("style")
	if b {
		var tokens tokenstream

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
		block := consumeBlock(tokens, true)
		for _, rule := range block.Rules {
			sel.SetAttr("!"+stringValue(rule.Key), stringValue(rule.Value))
		}
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
	fourValues := make(map[string]string)
	switch len(fields) {
	case 1:
		fourValues["top"] = fields[0]
		fourValues["right"] = fields[0]
		fourValues["bottom"] = fields[0]
		fourValues["left"] = fields[0]
	case 2:
		fourValues["top"] = fields[0]
		fourValues["right"] = fields[1]
		fourValues["bottom"] = fields[0]
		fourValues["left"] = fields[1]
	case 3:
		fourValues["top"] = fields[0]
		fourValues["right"] = fields[1]
		fourValues["bottom"] = fields[2]
		fourValues["left"] = fields[1]
	case 4:
		fourValues["top"] = fields[0]
		fourValues["right"] = fields[1]
		fourValues["bottom"] = fields[2]
		fourValues["left"] = fields[3]
	}

	return fourValues
}

// parseBorderAttribute splits "1pt solid black" into three parts.
func parseBorderAttribute(input string) (width string, style string, color string) {
	s := bufio.NewScanner(strings.NewReader(input))
	s.Split(bufio.ScanWords)

	width = "1pt"
	style = "none"
	color = "currentcolor"
	// 0 = width, 1 = style, 2 = color
	for s.Scan() {
		t := s.Text()
		// looking for width
		ok, wd := isDimension(t)
		if ok {
			width = wd
			continue
		}
		// looking for style
		if t := t; t == "none" || t == "hidden" || t == "dotted" || t == "dashed" || t == "solid" || t == "double" || t == "groove" || t == "ridge" || t == "inset" || t == "outset" {
			style = t
			continue
		}
		if strings.HasPrefix(t, "#") {
			color = t
			return
		}
		if colorMatcher.MatchString(t) {
			color = t
			for s.Scan() {
				color += " " + s.Text()
			}
			return
		}

		color = t
	}
	return
}

// ResolveAttributes returns the resolved styles and the attributes of the node.
// It changes "margin: 1cm;" into "margin-left: 1cm; margin-right: 1cm; ...".
func ResolveAttributes(attrs []html.Attribute) (resolved map[string]string, attributes map[string]string, newAttributes []html.Attribute) {
	resolved = make(map[string]string)
	attributes = make(map[string]string)
	newAttributes = make([]html.Attribute, 0, len(attrs))
	// attribute resolving must be in order of appearance.
	// For example the following border-left-style has no effect:
	//    border-left-style: dotted;
	//    border-left: thick green;
	// because the second line overrides the first line (style defaults to "none")
	for _, attr := range attrs {
		key := attr.Key
		if !strings.HasPrefix(key, "!") {
			newAttributes = append(newAttributes, attr)
			continue
		}
		key = strings.TrimPrefix(key, "!")

		switch key {
		case "margin":
			values := getFourValues(attr.Val)
			for _, margin := range toprightbottomleft {
				resolved["margin-"+margin] = values[margin]
				newAttributes = append(newAttributes, html.Attribute{Key: "!margin-" + margin, Val: values[margin]})
			}
		case "list-style":
			for _, part := range strings.Split(attr.Val, " ") {
				switch part {
				case "inside", "outside":
					resolved["list-style-position"] = part
					newAttributes = append(newAttributes, html.Attribute{Key: "!list-style-position", Val: part})
				default:
					if strings.HasPrefix(part, "url") {
						resolved["list-style-image"] = part
						newAttributes = append(newAttributes, html.Attribute{Key: "!list-style-image", Val: part})
					} else {
						resolved["list-style-type"] = part
						newAttributes = append(newAttributes, html.Attribute{Key: "!list-style-type", Val: part})
					}
				}
			}
		case "padding":
			values := getFourValues(attr.Val)
			for _, padding := range toprightbottomleft {
				resolved["padding-"+padding] = values[padding]
				newAttributes = append(newAttributes, html.Attribute{Key: "!padding-" + padding, Val: values[padding]})
			}
		case "border":
			wd, style, color := parseBorderAttribute(attr.Val)
			for _, loc := range toprightbottomleft {
				resolved["border-"+loc+"-style"] = style
				resolved["border-"+loc+"-width"] = wd
				resolved["border-"+loc+"-color"] = color
				newAttributes = append(newAttributes,
					html.Attribute{Key: "!border-" + loc + "-style", Val: style},
					html.Attribute{Key: "!border-" + loc + "-width", Val: wd},
					html.Attribute{Key: "!border-" + loc + "-color", Val: color},
				)
			}
		case "border-radius":
			for _, lr := range []string{"left", "right"} {
				for _, tb := range []string{"top", "bottom"} {
					resolved["border-"+tb+"-"+lr+"-radius"] = attr.Val
					newAttributes = append(newAttributes,
						html.Attribute{Key: "!border-" + tb + "-" + lr + "-radius", Val: attr.Val},
					)
				}
			}
		case "border-top", "border-right", "border-bottom", "border-left":
			wd, sty, col := parseBorderAttribute(attr.Val)
			resolved[key+"-width"], resolved[key+"-style"], resolved[key+"-color"] = wd, sty, col
			newAttributes = append(newAttributes,
				html.Attribute{Key: "!" + key + "-width", Val: wd},
				html.Attribute{Key: "!" + key + "-style", Val: sty},
				html.Attribute{Key: "!" + key + "-color", Val: col},
			)

		case "border-color":
			values := getFourValues(attr.Val)
			for _, loc := range toprightbottomleft {
				resolved["border-"+loc+"-color"] = values[loc]
				newAttributes = append(newAttributes,
					html.Attribute{Key: "!border-" + loc + "-color", Val: values[loc]},
				)
			}
			resolved[key] = attr.Val
		case "border-style":
			values := getFourValues(attr.Val)
			for _, loc := range toprightbottomleft {
				resolved["border-"+loc+"-style"] = values[loc]
				newAttributes = append(newAttributes,
					html.Attribute{Key: "!border-" + loc + "-style", Val: values[loc]},
				)
			}
		case "border-width":
			values := getFourValues(attr.Val)
			for _, loc := range toprightbottomleft {
				resolved["border-"+loc+"-width"] = values[loc]
				newAttributes = append(newAttributes,
					html.Attribute{Key: "!border-" + loc + "-width", Val: values[loc]},
				)
			}
			resolved[key] = attr.Val
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
						newAttributes = append(newAttributes,
							html.Attribute{Key: "!font-size", Val: field},
						)

					} else {
						resolved["font-name"] = field
						newAttributes = append(newAttributes,
							html.Attribute{Key: "!font-name", Val: field},
						)
					}
				}
			}
			resolved["font-style"] = fontstyle
			resolved["font-weight"] = fontweight
			newAttributes = append(newAttributes,
				html.Attribute{Key: "!font-style", Val: fontstyle},
				html.Attribute{Key: "!font-weight", Val: fontweight},
			)

		// font-stretch: ultra-condensed; extra-condensed; condensed;
		// semi-condensed; normal; semi-expanded; expanded; extra-expanded;
		// ultra-expanded;
		case "text-decoration":
			for _, part := range strings.Split(attr.Val, " ") {
				if part == "none" || part == "underline" || part == "overline" || part == "line-through" {
					resolved["text-decoration-line"] = part
					newAttributes = append(newAttributes,
						html.Attribute{Key: "!text-decoration-line", Val: part},
					)

				} else if part == "solid" || part == "double" || part == "dotted" || part == "dashed" || part == "wavy" {
					resolved["text-decoration-style"] = part
					newAttributes = append(newAttributes,
						html.Attribute{Key: "!text-decoration-style", Val: part},
					)
				}
			}

		case "background":
			// background-clip, background-color, background-image,
			// background-origin, background-position, background-repeat,
			// background-size, and background-attachment
			for _, part := range strings.Split(attr.Val, " ") {
				resolved["background-color"] = part
				newAttributes = append(newAttributes,
					html.Attribute{Key: "!background-color", Val: part},
				)

			}
		default:
			resolved[key] = attr.Val
			newAttributes = append(newAttributes, attr)
		}
	}

	if str, ok := resolved["text-decoration-line"]; ok && str != "none" {
		resolved["text-decoration-style"] = "solid"
		newAttributes = append(newAttributes,
			html.Attribute{Key: "!text-decoration-style", Val: "solid"},
		)
	}
	return
}

// ApplyCSS resolves CSS rules in the DOM. Each CSS rule is added to the
// selection as an attribute (prefixed with a !). Pseudo elements are prefixed
// with ::.
func (c *CSS) ApplyCSS(doc *goquery.Document) (*goquery.Document, error) {
	type selRule struct {
		selector cascadia.Sel
		rule     []qrule
	}

	rules := map[int][]selRule{}

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
	root := doc.Get(0)
	for _, k := range keys {
		for _, r := range rules[k] {
			for _, singlerule := range r.rule {
				for _, node := range cascadia.QueryAll(root, r.selector) {
					var prefix string
					if pe := r.selector.PseudoElement(); pe != "" {
						prefix = pe + "::"
					}
					// remove attributes with the same name, since the new ones
					// must override the old ones.
					key := "!" + prefix + stringValue(singlerule.Key)
					newAttributes := make([]html.Attribute, 0, len(node.Attr))
					for _, attr := range node.Attr {
						if attr.Key != key {
							newAttributes = append(newAttributes, attr)
						}
					}
					newAttributes = append(newAttributes, html.Attribute{Key: key, Val: stringValue(singlerule.Value)})
					node.Attr = newAttributes
					_, _, node.Attr = ResolveAttributes(node.Attr)
				}
			}
		}
	}

	doc.Each(resolveStyle)
	return doc, nil
}

// PapersizeWidthHeight converts the spec to the width and height. The parameter
// can be a known paper size (such as A4 or letter) or a one or two parameter
// string such as 20cm 20cm.
func PapersizeWidthHeight(spec string) (string, string) {
	spec = strings.ToLower(spec)
	var width, height string
	portrait := true
	for i, e := range strings.Fields(spec) {
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
