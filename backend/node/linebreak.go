package node

import (
	"fmt"

	"github.com/speedata/boxesandglue/backend/bag"
)

// LinebreakSettings contains all information about the final paragraph
type LinebreakSettings struct {
	HSize      bag.ScaledPoint
	LineHeight bag.ScaledPoint
}

// SimpleLinebreak returns a VList with horizontal lists where each horizontal
// list is a line.
func SimpleLinebreak(hl *HList, settings LinebreakSettings) *VList {
	type breakpoint struct {
		glueNode Node
		sumwd    bag.ScaledPoint
	}
	nl := hl.List
	vl := NewVList()
	var lastLine Node
	lastBreakpoint := breakpoint{}
	var sumwd bag.ScaledPoint
	linehead := nl

	for e := nl; e != nil; e = e.Next() {
		switch v := e.(type) {
		case *Glue:
			if sumwd < settings.HSize {
				// collect more nodes but remember this glue
				lastBreakpoint.glueNode = e
				lastBreakpoint.sumwd = sumwd
				sumwd = sumwd + v.Width
			} else {
				lastGlue := lastBreakpoint.glueNode
				lastNode := lastGlue.Prev()
				hl := HPackToWithEnd(linehead, lastNode, settings.HSize)
				hl.Height = settings.LineHeight
				sumwd = sumwd - lastBreakpoint.sumwd
				vl.List = InsertAfter(vl.List, lastLine, hl)
				linehead = lastBreakpoint.glueNode.Next()
				lastLine = hl
				vl.Height += hl.Height
			}
		case *Glyph:
			sumwd += v.Width
		case *Lang, *Disc:
			// ignore
		default:
			fmt.Println("Linebreak: unknown node type", v)
		}
	}

	if sumwd > settings.HSize {
		hl := HPackToWithEnd(linehead, lastBreakpoint.glueNode.Prev(), settings.HSize)
		hl.Height = settings.LineHeight
		sumwd = sumwd - lastBreakpoint.sumwd
		linehead = lastBreakpoint.glueNode.Next()
		vl.List = InsertAfter(vl.List, lastLine, hl)
		lastLine = hl
		vl.Height += hl.Height
	}
	hl = Hpack(linehead)

	hl.Height = settings.HSize
	InsertAfter(vl.List, lastLine, hl)
	vl.Width = settings.HSize
	vl.Height += hl.Height

	return vl
}
