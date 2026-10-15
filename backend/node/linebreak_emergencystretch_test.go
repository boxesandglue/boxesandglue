package node

import (
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/boxesandglue/boxesandglue/backend/bag"
)

// breakIndexes returns the position of each break in the list at head.
func breakIndexes(head Node, bps []*Breakpoint) []int {
	idx := map[Node]int{}
	i := 0
	for e := head; e != nil; e = e.Next() {
		idx[e] = i
		i++
	}
	var out []int
	for _, bp := range bps {
		out = append(out, idx[bp.Position])
	}
	return out
}

// EmergencyStretch is used in a last pass only, as in TeX: a paragraph that
// breaks within Tolerance without an emergency line breaks the same with it,
// and one that does not is broken again with it.
func TestEmergencyStretchOnlyInLastPass(t *testing.T) {
	rng := rand.New(rand.NewSource(92))
	same, rescued := 0, 0
	for c := range 300 {
		words := make([]string, 5+rng.Intn(30))
		for i := range words {
			words[i] = strings.Repeat("a", 1+rng.Intn(10))
		}
		build := func() Node { return buildWords(words, 5*bag.Factor, 4*bag.Factor) }
		s := NewLinebreakSettings()
		s.HSize = bag.ScaledPoint(60+rng.Intn(140)) * bag.Factor
		s.Tolerance = 1.26
		first, _, _ := breakPass(build(), s, false)
		head := build()
		_, without := Linebreak(head, s)
		want := breakIndexes(head, without)

		s.EmergencyStretch = 20 * bag.Factor
		last, _, _ := breakPass(build(), s, true)
		head = build()
		_, with := Linebreak(head, s)
		got := breakIndexes(head, with)
		switch {
		case !first.emergency:
			same++
			if !slices.Equal(got, want) {
				t.Errorf("case %d (%s): breaks within Tolerance %v, with EmergencyStretch %v", c, s.HSize, want, got)
			}
		case !last.emergency:
			rescued++
		}
	}
	if same == 0 || rescued == 0 {
		t.Fatalf("%d paragraphs break within Tolerance, %d only with EmergencyStretch; the test needs both", same, rescued)
	}
}
