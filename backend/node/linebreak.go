package node

import (
	"fmt"

	"github.com/speedata/boxesandglue/backend/bag"
)

// LinebreakSettings contains all information about the final paragraph
type LinebreakSettings struct {
	HSize bag.ScaledPoint
}

// Linebreak returns a VList with horizontal lists where each horizontal
// list is a line.
func Linebreak(nl *Nodelist, settings LinebreakSettings) *VList {
	type breakpoint struct {
		glueNode *Node
		sumwd    bag.ScaledPoint
	}

	vl := NewVList()
	lastBreakpoint := breakpoint{}
	var sumwd bag.ScaledPoint
	linehead := nl.Front()
	for e := nl.Front(); e != nil; e = e.Next() {
		switch v := e.Value.(type) {
		case *Glue:
			if sumwd < settings.HSize {
				// collect more nodes but remember this glue
				lastBreakpoint.glueNode = e
				lastBreakpoint.sumwd = sumwd
				sumwd = sumwd + v.Width
			} else {
				hl := HPackToWithEnd(linehead, lastBreakpoint.glueNode.Prev(), settings.HSize)
				sumwd = sumwd - lastBreakpoint.sumwd
				linehead = lastBreakpoint.glueNode.Next()
				vl.List.AppendNode(hl)
			}
		case *Glyph:
			sumwd += v.Width
		case *Lang, *Disc:
			// ignore
		default:
			fmt.Println("Linebreak: unknown node type", v)
		}

	}

	hl := HPackToWithEnd(linehead, linehead.list.Back(), settings.HSize)
	vl.List.AppendNode(hl)

	return vl
}
