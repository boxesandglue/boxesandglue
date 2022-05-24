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
			color.Cyan("vlist (%d) wd: %s, ht %s, dp %s", v.ID, v.Width, v.Height, v.Depth)
			debugNode(v.List, level+1)
		case *HList:
			color.HiBlue("hlist (%d) wd: %s ht: %s dp: %s", v.ID, v.Width, v.Height, v.Depth)
			debugNode(v.List, level+1)
		case *Disc:
			color.HiBlack("disc (%d)", v.ID)
		case *Glyph:
			var fontid int
			if fnt := v.Font; fnt != nil {
				fontid = fnt.Face.FaceID
			}
			color.HiGreen("glyph (%d): %s wd: %s ht: %s, dp: %s, cp: %d face %d", v.ID, v.Components, v.Width, v.Height, v.Depth, v.Codepoint, fontid)
		case *Glue:
			color.HiMagenta("glue (%d): %spt plus %spt minus %spt stretch order %d shrink order %d", v.ID, v.Width, v.Stretch, v.Shrink, v.StretchOrder, v.ShrinkOrder)
		case *Image:
			var filename string
			if v.Img != nil && v.Img.ImageFile != nil {
				filename = v.Img.ImageFile.Filename
			} else {
				filename = "(image object not set)"
			}
			color.Magenta("image (%d): %s", v.ID, filename)
		case *Kern:
			color.Blue("kern (%d): %s", v.ID, v.Kern)
		case *Lang:
			var langname string
			if v.Lang != nil {
				langname = v.Lang.Name
			} else {
				langname = "-"
			}
			color.Magenta("lang (%d): %s", v.ID, langname)
		case *Penalty:
			color.HiMagenta("peanlty (%d): %d wd: %spt", v.ID, v.Penalty, v.Width)
		case *Rule:
			color.HiBlack("rule (%d)", v.ID)
		case *StartStop:
			color.HiCyan("start/stop (%d)", v.ID)
		default:
			color.HiRed("Unhandled token %v", v)
		}
	}
}
