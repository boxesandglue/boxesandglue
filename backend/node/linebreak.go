package node

import (
	"math"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
)

// inspired by github.com/tdewolff/fuzz/canvas pdf package

var (
	positiveInf = math.Inf(1.0)
)

// The data structure here used to store the breakpoints is a two way linked
// list where the "next" pointer builds the chain of active nodes (all nodes to
// be considered when looking if the active can reach the current position) and
// the "from" pointer points to the line break of the previous line. The "from"
// pointer is set when creating a new breakpoint node and adding it to the list
// of active nodes.

// Breakpoint is a feasible break point.
type Breakpoint struct {
	from                                  *Breakpoint
	next                                  *Breakpoint
	Position                              Node
	Line                                  int
	Fitness                               int
	Width                                 bag.ScaledPoint
	sumW, sumY, sumZ                      bag.ScaledPoint
	stretchFil, stretchFill, stretchFilll bag.ScaledPoint
	R                                     float64
	Demerits                              int
}

type linebreaker struct {
	items            Node
	activeNodesA     *Breakpoint
	inactiveNodesP   *Breakpoint
	sumW, sumY, sumZ bag.ScaledPoint
	stretchFil       bag.ScaledPoint
	stretchFill      bag.ScaledPoint
	stretchFilll     bag.ScaledPoint
	settings         *LinebreakSettings
}

func newLinebreaker(hl Node, settings *LinebreakSettings) *linebreaker {
	lb := &linebreaker{
		settings: settings,
	}
	return lb
}

func (lb *linebreaker) computeAdjustmentRatio(n Node, a *Breakpoint) float64 {
	// compute the adjustment ratio r from a to n
	desiredLineWidth := lb.sumW - a.sumW
	if penalty, ok := n.(*Penalty); ok {
		desiredLineWidth += penalty.Width
	}

	r := 0.0
	if desiredLineWidth < lb.settings.HSize {
		y := lb.sumY - a.sumY
		if y > 0 {
			if lb.stretchFil > 0 || lb.stretchFill > 0 || lb.stretchFilll > 0 {
				return 0
			}
			r = float64(lb.settings.HSize-desiredLineWidth) / float64(lb.sumY-a.sumY)
		} else {
			r = positiveInf
		}
	} else if lb.settings.HSize < desiredLineWidth {
		z := lb.sumZ - a.sumZ
		if z > 0 {
			r = float64(lb.settings.HSize-desiredLineWidth) / float64(z)
		} else {
			r = positiveInf
		}
	}
	return r
}

func (lb *linebreaker) computeSum(n Node) (bag.ScaledPoint, bag.ScaledPoint, bag.ScaledPoint, bag.ScaledPoint, bag.ScaledPoint, bag.ScaledPoint) {
	// compute tw=(sum w)after(b), ty=(sum y)after(b), and tz=(sum z)after(b)
	w, y, z := lb.sumW, lb.sumY, lb.sumZ
	stretchFil, stretchFill, stretchFilll := lb.stretchFil, lb.stretchFill, lb.stretchFilll
compute:
	for e := n; e != nil; e = e.Next() {
		switch t := e.(type) {
		case *Glue:
			w += t.Width
			z += t.Shrink
			switch t.StretchOrder {
			case StretchFil:
				stretchFil += t.Stretch
			case StretchFill:
				stretchFill += t.Stretch
			case StretchFilll:
				stretchFilll += t.Stretch
			default:
				y += t.Stretch
			}
		case *Penalty:
			if t.Penalty == -10000 && e != n {
				break compute
			}
		default:
			break compute
		}
	}
	return w, y, z, stretchFil, stretchFill, stretchFilll
}

func showWordBefore(n Node) string {
	var start Node
	for e := n.Prev(); e != nil; e = e.Prev() {
		if _, ok := e.(*Glue); ok {
			start = e
			break
		}
	}
	if start == nil {
		return ""
	}
	var str []string

	for e := start; e != n; e = e.Next() {
		if g, ok := e.(*Glyph); ok {
			str = append(str, g.Components)
		}
	}
	return strings.Join(str, "")
}

func (lb *linebreaker) mainLoop(n Node) {
	active := lb.activeNodesA
	var preva *Breakpoint

	for active != nil {
		dmin := math.MaxInt
		dc := [4]int{math.MaxInt, math.MaxInt, math.MaxInt, math.MaxInt}
		ac := [4]*Breakpoint{}
		rc := [4]float64{}

		for active != nil {
			nexta := active.next
			r := lb.computeAdjustmentRatio(n, active)
			if p, ok := n.(*Penalty); r < -1 || ok && p.Penalty == -10000 {
				// If line is too wide or a forced break, we can remove the node
				// from the active list.
				if preva == nil {
					lb.activeNodesA = nexta
				} else {
					preva.next = nexta
				}
				active.next = lb.inactiveNodesP
				lb.inactiveNodesP = active
			} else {
				preva = active
			}
			if -1 <= r && r < lb.settings.Tolerance {
				// That looks like a good breakpoint.

				// compute demerits d and fitness class c
				badness := 100.0 * math.Pow(math.Abs(r), 3)
				onePlusBadnessSquared := int(math.Pow(1.0+badness, 2))
				var curpenalty int
				var curflagged bool
				if p, ok := n.(*Penalty); ok {
					curpenalty = p.Penalty
					curflagged = p.Flagged
				}
				demerits := 0

				if p, ok := active.Position.(*Penalty); ok {
					if curflagged && p.Flagged {
						demerits += lb.settings.DoublehyphenDemerits
					}
				}

				if curpenalty >= 0 {
					demerits = onePlusBadnessSquared + curpenalty*curpenalty
				} else if curpenalty > -10000 && curpenalty < 0 {
					demerits = onePlusBadnessSquared - curpenalty*curpenalty
				} else {
					demerits = onePlusBadnessSquared
				}

				// calculate fitness class
				var c int
				switch {
				case r < -0.5:
					c = 0
				case r <= 0.5:
					c = 1
				case r <= 1.0:
					c = 2
				default:
					c = 3
				}
				if c > active.Fitness {
					if c-active.Fitness > 1 {
						demerits += lb.settings.DemeritsFitness
					}
				} else {
					if active.Fitness-c > 1 {
						demerits += lb.settings.DemeritsFitness
					}
				}

				demerits += active.Demerits
				if demerits < dc[c] {
					dc[c] = demerits
					ac[c] = active
					rc[c] = r
					if demerits < dmin {
						dmin = demerits
					}
				}
			}
			j := active.Line + 1
			active = nexta
			if active != nil && j <= active.Line {
				// we omitted (j < j0) as j0 is difficult to know for complex cases
				break
			}
		}
		if dmin < math.MaxInt {
			W, Y, Z, stretchFil, stretchFill, stretchFilll := lb.computeSum(n)

			width := lb.sumW
			if p, ok := n.(*Penalty); ok {
				width += p.Width
			}

			for c := 0; c < 4; c++ {
				if dc[c] <= dmin+lb.settings.DemeritsFitness {
					bp := &Breakpoint{
						Position:     n,
						Line:         ac[c].Line + 1,
						from:         ac[c],
						next:         active,
						Fitness:      c,
						Width:        width,
						sumW:         W,
						sumY:         Y,
						sumZ:         Z,
						stretchFil:   stretchFil,
						stretchFill:  stretchFill,
						stretchFilll: stretchFilll,
						R:            rc[c],
						Demerits:     dc[c],
					}
					if preva == nil {
						lb.activeNodesA = bp
					} else {
						preva.next = bp
					}
					preva = bp
				}
			}
		}
	}
}

// Linebreak breaks the node list starting at n into lines. Returns a VList of
// HLists and information about each line.
func Linebreak(n Node, settings *LinebreakSettings) (*VList, []*Breakpoint) {
	var prevItemBox bool
	lb := newLinebreaker(n, settings)
	lb.activeNodesA = &Breakpoint{Fitness: 1, Position: n}
	var endNode Node

	for e := n; e != nil; e = e.Next() {
		// breakable after
		switch t := e.(type) {
		case *Glue:
			if prevItemBox {
				// b legal breakpoint
				lb.mainLoop(t)
			}

			lb.sumW += t.Width
			lb.sumZ += t.Shrink

			switch t.StretchOrder {
			case StretchFil:
				lb.stretchFil += t.Stretch
			case StretchFill:
				lb.stretchFill += t.Stretch
			case StretchFilll:
				lb.stretchFilll += t.Stretch
			default:
				lb.sumY += t.Stretch
			}

			prevItemBox = false
		case *Penalty:
			prevItemBox = false
			if t.Penalty < 10000 {
				lb.mainLoop(t)
			}
		default:
			prevItemBox = true
			wd := getWidth(e)
			lb.sumW += wd
		}
		endNode = e
	}

	var bps []*Breakpoint

	// There might be several nodes in here which end at the last glue with
	// different numbers of lines. Let's pick the one with the fewest total
	// demerits, as we do not specify a loosenes parameter yet.
	demerits := math.MaxInt
	lastNode := lb.activeNodesA
	for e := lb.activeNodesA; e != nil; e = e.next {
		if e.Demerits < demerits {
			lastNode = e
			demerits = e.Demerits
		}
	}

	// Now lastNode has the fewest total demerits.
	vl := NewVList()
	for e := lastNode; e != nil; e = e.from {
		startPos := e.Position
		if startPos.Prev() != nil {
			startPos = startPos.Next()
		}
		hl, _ := HPackToWithEnd(startPos, endNode.Prev(), lb.settings.HSize)
		hl.Height = lb.settings.LineHeight
		vl.List = InsertBefore(vl.List, vl.List, hl)
		endNode = e.Position
		bps = append(bps, e)
	}
	for i, j := 0, len(bps)-1; i < j; i, j = i+1, j-1 {
		bps[i], bps[j] = bps[j], bps[i]
	}

	return vl, bps[1:]
}

func getWidth(n Node) bag.ScaledPoint {
	switch t := n.(type) {
	case *Glue:
		return t.Width
	case *Glyph:
		return t.Width
	case *Penalty:
		return t.Width
	case *Lang:
		return 0
	default:
		bag.Logger.DPanicf("nyi: getWidth(%#v)", n)
	}
	return 0
}

// AppendLineEndAfter adds a penalty 10000, glue 0pt plus 1fil, penalty -10000
// after n.
func AppendLineEndAfter(n Node) {
	p := NewPenalty()
	p.Penalty = 10000
	InsertAfter(n, n, p)

	g := NewGlue()
	g.Width = 0
	g.Stretch = 1 * bag.Factor
	g.StretchOrder = 1
	InsertAfter(n, p, g)

	p = NewPenalty()
	p.Penalty = -10000
	n = InsertAfter(n, g, p)
}
