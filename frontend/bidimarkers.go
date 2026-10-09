package frontend

import (
	"sort"

	"github.com/boxesandglue/boxesandglue/backend/node"
)

// isBidiMarker reports whether n is a marker that the bidi reorder moves with
// what it marks instead of reordering it as content: a StartStop (link,
// destination, colour, decoration, inline background) or a node.Lang.
func isBidiMarker(n node.Node) bool {
	switch n.(type) {
	case *node.StartStop, *node.Lang:
		return true
	}
	return false
}

// bidiMarkers puts the markers of a paragraph back after the visual reorder.
//
// A pair of StartStop markers encloses a range of the logical order: the stop
// points to its start with StartNode. Reordered as content, the markers would
// cut the runs of the line, and an RTL run between them would come out with
// its stop in front of its start. So the markers are taken out, the content is
// reordered, and each pair is put back around every visually contiguous part
// of what it encloses, as CSS splits an inline box at a change of direction:
// the original start opens the part that holds its logical start, the
// original stop closes the part that holds its logical end, and the other
// parts get copies. A pair that runs over the end of a line is closed there
// and opened again on the next one, because its consumers (links,
// backgrounds, decorations, colours) read the start and the stop of a line
// from left to right. A marker without a partner, a destination say, stays at
// the logical start of the node that follows it.
type bidiMarkers struct {
	// stopOf is the stop of each start in the paragraph.
	stopOf map[*node.StartStop]*node.StartStop
	// open are the starts of the pairs that continue on the next line, outer
	// first.
	open []*node.StartStop
}

func newBidiMarkers(lines []*node.HList) *bidiMarkers {
	bm := &bidiMarkers{stopOf: map[*node.StartStop]*node.StartStop{}}
	for _, line := range lines {
		if line == nil {
			continue
		}
		for n := line.List; n != nil; n = n.Next() {
			if s, ok := n.(*node.StartStop); ok && s.StartNode != nil {
				bm.stopOf[s.StartNode] = s
			}
		}
	}
	return bm
}

// bidiPair is a pair of markers on one line. from and to are the range of the
// line's content it encloses, in logical indexes.
type bidiPair struct {
	start, stop         *node.StartStop
	from, to            int
	startHere, stopHere bool
}

// reorderLine reorders the content of line and puts its markers back.
func (bm *bidiMarkers) reorderLine(line *node.HList) {
	type marker struct {
		n node.Node
		// gap is the number of content nodes before the marker.
		gap int
	}
	var content []node.Node
	var marks []marker
	for n := line.List; n != nil; n = n.Next() {
		if isBidiMarker(n) {
			marks = append(marks, marker{n, len(content)})
		} else {
			content = append(content, n)
		}
	}
	lo, hi := lineFurniture(content)
	clamp := func(g int) int { return min(max(g, lo), hi) }

	// The pairs on the line, outer first: the ones open from the line before,
	// then the ones that start here, in their logical order.
	var pairs []*bidiPair
	byStart := map[*node.StartStop]*bidiPair{}
	for _, s := range bm.open {
		p := &bidiPair{start: s, stop: bm.stopOf[s], from: lo, to: hi}
		pairs = append(pairs, p)
		byStart[s] = p
	}
	var points []marker
	for _, m := range marks {
		s, _ := m.n.(*node.StartStop)
		switch {
		case s != nil && s.StartNode != nil && byStart[s.StartNode] != nil:
			p := byStart[s.StartNode]
			p.to, p.stopHere = clamp(m.gap), true
		case s != nil && bm.stopOf[s] != nil:
			p := &bidiPair{start: s, stop: bm.stopOf[s], from: clamp(m.gap), to: hi, startHere: true}
			pairs = append(pairs, p)
			byStart[s] = p
		default:
			points = append(points, m)
		}
	}
	// A pair that comes from the line before or goes on to the next one
	// begins and ends with ink, not with the spaces at the line edge.
	notInk := func(n node.Node) bool {
		switch n.(type) {
		case *node.Glue, *node.Penalty:
			return true
		}
		return false
	}
	bm.open = nil
	for _, p := range pairs {
		if !p.startHere {
			for p.from < p.to && notInk(content[p.from]) {
				p.from++
			}
		}
		if !p.stopHere {
			for p.to > p.from && notInk(content[p.to-1]) {
				p.to--
			}
			bm.open = append(bm.open, p.start)
		}
	}

	order := visualOrder(content, lo, hi)
	vpos := make([]int, len(content))
	for k, i := range order {
		vpos[i] = k
	}
	// visualGap is the visual position in front of which a marker at the
	// logical gap g goes: at the logical start of the node that follows it,
	// which is the right edge of a right to left node, or at the logical end
	// of the last one.
	rtl := func(i int) bool { return content[i].BidiLevel()%2 == 1 }
	visualGap := func(g int) int {
		switch {
		case g < lo || g > hi:
			return g
		case g < hi && rtl(g):
			return vpos[g] + 1
		case g < hi:
			return vpos[g]
		case hi > lo && rtl(hi-1):
			return vpos[hi-1]
		case hi > lo:
			return vpos[hi-1] + 1
		}
		return g
	}

	// The markers in front of each visual position: stops close inner parts
	// first, then the unpaired markers, then starts open outer parts first.
	stops := make([][]node.Node, len(content)+1)
	middle := make([][]node.Node, len(content)+1)
	starts := make([][]node.Node, len(content)+1)
	for _, m := range points {
		g := visualGap(m.gap)
		middle[g] = append(middle[g], m.n)
	}
	for _, p := range pairs {
		runs := visualRuns(vpos, p.from, p.to)
		if len(runs) == 0 {
			// An empty pair stays together where it was, one that
			// encloses nothing of this line is left out of it.
			if p.startHere && p.stopHere {
				g := visualGap(p.from)
				middle[g] = append(middle[g], p.start, p.stop)
			}
			continue
		}
		for _, r := range runs {
			s := p.start
			if !p.startHere || vpos[p.from] < r[0] || vpos[p.from] > r[1] {
				s = p.start.Copy().(*node.StartStop)
			}
			e := p.stop
			if !p.stopHere || vpos[p.to-1] < r[0] || vpos[p.to-1] > r[1] {
				e = p.stop.Copy().(*node.StartStop)
			}
			e.StartNode = s
			starts[r[0]] = append(starts[r[0]], s)
			stops[r[1]+1] = append([]node.Node{e}, stops[r[1]+1]...)
		}
	}

	var head, tail node.Node
	add := func(n node.Node) {
		n.SetPrev(nil)
		n.SetNext(nil)
		if head == nil {
			head = n
		} else {
			tail.SetNext(n)
			n.SetPrev(tail)
		}
		tail = n
	}
	for k := 0; k <= len(content); k++ {
		for _, bucket := range [][]node.Node{stops[k], middle[k], starts[k]} {
			for _, n := range bucket {
				add(n)
			}
		}
		if k < len(content) {
			add(content[order[k]])
		}
	}
	line.List = head
}

// visualRuns returns the visually contiguous parts of the logical range from
// to to, as first and last visual position, from left to right.
func visualRuns(vpos []int, from, to int) [][2]int {
	if from >= to {
		return nil
	}
	ps := make([]int, 0, to-from)
	for i := from; i < to; i++ {
		ps = append(ps, vpos[i])
	}
	sort.Ints(ps)
	var runs [][2]int
	for _, v := range ps {
		if len(runs) > 0 && runs[len(runs)-1][1] == v-1 {
			runs[len(runs)-1][1] = v
		} else {
			runs = append(runs, [2]int{v, v})
		}
	}
	return runs
}
