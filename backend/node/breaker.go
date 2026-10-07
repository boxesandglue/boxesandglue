package node

import "github.com/boxesandglue/boxesandglue/backend/bag"

// A Breaker chooses where a paragraph breaks. Linebreak finds the legal
// breakpoints, measures, packs and sets the lines, and asks the LineModel,
// as without one; only the choice among the breakpoints moves to the
// Breaker. A nil Breaker (LinebreakSettings.Breaker) keeps Knuth-Plass.
type Breaker interface {
	// Breaks returns the indexes into p.Candidates at which the paragraph
	// breaks, in order, the last one being the paragraph's end. Linebreak
	// breaks at every forced candidate and at the last one whether or not
	// they are returned, and ignores an index out of range or out of order.
	Breaks(p *BreakProblem) []int
}

// BreakProblem is one paragraph's choice of breaks.
type BreakProblem struct {
	// Candidates are the legal breakpoints, in order.
	Candidates []Candidate
	// Settings are the paragraph's own settings, which Fit reads; a
	// Breaker must not change them during Breaks.
	Settings *LinebreakSettings
	// Fit measures a line from candidate from (-1 for the paragraph's
	// start) to candidate to, set as row row (0 for the first line), the
	// way Knuth-Plass measures it: with the width of a penalty or a
	// discretionary's pre-break text at its end, without the glue discarded
	// at its start, from a tab stop the line reaches, and with the row's
	// Indent and IndentRight. Fit is valid only during Breaks. It assumes
	// from < to; otherwise it returns a line of negative width, not an
	// error.
	Fit func(from, to, row int) LineFit
}

// Candidate is a legal breakpoint.
type Candidate struct {
	// Node is the node the paragraph breaks at: a Glue, Penalty, Disc or
	// HardBreak.
	Node Node
	// Penalty is the cost of breaking here as Knuth-Plass counts it: 0 at a
	// glue, the penalty's, a discretionary's plus the Hyphenpenalty, and
	// -10000 at a HardBreak. -10000 or less is a forced break.
	Penalty int
	// Flagged is set at a discretionary, a hyphenated break.
	Flagged bool
}

// Forced reports whether the paragraph must break at c.
func (c Candidate) Forced() bool {
	return c.Penalty <= -10000
}

// LineFit is how a line fits its measure.
type LineFit struct {
	// Natural is the line's width with its glue unset, Measure the width it
	// is set to: HSize less the row's insets. Stretch and Shrink are its
	// finite stretch and shrink, with the font expansion and, for Stretch,
	// the EmergencyStretch; fil glue is not in them, see Ratio.
	Natural, Stretch, Shrink, Measure bag.ScaledPoint
	// Ratio is the adjustment ratio as Knuth-Plass computes it: 0 for a
	// line short of its measure that has fil glue (a ragged line), +Inf for
	// one that cannot reach its measure, negative for one that shrinks, less
	// than -1 for one too wide even shrunk.
	Ratio float64
	// Feasible is whether Knuth-Plass would take the line: Ratio from -1 to
	// below the Tolerance.
	Feasible bool
}

// candidate is a Candidate with the running sums Fit measures from: before,
// at the break, and after, past the glue the break discards. tabs is the
// number of tabs before it.
type candidate struct {
	Candidate
	before, after lineSums
	tabs          int
}

// addCandidate records n, a legal breakpoint, for the Breaker.
func (lb *linebreaker) addCandidate(n Node) {
	c := candidate{Candidate: Candidate{Node: n}, before: lb.lineSums, tabs: len(lb.tabs)}
	switch t := n.(type) {
	case *Penalty:
		c.Penalty = t.Penalty
	case *HardBreak:
		c.Penalty = -10000
	case *Disc:
		c.Penalty = lb.settings.Hyphenpenalty + t.Penalty
		c.Flagged = true
	}
	W, E, Y, Z := lb.computeSum(n)
	lb.markFirstTab(n)
	c.after = lineSums{
		sumW: W, sumExpand: E, sumY: Y, sumZ: Z,
		stretchFil: lb.stretchFil, stretchFill: lb.stretchFill, stretchFilll: lb.stretchFilll,
	}
	lb.cands = append(lb.cands, c)
}

// fit is BreakProblem.Fit. It measures as computeAdjustmentRatio does at
// candidate to, with the running sums and tabs set back to what they were
// there.
func (lb *linebreaker) fit(root *Breakpoint, from, to, row int) LineFit {
	c := &lb.cands[to]
	a := &Breakpoint{Position: root.Position, Line: row}
	if from >= 0 {
		a.Position, a.lineSums = lb.cands[from].Node, lb.cands[from].after
	}
	sums, tabs := lb.lineSums, lb.tabs
	lb.lineSums, lb.tabs = c.before, lb.tabs[:c.tabs]
	r, sumExpand, _ := lb.computeAdjustmentRatio(c.Node, a)
	m := lb.measured
	f := LineFit{
		Natural: m.width, Measure: m.maxwd,
		Stretch: lb.sumY - m.from.sumY + sumExpand + lb.settings.EmergencyStretch,
		Shrink:  lb.sumZ - m.from.sumZ + sumExpand,
		Ratio:   r, Feasible: -1 <= r && r < lb.settings.Tolerance,
	}
	lb.lineSums, lb.tabs = sums, tabs
	return f
}

// lineMeasure is a line computeAdjustmentRatio measured: its natural width,
// its measure, and the running sums it is measured from, which are only
// valid until the next call.
type lineMeasure struct {
	width, maxwd bag.ScaledPoint
	from         *lineSums
}

// chooseBreaks asks the Breaker for the breaks among the candidates and
// returns the breakpoint ending the paragraph, chained back to root as
// Knuth-Plass chains its choice.
func (lb *linebreaker) chooseBreaks(root *Breakpoint) *Breakpoint {
	n := len(lb.cands)
	if n == 0 {
		return root
	}
	pub := make([]Candidate, n)
	for i, c := range lb.cands {
		pub[i] = c.Candidate
	}
	chosen := make([]bool, n)
	last := -1
	for _, i := range lb.settings.Breaker.Breaks(&BreakProblem{
		Candidates: pub,
		Settings:   lb.settings,
		Fit:        func(from, to, row int) LineFit { return lb.fit(root, from, to, row) },
	}) {
		if i > last && i < n {
			chosen[i], last = true, i
		}
	}
	chosen[n-1] = true
	prev, prevIdx := root, -1
	for i, c := range lb.cands {
		if !chosen[i] && !c.Forced() {
			continue
		}
		f := lb.fit(root, prevIdx, i, prev.Line)
		width := c.before.sumW
		var pre Node
		switch v := c.Node.(type) {
		case *Penalty:
			width += v.Width
		case *Disc:
			width += 5 * bag.Factor
			pre = v.Pre
		}
		from := prev.lineSums
		bp := &Breakpoint{
			id:               int(breakpointNextID.Add(1)),
			Position:         c.Node,
			Pre:              pre,
			Line:             prev.Line + 1,
			from:             prev,
			Fitness:          calculateFitnessClass(f.Ratio),
			Width:            width - from.sumW,
			lineSums:         c.after,
			calculatedExpand: c.before.sumExpand - from.sumExpand,
			R:                f.Ratio,
		}
		prev, prevIdx = bp, i
	}
	return prev
}
