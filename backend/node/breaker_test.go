package node

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

const breakerText = "a bb ccc dddd eeeee ffffff ggggggg hh iii jjjj kkkkk l mm nnn oooo ppppp qqqqqq rr sss tttt uuuuu vvvvvv w xx yyy zzzz"

var (
	breakerChar  = bag.MustSP("5pt")
	breakerSpace = bag.MustSP("3pt")
)

func breakerList() Node {
	return buildWords(strings.Fields(breakerText), breakerChar, breakerSpace)
}

// shortLastList is breakerText with a short word that greedy sets on a last
// line of its own.
func shortLastList() Node {
	return buildWords(strings.Fields(breakerText+" ab"), breakerChar, breakerSpace)
}

// wordsPerLine counts the words on each line by its interword glue.
func wordsPerLine(vl *VList) []int {
	var out []int
	for n := vl.List; n != nil; n = n.Next() {
		hl, ok := n.(*HList)
		if !ok {
			continue
		}
		words := 1
		for m := hl.List; m != nil; m = m.Next() {
			if g, ok := m.(*Glue); ok && g.Attributes == nil {
				words++
			}
		}
		out = append(out, words)
	}
	return out
}

// greedyWords is what a greedy breaker puts on each line: as many words as
// fit the row's measure at their natural width.
func greedyWords(measure func(row int) bag.ScaledPoint) []int {
	var out []int
	var w bag.ScaledPoint
	n := 0
	for _, word := range strings.Fields(breakerText) {
		ww := bag.ScaledPoint(len(word)) * breakerChar
		if n > 0 && w+breakerSpace+ww > measure(len(out)) {
			out = append(out, n)
			n, w = 0, 0
		}
		if n > 0 {
			w += breakerSpace
		}
		w += ww
		n++
	}
	return append(out, n)
}

func ragged(ls *LinebreakSettings) {
	ls.LineEndGlue = NewGlue()
	ls.LineEndGlue.Stretch = bag.Factor
	ls.LineEndGlue.StretchOrder = StretchFil
}

// greedy is a first fit breaker: each line takes what fits at its natural
// width.
type greedy struct{}

func (greedy) Breaks(p *BreakProblem) []int {
	var breaks []int
	from, fits := -1, -1
	for i := 0; i < len(p.Candidates); i++ {
		f := p.Fit(from, i, len(breaks))
		switch {
		case f.Natural <= f.Measure && p.Candidates[i].Forced():
			breaks, from, fits = append(breaks, i), i, -1
		case f.Natural <= f.Measure:
			fits = i
		case fits >= 0:
			// The line ends at the last break that fit; measure i again
			// from there.
			breaks, from, fits = append(breaks, fits), fits, -1
			i--
		default:
			// Nothing fits: the line is overfull to here.
			breaks, from = append(breaks, i), i
		}
	}
	return breaks
}

// balance is a ragged breaker that keeps greedy's number of lines and evens
// out their natural widths, last line included.
type balance struct{}

func (balance) Breaks(p *BreakProblem) []int {
	lines := len(greedy{}.Breaks(p))
	n := len(p.Candidates)
	// cost[k][i] is the least cost of k+1 lines ending at candidate i.
	cost := make([][]float64, lines)
	prev := make([][]int, lines)
	for k := range cost {
		cost[k] = make([]float64, n)
		prev[k] = make([]int, n)
		for i := range n {
			cost[k][i] = math.Inf(1)
			froms := []int{-1}
			if k > 0 {
				froms = froms[:0]
				for j := range i {
					froms = append(froms, j)
				}
			}
			for _, j := range froms {
				base := 0.0
				if j >= 0 {
					base = cost[k-1][j]
				}
				f := p.Fit(j, i, k)
				if f.Natural > f.Measure || math.IsInf(base, 1) {
					continue
				}
				short := (f.Measure - f.Natural).ToPT()
				if c := base + short*short; c < cost[k][i] {
					cost[k][i], prev[k][i] = c, j
				}
			}
		}
	}
	breaks := make([]int, lines)
	for k, i := lines-1, n-1; k >= 0; k-- {
		breaks[k] = i
		i = prev[k][i]
	}
	return breaks
}

// replay breaks where a nil Breaker broke the same paragraph, and checks
// that Fit measures each of those lines as Knuth-Plass did.
type replay struct {
	t      *testing.T
	breaks []int // node indexes in the list
	bps    []*Breakpoint
	head   Node
}

func (r *replay) Breaks(p *BreakProblem) []int {
	index := map[Node]int{}
	i := 0
	for n := r.head; n != nil; n = n.Next() {
		index[n] = i
		i++
	}
	var out []int
	for ci, c := range p.Candidates {
		if slices.Contains(r.breaks, index[c.Node]) {
			out = append(out, ci)
		}
	}
	from := -1
	for row, ci := range out {
		if f := p.Fit(from, ci, row); f.Ratio != r.bps[row].R || !f.Feasible {
			r.t.Errorf("line %d: Fit's ratio %v (feasible %v), Knuth-Plass's %v", row, f.Ratio, f.Feasible, r.bps[row].R)
		}
		from = ci
	}
	return out
}

func nodeIndexes(index map[Node]int, bps []*Breakpoint) []int {
	var out []int
	for _, bp := range bps {
		out = append(out, index[bp.Position])
	}
	return out
}

// A Breaker that chooses Knuth-Plass's breaks sets the paragraph as Knuth-
// Plass does: Fit measures each line as the built-in breaker does, and
// Linebreak packs and sets the lines the same.
func TestBreakerReplaysKnuthPlass(t *testing.T) {
	plain := func(setup func(*LinebreakSettings)) func() *LinebreakSettings {
		return func() *LinebreakSettings {
			ls := NewLinebreakSettings()
			ls.HSize = bag.MustSP("150pt")
			setup(ls)
			return ls
		}
	}
	const tabbed = "a b\tccc ddd eee fff ggg hhh iii jjj kkk lll mmm nnn ooo ppp\tqqq rrr sss ttt uuu vvv www"
	tabs := func(align TabAlign) func() *LinebreakSettings {
		return func() *LinebreakSettings {
			ls := tabSettings(TabStop{Position: 150 * bag.Factor, Align: align})
			ls.LineEndGlue = NewGlue()
			return ls
		}
	}
	for _, c := range []struct {
		name     string
		list     func() Node
		settings func() *LinebreakSettings
	}{
		{"justified", breakerList, plain(func(*LinebreakSettings) {})},
		{"ragged", breakerList, plain(ragged)},
		{"indented", breakerList, plain(func(ls *LinebreakSettings) { ls.Indent, ls.IndentRows = bag.MustSP("20pt"), 2 })},
		{"tab", func() Node { return buildTabbed(tabbed) }, tabs(TabAlignLeft)},
		{"right tab", func() Node { return buildTabbed(tabbed) }, tabs(TabAlignRight)},
	} {
		t.Run(c.name, func(t *testing.T) {
			ls := c.settings()
			head := c.list()
			index := map[Node]int{}
			i := 0
			for n := head; n != nil; n = n.Next() {
				index[n] = i
				i++
			}
			want, wantBps := Linebreak(head, ls)

			ls = c.settings()
			r := &replay{t: t, breaks: nodeIndexes(index, wantBps), bps: wantBps, head: c.list()}
			ls.Breaker = r
			got, gotBps := Linebreak(r.head, ls)
			if len(gotBps) != len(wantBps) {
				t.Fatalf("%d lines, Knuth-Plass %d", len(gotBps), len(wantBps))
			}
			for i := range wantBps {
				if gotBps[i].Width != wantBps[i].Width || gotBps[i].R != wantBps[i].R || gotBps[i].Line != wantBps[i].Line {
					t.Errorf("line %d: width %v ratio %v row %d, Knuth-Plass %v %v %d", i, gotBps[i].Width, gotBps[i].R, gotBps[i].Line, wantBps[i].Width, wantBps[i].R, wantBps[i].Line)
				}
			}
			if got.Height != want.Height || got.Depth != want.Depth {
				t.Errorf("paragraph %v+%v, Knuth-Plass %v+%v", got.Height, got.Depth, want.Height, want.Depth)
			}
			gl, wl := lineContentWidths(got), lineContentWidths(want)
			if !slices.Equal(gl, wl) {
				t.Errorf("line contents %v, Knuth-Plass %v", gl, wl)
			}
		})
	}
}

// A greedy Breaker fills each line with what fits at its natural width,
// where Knuth-Plass chooses other breaks for the paragraph as a whole.
func TestGreedyBreaker(t *testing.T) {
	ls := NewLinebreakSettings()
	ls.HSize = bag.MustSP("150pt")
	kp, _ := Linebreak(breakerList(), ls)
	ls.Breaker = greedy{}
	vl, _ := Linebreak(breakerList(), ls)
	want := greedyWords(func(int) bag.ScaledPoint { return ls.HSize })
	if got := wordsPerLine(vl); !slices.Equal(got, want) {
		t.Errorf("words per line %v, want %v", got, want)
	}
	if slices.Equal(wordsPerLine(kp), want) {
		t.Errorf("Knuth-Plass breaks as greedy does (%v): the fixture shows nothing", want)
	}
}

// A balancing Breaker keeps greedy's number of lines and evens their widths.
func TestBalancingBreaker(t *testing.T) {
	spread := func(vl *VList) bag.ScaledPoint {
		ws := lineContentWidths(vl)
		return slices.Max(ws) - slices.Min(ws)
	}
	ls := NewLinebreakSettings()
	ls.HSize = bag.MustSP("150pt")
	ragged(ls)
	ls.Breaker = greedy{}
	g, _ := Linebreak(shortLastList(), ls)
	ls.Breaker = balance{}
	b, _ := Linebreak(shortLastList(), ls)
	if len(wordsPerLine(b)) != len(wordsPerLine(g)) {
		t.Fatalf("balanced %v, greedy %v: not the same number of lines", wordsPerLine(b), wordsPerLine(g))
	}
	if spread(b) >= spread(g) {
		t.Errorf("balanced lines spread %v, greedy %v", spread(b), spread(g))
	}
}

// Fit measures a line by its row, so a Breaker fills the indented rows to
// their narrower measure (a parshape, or a float beside the first rows).
func TestBreakerRows(t *testing.T) {
	ls := NewLinebreakSettings()
	ls.HSize = bag.MustSP("150pt")
	ls.Indent, ls.IndentRows = bag.MustSP("30pt"), 2
	ls.IndentRight, ls.IndentRightRows = bag.MustSP("10pt"), -3
	ls.Breaker = greedy{}
	measure := func(row int) bag.ScaledPoint {
		return ls.HSize - ls.IndentForRow(row) - ls.IndentRightForRow(row)
	}
	vl, _ := Linebreak(breakerList(), ls)
	want := greedyWords(measure)
	if got := wordsPerLine(vl); !slices.Equal(got, want) {
		t.Errorf("words per line %v, want %v", got, want)
	}
	flat := greedyWords(func(int) bag.ScaledPoint { return ls.HSize })
	if slices.Equal(flat, want) {
		t.Errorf("the rows break as without insets (%v): the fixture shows nothing", want)
	}
}

// A Breaker that leaves out the forced breaks and the paragraph's end still
// breaks there.
func TestBreakerKeepsForcedBreaks(t *testing.T) {
	var head, cur Node
	head, cur = glyphRun(head, cur, "ab", breakerChar)
	hb := NewHardBreak()
	head = InsertAfter(head, cur, hb)
	cur = hb
	head, cur = glyphRun(head, cur, "cd", breakerChar)
	head, _ = AppendLineEndAfter(head, cur)
	ls := NewLinebreakSettings()
	ls.HSize = bag.MustSP("150pt")
	ls.Breaker = breakerFunc(func(*BreakProblem) []int { return nil })
	if _, bps := Linebreak(head, ls); len(bps) != 2 || bps[0].Position != hb {
		t.Errorf("%d lines, the first ending at %v", len(bps), bps[0].Position)
	}
}

type breakerFunc func(*BreakProblem) []int

func (f breakerFunc) Breaks(p *BreakProblem) []int { return f(p) }
