package frontend

import (
	"io"
	"testing"

	"github.com/boxesandglue/textshape/ot"
)

// seedFamily wires a FontSource into a fresh FontFamily without touching the
// PDF face loader; we drive coverage decisions through the per-Document cache
// directly so tests don't need real fonts.
func seedFamily(fe *Document, name string, src *FontSource) *FontFamily {
	ff := fe.NewFontFamily(name)
	if err := ff.AddMember(src, FontWeight400, FontStyleNormal); err != nil {
		panic(err)
	}
	return ff
}

// seedCoverage primes the per-Document cache so coverageSegments doesn't fall
// through to LoadFace. The probe sees the cached value and skips the loader
// entirely — that's the entire purpose of the cache.
func seedCoverage(fe *Document, fs *FontSource, runes string, has bool) {
	for _, r := range runes {
		fe.coverageCache.store(fs, ot.Codepoint(r), has)
	}
}

// TestCoverageSegmentsWhitespacePinsToPrimary guards the spec'd whitespace
// rule: tab/space metrics come from the primary font even if a fallback also
// covers the space. Otherwise inter-word advance would jitter at every
// face boundary.
func TestCoverageSegmentsWhitespacePinsToPrimary(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatalf("NewForWriter: %v", err)
	}
	primary := &FontSource{Name: "primary"}
	secondary := &FontSource{Name: "secondary"}
	// Primary lacks the emoji, secondary has it. Both cover the space.
	seedCoverage(fe, primary, "hello world", true)
	seedCoverage(fe, secondary, "hello world", true)
	const emoji = "🟢"
	for _, r := range emoji {
		fe.coverageCache.store(primary, ot.Codepoint(r), false)
		fe.coverageCache.store(secondary, ot.Codepoint(r), true)
	}
	pFam := seedFamily(fe, "primary", primary)
	sFam := seedFamily(fe, "secondary", secondary)

	runs := fe.coverageSegments("hi "+emoji+" yo", []*FontFamily{pFam, sFam}, FontWeight400, FontStyleNormal)
	wantTexts := []string{"hi ", emoji, " yo"}
	wantIdx := []int{0, 1, 0}
	if len(runs) != len(wantTexts) {
		t.Fatalf("got %d runs, want %d: %#v", len(runs), len(wantTexts), runs)
	}
	for i, r := range runs {
		if r.Text != wantTexts[i] {
			t.Errorf("run %d text = %q, want %q", i, r.Text, wantTexts[i])
		}
		if r.StackIndex != wantIdx[i] {
			t.Errorf("run %d StackIndex = %d, want %d", i, r.StackIndex, wantIdx[i])
		}
	}
}

// TestCoverageSegmentsMergesAdjacent verifies that consecutive clusters
// resolving to the same source produce ONE run, not one run per cluster.
// shapeForBuild shapes per-run, so unmerged runs would multiply HarfBuzz
// calls and disable cross-cluster shaping (kerning, ligatures) within the
// same font.
func TestCoverageSegmentsMergesAdjacent(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatalf("NewForWriter: %v", err)
	}
	primary := &FontSource{Name: "primary"}
	seedCoverage(fe, primary, "abcdef", true)
	pFam := seedFamily(fe, "primary", primary)

	runs := fe.coverageSegments("abcdef", []*FontFamily{pFam, pFam}, FontWeight400, FontStyleNormal)
	if len(runs) != 1 {
		t.Fatalf("got %d runs, want 1: %#v", len(runs), runs)
	}
	if runs[0].Text != "abcdef" {
		t.Errorf("merged run text = %q, want %q", runs[0].Text, "abcdef")
	}
}

// TestCoverageSegmentsKeepsZWJCluster guards UAX#29 grapheme-cluster
// integrity: a ZWJ-glued emoji sequence (e.g. family 👨‍👩‍👧) is one cluster
// and must stay on one font. Splitting it would either render as separate
// emoji or trigger broken cluster shaping.
func TestCoverageSegmentsKeepsZWJCluster(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatalf("NewForWriter: %v", err)
	}
	primary := &FontSource{Name: "primary"}
	secondary := &FontSource{Name: "secondary"}
	// Whole family sequence is in secondary. Primary lacks the man.
	for _, r := range "👨👩👧" {
		fe.coverageCache.store(primary, ot.Codepoint(r), false)
		fe.coverageCache.store(secondary, ot.Codepoint(r), true)
	}
	// ZWJ + VS are coverage-ignorable; no cache entry needed.
	pFam := seedFamily(fe, "primary", primary)
	sFam := seedFamily(fe, "secondary", secondary)

	const family = "\U0001F468‍\U0001F469‍\U0001F467" // 👨‍👩‍👧
	runs := fe.coverageSegments(family, []*FontFamily{pFam, sFam}, FontWeight400, FontStyleNormal)
	if len(runs) != 1 {
		t.Fatalf("ZWJ family split across %d runs: %#v", len(runs), runs)
	}
	if runs[0].Text != family {
		t.Errorf("run text = %q, want full ZWJ sequence", runs[0].Text)
	}
	if runs[0].StackIndex != 1 {
		t.Errorf("ZWJ cluster resolved to stack[%d], want secondary (1)", runs[0].StackIndex)
	}
}

// TestCoverageSegmentsVariationSelectorIgnored verifies that VS16 (U+FE0F)
// alone does NOT veto a font's coverage decision. VS glyphs are typically
// unencoded in the cmap; we still need the base char's font to win so the
// color/text presentation bias from the source is preserved at shape time.
func TestCoverageSegmentsVariationSelectorIgnored(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatalf("NewForWriter: %v", err)
	}
	primary := &FontSource{Name: "primary"}
	const heart = "❤️" // ❤️ — heart + VS16
	// Primary covers the base char but not VS16. The base coverage is
	// what matters; VS16 must be filtered out of the probe.
	fe.coverageCache.store(primary, 0x2764, true)
	fe.coverageCache.store(primary, 0xFE0F, false)
	pFam := seedFamily(fe, "primary", primary)

	runs := fe.coverageSegments(heart, []*FontFamily{pFam}, FontWeight400, FontStyleNormal)
	if len(runs) != 1 {
		t.Fatalf("VS-only veto: split into %d runs: %#v", len(runs), runs)
	}
	if runs[0].StackIndex != 0 {
		t.Errorf("heart+VS16 resolved to stack[%d], want primary (0)", runs[0].StackIndex)
	}
}

// TestCoverageSegmentsEmptyInputs documents the contract for degenerate
// inputs — coverageSegments must not panic when the caller forgets the
// stack-length gate or passes an empty string.
func TestCoverageSegmentsEmptyInputs(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatalf("NewForWriter: %v", err)
	}
	if got := fe.coverageSegments("", nil, FontWeight400, FontStyleNormal); got != nil {
		t.Errorf("empty stack + empty string: got %v, want nil", got)
	}
	if got := fe.coverageSegments("x", nil, FontWeight400, FontStyleNormal); got != nil {
		t.Errorf("nil stack: got %v, want nil", got)
	}
	pFam := seedFamily(fe, "p", &FontSource{Name: "p"})
	if got := fe.coverageSegments("", []*FontFamily{pFam}, FontWeight400, FontStyleNormal); got != nil {
		t.Errorf("empty string: got %v, want nil", got)
	}
}

// TestCoverageSegmentsPinsToPrimaryOnTotalMiss verifies the .notdef fallback
// — when no family in the stack covers the cluster, we resolve to the
// primary. This keeps the .notdef glyph attached to the originally-intended
// face (parity with single-family behaviour) so author-visible Tofu still
// renders in the body font, not a fallback.
func TestCoverageSegmentsPinsToPrimaryOnTotalMiss(t *testing.T) {
	fe, err := NewForWriter(io.Discard)
	if err != nil {
		t.Fatalf("NewForWriter: %v", err)
	}
	primary := &FontSource{Name: "primary"}
	secondary := &FontSource{Name: "secondary"}
	// Neither face covers the codepoint.
	fe.coverageCache.store(primary, 0x1F600, false)
	fe.coverageCache.store(secondary, 0x1F600, false)
	pFam := seedFamily(fe, "p", primary)
	sFam := seedFamily(fe, "s", secondary)

	runs := fe.coverageSegments("\U0001F600", []*FontFamily{pFam, sFam}, FontWeight400, FontStyleNormal)
	if len(runs) != 1 {
		t.Fatalf("total miss: got %d runs, want 1: %#v", len(runs), runs)
	}
	if runs[0].StackIndex != 0 || runs[0].Source != primary {
		t.Errorf("total miss did not pin to primary: %#v", runs[0])
	}
}

// TestIsCoverageIgnorable spot-checks the codepoint classifier — the listed
// codepoints would otherwise force every emoji cluster to find a font that
// covers invisible format characters, which no font does outside a tiny
// privileged set.
func TestIsCoverageIgnorable(t *testing.T) {
	cases := []struct {
		r    rune
		want bool
	}{
		{0x200C, true},  // ZWNJ
		{0x200D, true},  // ZWJ
		{0xFE0E, true},  // VS15
		{0xFE0F, true},  // VS16
		{0xE0100, true}, // IVS-1
		{0x00AD, true},  // soft hyphen
		{'a', false},
		{'🟢', false},
		{0x0301, false}, // combining acute — not Cf, must stay relevant for coverage
	}
	for _, c := range cases {
		if got := isCoverageIgnorable(c.r); got != c.want {
			t.Errorf("isCoverageIgnorable(U+%04X) = %v, want %v", c.r, got, c.want)
		}
	}
}
