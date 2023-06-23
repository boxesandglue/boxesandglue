package csshtml

import (
	"fmt"
	"strings"
)

func indent(s string) string {
	ret := []string{}
	for _, line := range strings.Split(s, "\n") {
		ret = append(ret, "    "+line)
	}
	return strings.Join(ret, "\n")
}

func (b SBlock) String() string {
	ret := []string{}
	var firstline string
	if b.Name != "" {
		firstline = fmt.Sprintf("@%s ", b.Name)
	}
	firstline = firstline + b.ComponentValues.String() + " {"
	ret = append(ret, firstline)
	for _, v := range b.Rules {
		ret = append(ret, "    "+v.Key.String()+":"+v.Value.String()+";")
	}
	for _, v := range b.ChildAtRules {
		ret = append(ret, indent(v.String()))
	}
	for _, v := range b.Blocks {
		ret = append(ret, indent(v.String()))
	}
	ret = append(ret, "}")
	return strings.Join(ret, "\n")
}

func (t Tokenstream) String() string {
	ret := []string{}
	for _, tok := range t {
		ret = append(ret, tok.Value)
	}
	return strings.Join(ret, "")
}

// Show returns the CSS written as a nice string
func (c *CSS) Show() string {
	var sb strings.Builder

	// for name, ff := range c.Fontfamilies {
	// 	w(" Font family", name)
	// 	w("    Regular: ", ff.Regular)
	// 	w("    Italic: ", ff.Italic)
	// 	w("    Bold: ", ff.Bold)
	// 	w("    BoldItalic: ", ff.BoldItalic)
	// }
	for name, pg := range c.Pages {
		fmt.Fprintln(&sb, " Page", name)
		fmt.Fprintln(&sb, "   Size", pg.Papersize)
		styles, _ := ResolveAttributes(pg.Attributes)
		fmt.Fprintln(&sb, "   Margin: ", styles["margin-top"], styles["margin-right"], styles["margin-bottom"], styles["margin-left"])
		for areaname, rules := range pg.pageareaRules {
			fmt.Fprintln(&sb, "   @", areaname)
			for _, rule := range rules {
				fmt.Fprintln(&sb, "     ", rule.Key, rule.Value)

			}
		}
	}
	for _, stylesheet := range c.Stylesheet {
		for _, block := range stylesheet.Blocks {
			fmt.Fprintln(&sb, "-------")
			fmt.Fprintln(&sb, block)
		}
		fmt.Fprintln(&sb, "++++++++++")
	}
	return sb.String()
}
