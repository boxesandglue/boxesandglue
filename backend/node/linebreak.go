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
	Position Node
	Pre      Node
	from     *Breakpoint
	next     *Breakpoint
	id       int
	Line     int
	Fitness  int
	Width    bag.ScaledPoint
	lineSums
	calculatedExpand bag.ScaledPoint
	R                float64
	Demerits         int
}

// lineSums are the running sums of the first pass: the natural width, the
// finite stretch and shrink, the font expansion and the infinite stretch
// orders.
type lineSums struct {
	sumW, sumY, sumZ                      bag.ScaledPoint
	sumExpand                             bag.ScaledPoint
	stretchFil, stretchFill, stretchFilll bag.ScaledPoint
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
	activeNodesA   *Breakpoint
	inactiveNodesP *Breakpoint
	preva          *Breakpoint
	settings       *LinebreakSettings
	lineSums
	ts   *TabStops
	tabs []tabMark
	// firstTab is the index of the first tab on a line that starts at a
	// break at the node. Kept here rather than in Breakpoint, which is
	// allocated for every feasible break of every paragraph.
	firstTab map[Node]int
	org      lineOrigin
	runs     map[*Glue]*tabRun
	// keep is set inside a run that has no legal breakpoints.
	keep bool
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
	curW := lb.sumW
	switch t := n.(type) {
	case *Penalty:
		curW += t.Width
	case *Disc:
		// A Disc followed by glue breaks at a word boundary, the second
		// pass sets no hyphen there, so the hyphen is not measured either.
		if _, isGlue := t.Next().(*Glue); !isGlue && !lb.settings.HangingPunctuationEnd {
			wd, _, _ := Dimensions(t.Pre, nil, Horizontal)
			curW += wd
		}
	case *Glue:
		if lb.settings.HangingPunctuationEnd {
			if p := t.Prev(); p.Type() == TypeGlyph {
				if g := p.(*Glyph); len(g.Components) == 1 && unicode.IsPunct(rune(g.Components[0])) {
					curW -= g.Width
				}
			}
		}
	}
	// Past a tab stop the line is measured from the tab instead of from a.
	var x bag.ScaledPoint
	from := &a.lineSums
	aligned := false
	if len(lb.tabs) > 0 {
		if o := lb.origin(a, curW); o != nil {
			x, from, aligned = o.x, &o.lineSums, o.aligned
		}
	}
	thisLineWidth := x + curW - from.sumW
	// subtract the per-row insets: the measure a line has to fit into is the
	// hsize less whatever is taken off either end for this row.
	maxwd := lb.settings.HSize - lb.getIndent(a.Line) - lb.getIndentRight(a.Line)
	sumExpand = lb.sumExpand - from.sumExpand
	if thisLineWidth < maxwd {
		// needs to stretch. EmergencyStretch (TeX \emergencystretch) is added
		// to the per-line stretch capacity unconditionally — it acts as a
		// last-resort budget so a feasible underfull break can be found when
		// the only glue on the line is the candidate breakpoint itself (whose
		// own stretch is discarded at the break).
		//
		// LineStartGlue / LineEndGlue (TeX \leftskip / \rightskip) are
		// inserted per line in the post-linebreak pass (see Linebreak),
		// so their stretch is not part of lb.sumY / lb.stretchFil etc.
		// If they carry fil-order stretch (the typical setup for
		// HAlignLeft / Right / Center), every underfull line is
		// effectively perfect: the fil-stretch absorbs whatever slack
		// remains. Treat that case as r=0 here so feasible breaks at
		// inter-word glue aren't rejected with r=+inf just because the
		// line itself has no normal stretch reservoir.
		hasFilStretch := aligned || (lb.stretchFil-from.stretchFil) > 0 || (lb.stretchFill-from.stretchFill) > 0 || (lb.stretchFilll-from.stretchFilll) > 0
		if !hasFilStretch {
			if g := lb.settings.LineEndGlue; g != nil && g.StretchOrder >= StretchFil && g.Stretch > 0 {
				hasFilStretch = true
			}
			if g := lb.settings.LineStartGlue; g != nil && g.StretchOrder >= StretchFil && g.Stretch > 0 {
				hasFilStretch = true
			}
		}
		if hasFilStretch {
			r = 0
		} else {
			y := lb.sumY - from.sumY + sumExpand + lb.settings.EmergencyStretch
			if y > 0 {
				r = float64(maxwd-thisLineWidth) / float64(y)
			} else {
				r = positiveInf
			}
		}
	} else if maxwd < thisLineWidth {
		// needs to shrink
		z := lb.sumZ - from.sumZ + sumExpand
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
	// The glue after a Disc is discarded with the break like the glue after
	// any other break, the second pass drops it from the next line. n stays
	// the break itself, so a tab right after the Disc is kept below.
	start := n
	if _, isDisc := n.(*Disc); isDisc {
		start = n.Next()
	}
compute:
	for e := start; e != nil; e = e.Next() {
		switch t := e.(type) {
		case *Glue:
			// A tab after a break stays on the line and positions what
			// follows, so it is not discarded with the break.
			if e != n && lb.isTab(t) {
				break compute
			}
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
	return indentForRow(lb.settings.Indent, lb.settings.IndentRows, row)
}

// getIndentRight is getIndent for the right-hand inset.
func (lb *linebreaker) getIndentRight(row int) bag.ScaledPoint {
	return indentForRow(lb.settings.IndentRight, lb.settings.IndentRightRows, row)
}

// indentForRow selects the inset for a row: 0 rows means every row, a positive
// count means the first n, a negative count means every row except the first n.
// IndentForRow returns the left inset of the given row (counted from 0)
// under these settings, see Indent and IndentRows.
func (ls *LinebreakSettings) IndentForRow(row int) bag.ScaledPoint {
	return indentForRow(ls.Indent, ls.IndentRows, row)
}

// IndentRightForRow returns the right inset of the given row (counted from
// 0) under these settings, see IndentRight and IndentRightRows.
func (ls *LinebreakSettings) IndentRightForRow(row int) bag.ScaledPoint {
	return indentForRow(ls.IndentRight, ls.IndentRightRows, row)
}

func indentForRow(indent bag.ScaledPoint, rows, row int) bag.ScaledPoint {
	switch {
	case rows == 0:
		return indent
	case rows < 0:
		if row >= -1*rows {
			return indent
		}
		return bag.ScaledPoint(0)

	case rows > 0:
		if rows > row {
			return indent
		}
		return bag.ScaledPoint(0)
	}
	return bag.ScaledPoint(0)
}

func (lb *linebreaker) mainLoop(n Node) {
	active := lb.activeNodesA
	lb.preva = nil

	// Best anchor for the emergency overfull-line fallback below: the latest
	// (max sumW = closest to n, so the forced overfull line is as short as
	// possible) breakpoint that gets deactivated *because its line is
	// overfull*, ties broken toward the fewest demerits. Accumulated across
	// all line-group iterations of this mainLoop call, because the emergency
	// only fires once the active list is fully drained. Only overfull
	// deactivations qualify — a forced break (HardBreak) that deactivates an
	// underfull line must keep the LIFO anchor so stacked blank lines from
	// consecutive forced breaks are preserved.
	var bestOverfull *Breakpoint

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
				if r < -1 || overfullNoShrink {
					if bestOverfull == nil || active.sumW > bestOverfull.sumW ||
						(active.sumW == bestOverfull.sumW && active.Demerits < bestOverfull.Demerits) {
						bestOverfull = active
					}
				}
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
			lb.markFirstTab(n)
			// Anchor the forced overfull line. Prefer the best overfull
			// breakpoint found this round (latest position, fewest demerits)
			// so an unbreakable run wider than HSize — e.g. a long URL, or
			// the mailmerge company block where dropped <br/> glued several
			// fields into one ~220pt token — does not drag every preceding
			// word onto its own line. With ragged alignment every word break
			// is feasible (r=0), so the plain LIFO inactiveNodesP would be the
			// deepest single-word chain. Fall back to LIFO when nothing was
			// deactivated for being overfull (e.g. stacked blank lines from
			// consecutive forced breaks), where the most recent node is right.
			lastInactive := lb.inactiveNodesP
			if bestOverfull != nil {
				lastInactive = bestOverfull
			}
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
				id:       int(breakpointNextID.Add(1)),
				Position: n,
				Pre:      pre,
				Line:     lastInactive.Line + 1,
				from:     lastInactive,
				next:     active,
				Fitness:  3,
				Width:    lb.sumW - lastInactive.sumW,
				lineSums: lineSums{
					sumW: W, sumExpand: E, sumY: Y, sumZ: Z,
					stretchFil: lb.stretchFil, stretchFill: lb.stretchFill, stretchFilll: lb.stretchFilll,
				},
				calculatedExpand: lb.sumExpand - lastInactive.sumExpand,
				R:                0,
				Demerits:         lastInactive.Demerits + 1000,
			}
			lb.appendNewBreakpoint(bp)
		}
	}
}

func (lb *linebreaker) appendBreakpointHere(n Node, dmin int, dc [4]int, ac [4]*Breakpoint, rc [4]float64, ec [4]bag.ScaledPoint, active *Breakpoint) {
	W, E, Y, Z := lb.computeSum(n)
	lb.markFirstTab(n)

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
				id:       int(breakpointNextID.Add(1)),
				Position: n,
				Pre:      pre,
				Line:     ac[c].Line + 1,
				from:     ac[c],
				next:     active,
				Fitness:  c,
				Width:    width - ac[c].sumW,
				lineSums: lineSums{
					sumW: W, sumExpand: E, sumY: Y, sumZ: Z,
					stretchFil: lb.stretchFil, stretchFill: lb.stretchFill, stretchFilll: lb.stretchFilll,
				},
				calculatedExpand: ec[c],
				R:                rc[c],
				Demerits:         dc[c],
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
	if lb.ts = NewTabStops(settings); lb.ts != nil {
		lb.firstTab = map[Node]int{}
		lb.runs = lb.measureTabRuns(n)
	}
	lb.activeNodesA = &Breakpoint{id: int(breakpointNextID.Add(1)), Fitness: 1, Position: n}
	var endNode Node

	for e := n; e != nil; e = e.Next() {
		// breakable after
		switch t := e.(type) {
		case *Glue:
			if lb.keep && lb.isTab(t) {
				lb.keep = false
			}
			if prevItemBox && !lb.keep {
				// b legal breakpoint
				lb.mainLoop(t)
			}
			wBefore := lb.sumW

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
			if lb.isTab(t) {
				run := lb.runs[t]
				lb.tabs = append(lb.tabs, tabMark{wBefore: wBefore, after: lb.lineSums, run: run})
				lb.keep = run.keep
			}
			prevItemBox = false
		case *Penalty:
			prevItemBox = false
			if lb.keep && isForcedBreak(t) {
				lb.keep = false
			}
			if t.Penalty < 10000 && !lb.keep {
				lb.mainLoop(t)
			}
		case *HardBreak:
			prevItemBox = false
			lb.keep = false
			lb.mainLoop(t)
		case *Disc:
			// NOTE: Do NOT reset prevItemBox here. A Disc is not a "box" in TeX terms.
			// If we reset it, a Glue following a Disc won't be considered as a breakpoint,
			// causing breaks at Disc (with hyphen) instead of at Glue (space).
			if !lb.keep {
				lb.mainLoop(t)
			}
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
			// The glyph loses its advance so the line end ignores it. In a
			// left to right line the ink runs on from the origin past the
			// line end. A right to left line is mirrored after the break,
			// the glyph ends up at the left edge, and XOffset pulls the ink
			// out to the left by the advance. The PDF backend takes the
			// difference between Glyph.Width and the font advance back in
			// the TJ array, so the zero width does not open a gap before
			// the next glyph.
			hang := func(glyf *Glyph) {
				if settings.TextDirection == TextDirRTL {
					glyf.XOffset -= glyf.Width
				}
				glyf.Width = 0
			}
			if e.Position.Type() == TypeDisc {
				hang(e.Position.(*Disc).Pre.(*Glyph))
			}
			if glyf, ok := e.Position.Prev().(*Glyph); ok {
				if len(glyf.Components) == 1 && unicode.IsPunct(rune(glyf.Components[0])) {
					hang(glyf)
				}
			}
		}
		startPos := e.Position
		// startPos.Prev() is nil at paragraph start
		if startPos.Prev() != nil {
			startPos = startPos.Next()
			// If we broke at a Disc followed by a Glue (space), skip the Glue.
			// Otherwise the space appears at the start of the next line. A
			// tab stays, as after any break.
			if e.Position.Type() == TypeDisc {
				if _, isGlue := startPos.(*Glue); isGlue && !lb.isTab(startPos) {
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
			var tabbed, aligned bool
			if lb.ts != nil {
				tabbed, aligned = lb.setTabs(startPos, endNode, e.Line)
			}
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
			//
			// The slack goes to the line end, which in a right to left line
			// is the left edge: the leftskip takes the fill there.
			leftskip := settings.LineStartGlue.Copy().(*Glue)
			if _, endsAtHB := endNode.(*HardBreak); endsAtHB {
				if settings.LineEndGlue.StretchOrder < StretchFil &&
					settings.LineStartGlue.StretchOrder < StretchFil {
					fill := NewGlue()
					fill.Stretch = bag.Factor
					fill.StretchOrder = StretchFill
					if settings.TextDirection == TextDirRTL {
						fill.Subtype = GlueLineStart
						leftskip = fill
					} else {
						fill.Subtype = GlueLineEnd
						lineEnd = fill
					}
				}
			}
			if tabbed {
				lb.setFromStart(leftskip, lineEnd, aligned)
			}
			lineEnd.Attributes = H{"origin": "lineend"}
			// The right-hand inset is width added to the line-end glue rather
			// than a narrower hbox: the line still spans the full HSize, so
			// alignment resolves inside the measure that is left, exactly as it
			// does for the left inset.
			lineEnd.Width += lb.getIndentRight(e.Line)
			InsertAfter(startPos, endNode.Prev(), lineEnd)

			// indentation
			leftskip.Attributes = H{"origin": "leftskip"}
			leftskip.Width += lb.getIndent(e.Line)
			startPos = InsertBefore(startPos, startPos, leftskip)
			hl := HpackToWithEnd(startPos, endNode.Prev(), lb.settings.HSize, FontExpansion(lb.settings.FontExpansion), SqueezeOverfullBoxes(settings.SqueezeOverfullBoxes))
			if hl.Attributes == nil {
				hl.Attributes = H{"origin": "line"}
			} else {
				hl.Attributes["origin"] = "line"
			}
			hl.TextDir = settings.TextDirection
			if settings.HalfLeading {
				// CSS half-leading: grow the line box symmetrically to
				// LineHeight instead of emitting lineskip glue below. The
				// output positions each line by its Height, so the baseline
				// moves down automatically; baseline-to-baseline distances
				// and the total paragraph height are unchanged. Lines taller
				// than LineHeight keep their natural size (no negative
				// leading).
				if extra := settings.LineHeight - hl.Height - hl.Depth; extra > 0 {
					half := extra / 2
					hl.Height += half
					hl.Depth += extra - half
				}
			}
			vert = InsertBefore(vert, vert, hl)
			// insert vertical glue if necessary
			if e.next != nil {
				if !settings.HalfLeading {
					lineskip := NewGlue()
					lineskip.Attributes = H{"origin": "lineskip"}
					if totalHeightHL := hl.Height + hl.Depth; totalHeightHL < settings.LineHeight {
						lineskip.Width = settings.LineHeight - totalHeightHL
					}
					vert = InsertBefore(vert, vert, lineskip)
				}
				endNode = e.Position
				bps = append(bps, e)
			}
		}
	}
	// reverse the order
	for i, j := 0, len(bps)-1; i < j; i, j = i+1, j-1 {
		bps[i], bps[j] = bps[j], bps[i]
	}
	// In half-leading mode there is no trailing glue to omit: half the
	// leading sits in the last line's depth by construction, so
	// OmitLastLeading has no meaning there.
	if !settings.OmitLastLeading && !settings.HalfLeading {
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
	vl.TextDir = settings.TextDirection
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
