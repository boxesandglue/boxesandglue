package debug

import (
	"fmt"
	"strings"

	"github.com/speedata/texperiments/node"

	"github.com/fatih/color"
)

// Debug outputs a colorful representation of the nodelist
func Debug(nl *node.Nodelist) {
	debug(nl, 0)
}

func debug(nl *node.Nodelist, level int) {
	for e := nl.Front(); e != nil; e = e.Next() {
		fmt.Print(strings.Repeat(" | ", level))
		switch v := e.Value.(type) {
		case *node.VList:
			color.Cyan("vlist wd: %s", v.Width)
			debug(v.List, level+1)
		case *node.HList:
			color.HiBlue("hlist wd: %s", v.Width)
			debug(v.List, level+1)
		case *node.Glyph:
			color.HiGreen("glyph: %s wd: %s", v.Components, v.Width)
		case *node.Lang:
			color.Magenta("lang: %s", v.Lang.Name)
		case *node.Glue:
			color.HiMagenta("glue: %s", v.Width)
		case *node.Disc:
			color.HiBlack("disc")
		default:
			color.HiRed("Unhandled token %v", v)
		}
	}

}
