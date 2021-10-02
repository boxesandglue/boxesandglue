package node

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// Debug outputs a colorful representation of the nodelist
func Debug(nl *Nodelist) {
	debug(nl, 0)
}

func debug(nl *Nodelist, level int) {
	for e := nl.Front(); e != nil; e = e.Next() {
		fmt.Print(strings.Repeat(" | ", level))
		switch v := e.Value.(type) {
		case *VList:
			color.Cyan("vlist wd: %s ht %s", v.Width, v.Height)
			debug(v.List, level+1)
		case *HList:
			color.HiBlue("hlist wd: %s ht: %s", v.Width, v.Height)
			debug(v.List, level+1)
		case *Glyph:
			color.HiGreen("glyph: %s wd: %s cp: %d", v.Components, v.Width, v.Codepoint)
		case *Lang:
			color.Magenta("lang: %s", v.Lang.Name)
		case *Glue:
			color.HiMagenta("glue: %s", v.Width)
		case *Disc:
			color.HiBlack("disc")
		default:
			color.HiRed("Unhandled token %v", v)
		}
	}

}
