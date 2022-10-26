package node

import (
	"fmt"
	"math"
	"strings"

	"github.com/speedata/boxesandglue/backend/bag"
)

var (
	positiveInf   = math.Inf(1.0)
	negativeInf   = math.Inf(-1.0)
	breakpointIDs chan int
)

func init() {
	breakpointIDs = make(chan int)
	go genIntegerSequence(breakpointIDs)
}

// The data structure here used to store the breakpoints is a two way linked
// list where the "next" pointer builds the chain of active nodes (all nodes to
// be considered when looking if the active can reach the current position) and
// the "from" pointer points to the line break of the previous line. The "from"
// pointer is set when creating a new breakpoint node and adding it to the list
// of active nodes.

// Breakpoint is a feasible break point.
type Breakpoint struct {
	id                                    int
	from                                  *Breakpoint
	next                                  *Breakpoint
	Position                              Node
	Pre                                   Node
	Line                                  int
	Fitness                               int
	Width                                 bag.ScaledPoint
	sumW, sumY, sumZ                      bag.ScaledPoint
	stretchFil, stretchFill, stretchFilll bag.ScaledPoint
	R                                     float64
	Demerits                              int
}

func (bp *Breakpoint) String() string {
	ret := []string{}
	prefix := "cur"
	for e := bp; e != nil; e = e.from {
		ret = append(ret, fmt.Sprintf("%-7s %3d(id) %d(l) %10d(d) %d(f) (%s)", prefix, e.id, e.Line, e.Demerits, e.Fitness, showRecentNodes(e.Position, 10)))
		prefix = "  └───>"
	}
	return strings.Join(ret, "\n")
}

type linebreaker struct {
	activeNodesA     *Breakpoint
	inactiveNodesP   *Breakpoint
	preva            *Breakpoint
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

func getNextNodeWidth(n Node, maxwidth bag.ScaledPoint) bag.ScaledPoint {
	sumwd := bag.ScaledPoint(0)
	firstGlue := true
sum:
	for e := n; e != nil; e = e.Next() {
		switch t := e.(type) {
		case *Glue:
			if firstGlue {
				firstGlue = false
				sumwd += t.Width
			} else {
				break sum
			}
		case *Penalty:
			if t.Penalty < 10000 {
				break sum
			}
		case *Disc:
			break sum
		default:
			sumwd += getWidth(e, Horizontal)
		}
	}
	return sumwd
}

func (lb *linebreaker) computeAdjustmentRatio(n Node, a *Breakpoint) float64 {
	// compute the adjustment ratio r from a to n
	desiredLineWidth := lb.sumW - a.sumW
	switch t := n.(type) {
	case *Penalty:
		desiredLineWidth += t.Width
	case *Disc:
		desiredLineWidth += Dimensions(t.Pre, Horizontal)
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
			r = negativeInf
		}
	}
	if r == negativeInf || r == positiveInf {
		if desiredLineWidth+getNextNodeWidth(n, lb.settings.HSize) > lb.settings.HSize {
			return negativeInf
		}
	}
	return r
}

// computeSum computes the sum of all glues from n
func (lb *linebreaker) computeSum(n Node) (bag.ScaledPoint, bag.ScaledPoint, bag.ScaledPoint) {
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
	return w, y, z
}

func (lb *linebreaker) removeActiveNode(active *Breakpoint) {
	if lb.preva == nil {
		lb.activeNodesA = active.next
	} else {
		lb.preva.next = active.next
	}
	active.next = lb.inactiveNodesP
	lb.inactiveNodesP = active

}

func calculateFitnessClass(r float64) int {
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
	return c
}

func (lb *linebreaker) calculateDemerits(active *Breakpoint, r float64, n Node) (fitnessClass int, demerits int) {
	// compute demerits d and fitness class c
	badness := 100.0 * math.Pow(math.Abs(r), 3)
	onePlusBadnessSquared := int(math.Pow(1.0+badness, 2))
	var curpenalty int
	var curflagged bool
	switch t := n.(type) {
	case *Penalty:
		curpenalty = t.Penalty
	case *Disc:
		curpenalty = lb.settings.Hyphenpenalty + t.Penalty
		curflagged = true
	}

	if curpenalty >= 0 {
		demerits = onePlusBadnessSquared + curpenalty*curpenalty
	} else if curpenalty > -10000 && curpenalty < 0 {
		demerits = onePlusBadnessSquared - curpenalty*curpenalty
	} else {
		demerits = onePlusBadnessSquared
	}

	if _, ok := active.Position.(*Disc); ok {
		if curflagged {
			demerits += lb.settings.DoublehyphenDemerits
		}
	}

	// calculate fitness class
	c := calculateFitnessClass(r)

	// if c and active.Fitness differs by more then 1, add DemeritsFitness
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
	// integer overflow?
	if demerits < 0 {
		demerits = math.MaxInt
	}
	return
}

func (lb *linebreaker) mainLoop(n Node) {
	active := lb.activeNodesA
	lb.preva = nil

	// The outer loop calculates dmin for each of the four fitness classes c.
	for active != nil {
		dmin := math.MaxInt
		dc := [4]int{math.MaxInt, math.MaxInt, math.MaxInt, math.MaxInt}
		ac := [4]*Breakpoint{}
		rc := [4]float64{}

		// The inner loop deactivates all unreachable breakpoints and calculates
		// demerits/dmin.
		for {
			nexta := active.next

			// For each active breakpoint check if the breakpoint is still
			// active (= reachable from the current position backward). If not,
			// remove them from the current list of active nodes.
			r := lb.computeAdjustmentRatio(n, active)

			if p, ok := n.(*Penalty); r < -1 || ok && p.Penalty == -10000 {
				// If line is too wide or a forced break, we can remove the node
				// from the active list.
				lb.removeActiveNode(active)
			} else {
				lb.preva = active
			}

			// There might be active breakpoints (after cleanup), so all of them
			// are a candidate for a final breakpoint. For each fitness class,
			// we chose the best candidate (with the fewest total demerits)
			if -1 <= r && r < lb.settings.Tolerance {
				// That looks like a good breakpoint.
				c, demerits := lb.calculateDemerits(active, r, n)

				// Update candidate if (and only if) the total demerits are less
				// than the previous total demerits for this fitness class.
				//
				// Also update the minimum demerits for this position.
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

			if active = nexta; active == nil {
				break
			}
			// The next active node can be in the next line, so we quit the
			// calculation of the best breakpoint. This works, because the list
			// of active nodes are ordered ascending (wrt line number).
			if j <= active.Line {
				// we omitted (j < j0) as j0 is difficult to know for complex cases
				break
			}
		}
		if dmin < math.MaxInt {
			lb.appendBreakpointHere(n, dmin, dc, ac, rc, active)
		}
		if dmin == math.MaxInt && lb.activeNodesA == nil {
			W, Y, Z := lb.computeSum(n)
			lastInactive := lb.inactiveNodesP
			width := lb.sumW
			var pre Node
			switch v := n.(type) {
			case *Penalty:
				width += v.Width
			case *Disc:
				width += 5 * bag.Factor
				pre = v.Pre
			}

			bp := &Breakpoint{
				id:       <-breakpointIDs,
				Position: n,
				Pre:      pre,
				Line:     lastInactive.Line + 1,
				from:     lastInactive,
				next:     active,
				Fitness:  3,
				Width:    lb.sumW - lastInactive.sumW,
				sumW:     W,
				sumY:     Y,
				sumZ:     Z,
				R:        0,
				Demerits: lastInactive.Demerits + 1000,
			}
			lb.appendNewBreakpoint(bp)
		}
	}
}

func (lb *linebreaker) appendBreakpointHere(n Node, dmin int, dc [4]int, ac [4]*Breakpoint, rc [4]float64, active *Breakpoint) {
	W, Y, Z := lb.computeSum(n)

	width := lb.sumW
	var pre Node
	switch v := n.(type) {
	case *Penalty:
		width += v.Width
	case *Disc:
		width += 5 * bag.Factor
		pre = v.Pre
	}

	for c := 0; c < 4; c++ {
		if dc[c] <= dmin+lb.settings.DemeritsFitness {
			bp := &Breakpoint{
				id:       <-breakpointIDs,
				Position: n,
				Pre:      pre,
				Line:     ac[c].Line + 1,
				from:     ac[c],
				next:     active,
				Fitness:  c,
				Width:    width - ac[c].Width,
				sumW:     W,
				sumY:     Y,
				sumZ:     Z,
				R:        rc[c],
				Demerits: dc[c],
			}
			lb.appendNewBreakpoint(bp)
		}
	}
}

func (lb *linebreaker) appendNewBreakpoint(bp *Breakpoint) {
	if lb.preva == nil {
		lb.activeNodesA = bp
	} else {
		lb.preva.next = bp
	}
	lb.preva = bp
}

// Linebreak breaks the node list starting at n into lines. Returns a VList of
// HLists and information about each line.
func Linebreak(n Node, settings *LinebreakSettings) (*VList, []*Breakpoint) {
	if n == nil {
		return nil, nil
	}
	var prevItemBox bool
	lb := newLinebreaker(n, settings)
	lb.activeNodesA = &Breakpoint{id: <-breakpointIDs, Fitness: 1, Position: n}
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
		case *Disc:
			prevItemBox = false
			lb.mainLoop(t)
		default:
			prevItemBox = true
			wd := getWidth(e, Horizontal)
			lb.sumW += wd
		}
		endNode = e
	}
	// The order of the breakpoints is from last breakpoint to first breakpoint.
	var bps []*Breakpoint

	// There might be several nodes in here which end at the last glue with
	// different numbers of lines. Let's pick the one with the fewest total
	// demerits, as we do not specify a looseness parameter yet.
	demerits := math.MaxInt
	lastNode := lb.activeNodesA
	if lastNode == nil {
		lastNode = lb.inactiveNodesP
	}

	for e := lb.activeNodesA; e != nil; e = e.next {
		if e.Demerits < demerits {
			lastNode = e
			demerits = e.Demerits
		}
	}

	var curPre Node
	// Now lastNode has the fewest total demerits.
	var vert Node
	bps = append(bps, lastNode)
	for e := lastNode; e != nil; e = e.from {
		startPos := e.Position
		// startPos.Prev() is nil at paragraph start
		if startPos.Prev() != nil {
			startPos = startPos.Next()
		}
		if curPre != nil {
			InsertAfter(startPos, endNode.Prev(), curPre)
		}
		curPre = e.Pre
		if startPos != nil {
			// if PDF/UA is written, the line end should have a space at the end.
			InsertAfter(startPos, endNode.Prev(), settings.LineEndGlue.Copy())
			startPos = InsertBefore(startPos, startPos, settings.LineStartGlue.Copy())

			hl := HpackToWithEnd(startPos, endNode.Prev(), lb.settings.HSize)
			vert = InsertBefore(vert, vert, hl)
			// insert vertical glue if necessary
			if e.next != nil {
				lineskip := NewGlue()
				lineskip.Attributes = H{"origin": "lineskip"}
				if totalHeightHL := hl.Height + hl.Depth; totalHeightHL < settings.LineHeight {
					lineskip.Width = settings.LineHeight - totalHeightHL
				}
				vert = InsertBefore(vert, vert, lineskip)
				endNode = e.Position
				bps = append(bps, e)
			}
		}
	}
	// reverse the order
	for i, j := 0, len(bps)-1; i < j; i, j = i+1, j-1 {
		bps[i], bps[j] = bps[j], bps[i]
	}

	vl := Vpack(vert)
	return vl, bps
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
