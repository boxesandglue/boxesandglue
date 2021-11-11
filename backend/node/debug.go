package node

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// Debug shows node list debug output
func Debug(n Node) {
	debugNode(n, 0)
}

func debugNode(n Node, level int) {
	for e := n; e != nil; e = e.Next() {
		fmt.Print(strings.Repeat(" | ", level))
		switch v := e.(type) {
		case *VList:
			color.Cyan("vlist (%d) wd: %s ht %s", v.ID, v.Width, v.Height)
			debugNode(v.List, level+1)
		case *HList:
			color.HiBlue("hlist (%d) wd: %s ht: %s", v.ID, v.Width, v.Height)
			debugNode(v.List, level+1)
		case *Disc:
			color.HiBlack("disc (%d)", v.ID)
		case *Glyph:
			color.HiGreen("glyph (%d): %s wd: %s cp: %d", v.ID, v.Components, v.Width, v.Codepoint)
		case *Glue:
			color.HiMagenta("glue (%d): %spt", v.ID, v.Width)
		case *Image:
			var filename string
			if v.Img != nil && v.Img.ImageFile != nil {
				filename = v.Img.ImageFile.Filename
			} else {
				filename = "(image object not set)"
			}
			color.Magenta("image (%d): %s", v.ID, filename)
		case *Lang:
			var langname string
			if v.Lang != nil {
				langname = v.Lang.Name
			} else {
				langname = "-"
			}
			color.Magenta("lang (%d): %s", v.ID, langname)
		case *Penalty:
			color.HiMagenta("peanlty (%d): %d flagged: %t wd: %spt", v.ID, v.Penalty, v.Flagged, v.Width)
		default:
			color.HiRed("Unhandled token %v", v)
		}
	}

}
