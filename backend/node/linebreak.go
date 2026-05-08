package node

import (
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

var (
	positiveInf      = math.Inf(1.0)
	breakpointNextID atomic.Int64
)

// isForcedBreak reports whether n is a node that must end the current line.
// Penalty(-10000) is the classic forced-break signal; HardBreak is a typed
// forced break emitted by string atom processing for "\n".
func isForcedBreak(n Node) bool {
	switch t := n.(type) {
	case *Penalty:
		return t.Penalty <= -10000
	case *HardBreak:
		return true
	}
	return false
}

// The data structure here used to store the breakpoints is a two way linked
// list where the "next" pointer builds the chain of active nodes (all nodes to
// be considered when looking if the active can reach the current position) and
// the "from" pointer points to the line break of the previous line. The "from"
// pointer is set when creating a new breakpoint node and adding it to the list
// of active nodes.

// Breakpoint is a feasible break point.
type Breakpoint struct {
	Position                              Node
	Pre                                   Node
	from                                  *Breakpoint
	next                                  *Breakpoint
	id                                    int
	Line                                  int
	Fitness                               int
	Width                                 bag.ScaledPoint
	sumW, sumY, sumZ                      bag.ScaledPoint
	sumExpand                             bag.ScaledPoint
	calculatedExpand                      bag.ScaledPoint
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
	settings         *LinebreakSettings
	sumW, sumY, sumZ bag.ScaledPoint
	sumExpand        bag.ScaledPoint
	stretchFil       bag.ScaledPoint
	stretchFill      bag.ScaledPoint
	stretchFilll     bag.ScaledPoint
}

func newLinebreaker(settings *LinebreakSettings) *linebreaker {
	lb := &linebreaker{
		settings: settings,
	}
	return lb
}

// computeAdjustmentRatio computes the Knuth-Plass adjustment ratio r for a
// candidate line from active node a to break trigger n. It additionally
// returns overfullNoShrink, which signals the "L > l_j AND Z = 0" case from
// Knuth-Plass 1981 §4 — a definitively overfull line with no shrink budget
// available. The paper writes r := ∞ here, but the active-deactivation
// criterion for this case is *not* simply r < -1; it has to be tested as a
// separate condition on the (L > l_j, Z = 0) state, which this flag carries.
func (lb *linebreaker) computeAdjustmentRatio(n Node, a *Breakpoint) (r float64, sumExpand bag.ScaledPoint, overfullNoShrink bool) {
	// compute the adjustment ratio r from a to n
	thisLineWidth := lb.sumW - a.sumW
	switch t := n.(type) {
	case *Penalty:
		thisLineWidth += t.Width
	case *Disc:
		if !lb.settings.HangingPunctuationEnd {
			wd, _, _ := Dimensions(t.Pre, nil, Horizontal)
			thisLineWidth += wd
		}
	case *Glue:
		if lb.settings.HangingPunctuationEnd {
			if p := t.Prev(); p.Type() == TypeGlyph {
				if g := p.(*Glyph); len(g.Components) == 1 && unicode.IsPunct(rune(g.Components[0])) {
					thisLineWidth -= g.Width
				}
			}
		}
	}
	// subtract left glue setting
	maxwd := lb.settings.HSize - lb.getIndent(a.Line)
	sumExpand = lb.sumExpand - a.sumExpand
	if thisLineWidth < maxwd {
		// needs to stretch. EmergencyStretch (TeX \emergencystretch) is added
		// to the per-line stretch capacity unconditionally — it acts as a
		// last-resort budget so a feasible underfull break can be found when
		// the only glue on the line is the candidate breakpoint itself (whose
		// own stretch is discarded at the break).
		y := lb.sumY - a.sumY + sumExpand + lb.settings.EmergencyStretch
		if y > 0 {
			if (lb.stretchFil-a.stretchFil) > 0 || (lb.stretchFill-a.stretchFill) > 0 || (lb.stretchFilll-a.stretchFilll) > 0 {
				// stretchable glue available
				r = 0
			} else {
				r = float64(maxwd-thisLineWidth) / float64(y)
			}
		} else {
			r = positiveInf
		}
	} else if maxwd < thisLineWidth {
		// needs to shrink
		z := lb.sumZ - a.sumZ + sumExpand
		if z > 0 {
			r = float64(maxwd-thisLineWidth) / float64(z)
		} else {
			// Definitively overfull, no shrink available. Knuth-Plass 1981 §4
			// writes r := ∞ here as a generic infeasibility sentinel; the
			// active-deactivation condition for this case is handled
			// separately at the call site (see mainLoop). r is positiveInf
			// here for symmetry with the underfull-no-stretch case.
			r = positiveInf
			overfullNoShrink = true
		}
	}
	return r, sumExpand, overfullNoShrink
}

// computeSum computes the sum of all glues from n
func (lb *linebreaker) computeSum(n Node) (bag.ScaledPoint, bag.ScaledPoint, bag.ScaledPoint, bag.ScaledPoint) {
	// compute tw=(sum w)after(b), ty=(sum y)after(b), and tz=(sum z)after(b)
	w, y, z := lb.sumW, lb.sumY, lb.sumZ
	e := lb.sumExpand
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
		case *HardBreak:
			if e != n {
				break compute
			}
		default:
			break compute
		}
	}
	return w, e, y, z
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
	case *HardBreak:
		curpenalty = -10000
	case *Disc:
		curpenalty = lb.settings.Hyphenpenalty + t.Penalty
		curflagged = true
	}
	switch {
	case curpenalty >= 0:
		demerits = onePlusBadnessSquared + curpenalty*curpenalty
	case curpenalty > -10000 && curpenalty < 0:
		demerits = onePlusBadnessSquared - curpenalty*curpenalty
	default:
		demerits = onePlusBadnessSquared
	}

	if _, ok := active.Position.(*Disc); ok {
		if curflagged {
			demerits += lb.settings.DoublehyphenDemerits
		}
	}

	// calculate fitness class
	fitnessClass = calculateFitnessClass(r)
	// if fitnessClass and active.Fitness differs by more then 1, add DemeritsFitness
	if fitnessClass > active.Fitness {
		if fitnessClass-active.Fitness > 1 {
			demerits += lb.settings.DemeritsFitness
		}
	} else {
		if active.Fitness-fitnessClass > 1 {
			demerits += lb.settings.DemeritsFitness
		}
	}

	// Detect *signed* integer overflow when accumulating with the active
	// path's demerits. A naively-negative result is not necessarily an
	// overflow: Knuth-Plass §4 lets demerits go negative legitimately when
	// a flagged break carries a negative penalty (a bonus, e.g. an
	// explicit hyphenpenalty < 0 to favour hyphenation). The classic
	// signed-overflow rule for a + b: overflow happened iff both addends
	// have the same sign and the result has the opposite sign. Any other
	// sign combination is a real arithmetic result and must pass through
	// unchanged.
	prev := demerits
	demerits += active.Demerits
	if prev > 0 && active.Demerits > 0 && demerits < 0 {
		demerits = math.MaxInt
	} else if prev < 0 && active.Demerits < 0 && demerits > 0 {
		demerits = math.MinInt
	}
	return
}

func (lb *linebreaker) getIndent(row int) bag.ScaledPoint {
	rows := lb.settings.IndentRows
	switch {
	case rows == 0:
		return lb.settings.Indent
	case rows < 0:
		if row >= -1*rows {
			return lb.settings.Indent
		}
		return bag.ScaledPoint(0)

	case rows > 0:
		if rows > row {
			return lb.settings.Indent
		}
		return bag.ScaledPoint(0)
	}
	return bag.ScaledPoint(0)
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
		ec := [4]bag.ScaledPoint{}

		// The inner loop deactivates all unreachable breakpoints and calculates
		// demerits/dmin.
		for {
			nexta := active.next

			// For each active breakpoint check if the breakpoint is still
			// active (= reachable from the current position backward). If not,
			// remove them from the current list of active nodes.
			r, sumExpand, overfullNoShrink := lb.computeAdjustmentRatio(n, active)

			// Knuth-Plass 1981 §4: deactivate active a if the line a→b is
			// definitively overfull, i.e. either r < -1 (Z > 0 but shrink
			// would have to exceed 100%) or "L > l_j AND Z = 0" (no shrink
			// reservoir at all). The two cases are equivalent in semantics
			// — once a→b is too wide, no later b' > b can become feasible
			// from a, since the line only grows.
			if r < -1 || overfullNoShrink || isForcedBreak(n) {
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
					ec[c] = sumExpand
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
			lb.appendBreakpointHere(n, dmin, dc, ac, rc, ec, active)
		}
		if dmin == math.MaxInt && lb.activeNodesA == nil {
			W, E, Y, Z := lb.computeSum(n)
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
				id:               int(breakpointNextID.Add(1)),
				Position:         n,
				Pre:              pre,
				Line:             lastInactive.Line + 1,
				from:             lastInactive,
				next:             active,
				Fitness:          3,
				Width:            lb.sumW - lastInactive.sumW,
				sumW:             W,
				sumExpand:        E,
				sumY:             Y,
				sumZ:             Z,
				calculatedExpand: lb.sumExpand - lastInactive.sumExpand,
				R:                0,
				Demerits:         lastInactive.Demerits + 1000,
				stretchFil:       lb.stretchFil,
				stretchFill:      lb.stretchFill,
				stretchFilll:     lb.stretchFilll,
			}
			lb.appendNewBreakpoint(bp)
		}
	}
}

func (lb *linebreaker) appendBreakpointHere(n Node, dmin int, dc [4]int, ac [4]*Breakpoint, rc [4]float64, ec [4]bag.ScaledPoint, active *Breakpoint) {
	W, E, Y, Z := lb.computeSum(n)

	width := lb.sumW
	var pre Node
	switch v := n.(type) {
	case *Penalty:
		width += v.Width
	case *Disc:
		width += 5 * bag.Factor
		pre = v.Pre
	}

	for c := range 4 {
		if dc[c] <= dmin+lb.settings.DemeritsFitness {
			bp := &Breakpoint{
				id:               int(breakpointNextID.Add(1)),
				Position:         n,
				Pre:              pre,
				Line:             ac[c].Line + 1,
				from:             ac[c],
				next:             active,
				Fitness:          c,
				Width:            width - ac[c].Width,
				sumW:             W,
				sumExpand:        E,
				sumY:             Y,
				sumZ:             Z,
				calculatedExpand: ec[c],
				R:                rc[c],
				Demerits:         dc[c],
				stretchFil:       lb.stretchFil,
				stretchFill:      lb.stretchFill,
				stretchFilll:     lb.stretchFilll,
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
	lb := newLinebreaker(settings)
	lb.activeNodesA = &Breakpoint{id: int(breakpointNextID.Add(1)), Fitness: 1, Position: n}
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
		case *HardBreak:
			prevItemBox = false
			lb.mainLoop(t)
		case *Disc:
			// NOTE: Do NOT reset prevItemBox here. A Disc is not a "box" in TeX terms.
			// If we reset it, a Glue following a Disc won't be considered as a breakpoint,
			// causing breaks at Disc (with hyphen) instead of at Glue (space).
			lb.mainLoop(t)
		case *Glyph:
			prevItemBox = true
			lb.sumW += t.Width
			if lb.settings.FontExpansion != 0 {
				extend := bag.MultiplyFloat(t.Width, settings.FontExpansion)
				lb.sumExpand += extend
			}
		default:
			prevItemBox = true
			wd, _, _ := e.Sizes(Horizontal)
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
		if settings.HangingPunctuationEnd {
			if e.Position.Type() == TypeDisc {
				e.Position.(*Disc).Pre.(*Glyph).Width = 0
			}
			if glyf, ok := e.Position.Prev().(*Glyph); ok {
				if len(glyf.Components) == 1 && unicode.IsPunct(rune(glyf.Components[0])) {
					glyf.Width = 0
				}
			}
		}
		startPos := e.Position
		// startPos.Prev() is nil at paragraph start
		if startPos.Prev() != nil {
			startPos = startPos.Next()
			// If we broke at a Disc followed by a Glue (space), skip the Glue.
			// Otherwise the space appears at the start of the next line.
			if e.Position.Type() == TypeDisc {
				if _, isGlue := startPos.(*Glue); isGlue {
					startPos = startPos.Next()
				}
			}
		}
		if curPre != nil {
			InsertAfter(startPos, endNode.Prev(), curPre)
		}
		// Set curPre for the next line, but NOT if we broke at a Disc
		// that's followed by a Glue (space) - in that case we're breaking
		// between words, not within a word, so no hyphen should appear.
		curPre = e.Pre
		if e.Position.Type() == TypeDisc {
			if _, isGlue := e.Position.Next().(*Glue); isGlue {
				curPre = nil // Don't insert hyphen when breaking at word boundary
			}
		}
		if startPos != nil {
			// if PDF/UA is written, the line end should have a space at the end.
			lineEnd := settings.LineEndGlue.Copy().(*Glue)
			// Forced-break suppression of justification: a line that
			// ends in a HardBreak should not be justified, even when
			// the surrounding paragraph is. In Justify mode
			// LineEndGlue / LineStartGlue are zero-stretch placeholders
			// (StretchOrder = StretchNormal), so the only stretch
			// source for the line is the inline Word-Glues — those
			// would get spread out to span HSize. Substitute a fill-
			// stretch LineEndGlue for this one line; its higher
			// StretchOrder dominates the per-line glue accounting and
			// the Word-Glues stay at their natural width.
			//
			// Left/Right/Center already configure a fill-stretch
			// LineEndGlue or LineStartGlue at the paragraph level, so
			// no override is needed there — the existing per-line glue
			// carries the effect for free.
			//
			// Note: the line being built here ends at endNode (the
			// next break trigger that follows e in source order). When
			// endNode is the HardBreak, this is the line we want to
			// fix. e.Position itself marks the START of this line and
			// is therefore the wrong node to test.
			if _, endsAtHB := endNode.(*HardBreak); endsAtHB {
				if settings.LineEndGlue.StretchOrder < StretchFil &&
					settings.LineStartGlue.StretchOrder < StretchFil {
					lineEnd = NewGlue()
					lineEnd.Stretch = bag.Factor
					lineEnd.StretchOrder = StretchFill
					lineEnd.Subtype = GlueLineEnd
				}
			}
			lineEnd.Attributes = H{"origin": "lineend"}
			InsertAfter(startPos, endNode.Prev(), lineEnd)

			// indentation
			leftskip := settings.LineStartGlue.Copy().(*Glue)
			leftskip.Attributes = H{"origin": "leftskip"}
			leftskip.Width += lb.getIndent(e.Line)
			startPos = InsertBefore(startPos, startPos, leftskip)
			hl := HpackToWithEnd(startPos, endNode.Prev(), lb.settings.HSize, FontExpansion(lb.settings.FontExpansion), SqueezeOverfullBoxes(settings.SqueezeOverfullBoxes))
			if hl.Attributes == nil {
				hl.Attributes = H{"origin": "line"}
			} else {
				hl.Attributes["origin"] = "line"
			}
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
	if !settings.OmitLastLeading {
		lineskip := NewGlue()
		lineskip.Attributes = H{"origin": "last lineskip"}
		hl := Tail(vert).(*HList)

		if totalHeightHL := hl.Height + hl.Depth; totalHeightHL < settings.LineHeight {
			lineskip.Width = settings.LineHeight - totalHeightHL
		}
		vert = InsertAfter(vert, hl, lineskip)
	}
	vl := Vpack(vert)
	vl.Attributes = H{"origin": "Linebreak"}
	return vl, bps
}

// AppendLineEndAfter adds a penalty 10000, glue 0pt plus 1fil, penalty -10000
// after n (the node lists starting with head). It returns the new head (if head
// is nil) and the penalty node (the tail of the list).
func AppendLineEndAfter(head, n Node) (Node, Node) {
	if head == nil {
		head = n
	}
	p := NewPenalty()
	p.Penalty = 10000
	head = InsertAfter(head, n, p)
	g := NewGlue()
	g.Attributes = H{"origin": "lineend"}
	g.Width = 0
	g.Stretch = 1 * bag.Factor
	g.StretchOrder = 1
	head = InsertAfter(head, p, g)

	p = NewPenalty()
	p.Penalty = -10000
	head = InsertAfter(head, g, p)
	return head, p
}
